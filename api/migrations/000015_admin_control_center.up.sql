-- Administrative controls stay behind SECURITY DEFINER functions which recheck
-- platform membership; ordinary runtime connections retain tenant isolation.
INSERT INTO billing_plans(id,name,price_cents,vehicle_limit,user_limit,published)
VALUES('enterprise','Enterprise',0,100000,10000,false) ON CONFLICT DO NOTHING;
ALTER TABLE users ADD COLUMN session_version integer NOT NULL DEFAULT 0;
CREATE FUNCTION auth_session_version(actor uuid) RETURNS integer LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT session_version FROM users WHERE id=actor AND deleted_at IS NULL $$;
REVOKE ALL ON FUNCTION auth_session_version(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_session_version(uuid) TO geevheit_runtime;

ALTER TABLE billing_invoices ADD COLUMN source text NOT NULL DEFAULT 'asaas' CHECK(source IN ('asaas','manual')),
 ADD COLUMN reference text NOT NULL DEFAULT '' CHECK(length(reference)<=200),
 ADD COLUMN version bigint NOT NULL DEFAULT 1;
CREATE INDEX billing_financial_due ON billing_invoices(environment,due_date,status);
CREATE INDEX billing_financial_paid ON billing_invoices(environment,paid_at) WHERE status='paid';

CREATE FUNCTION platform_save_plan(actor uuid,target_id text,expected_version bigint,cents integer,vehicle_count integer,user_count integer,is_published boolean,apply_limits boolean,justification text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE affected integer:=0; tenant uuid;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('plan:'||target_id,19));
 IF apply_limits THEN
  FOR tenant IN SELECT s.company_id FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE s.plan_id=target_id AND c.deleted_at IS NULL ORDER BY s.company_id LOOP
   PERFORM pg_advisory_xact_lock(hashtextextended(tenant::text,7));
  END LOOP;
  IF EXISTS(SELECT 1 FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE s.plan_id=target_id AND c.deleted_at IS NULL AND
   ((SELECT count(*) FROM vehicles v WHERE v.company_id=c.id AND v.deleted_at IS NULL)>vehicle_count OR (SELECT count(*) FROM users u WHERE u.company_id=c.id AND u.deleted_at IS NULL)>user_count)) THEN
   RAISE EXCEPTION 'Usage exceeds plan limits' USING ERRCODE='22023';
  END IF;
 END IF;
 PERFORM platform_configure_plan(actor,target_id,expected_version,cents,vehicle_count,user_count,is_published,justification);
 IF apply_limits THEN
  UPDATE subscriptions s SET vehicle_limit=vehicle_count,user_limit=user_count,updated_at=now() FROM companies c WHERE c.id=s.company_id AND c.deleted_at IS NULL AND s.plan_id=target_id;
  GET DIAGNOSTICS affected=ROW_COUNT;
  INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,'sync_plan_limits',target_id,justification,jsonb_build_object('clients_updated',affected,'vehicle_limit',vehicle_count,'user_limit',user_count));
 END IF;
 RETURN jsonb_build_object('ok',true,'clients_updated',affected);
END $$;

CREATE OR REPLACE FUNCTION platform_client_access(actor uuid,target_id uuid,target_plan text,days integer,expected_updated_at timestamptz,justification text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE p billing_plans; current_updated_at timestamptz;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF length(trim(justification)) NOT BETWEEN 5 AND 500 OR days NOT BETWEEN 1 AND 366 THEN RAISE EXCEPTION 'Invalid access period' USING ERRCODE='22023'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('plan:'||target_plan,19));
 PERFORM pg_advisory_xact_lock(hashtextextended(target_id::text,7));
 IF NOT EXISTS(SELECT 1 FROM companies WHERE id=target_id AND deleted_at IS NULL) THEN RAISE EXCEPTION 'Unknown company' USING ERRCODE='P0002'; END IF;
 SELECT * INTO p FROM billing_plans WHERE id=target_plan;
 IF NOT FOUND THEN RAISE EXCEPTION 'Unknown plan' USING ERRCODE='22023'; END IF;
 SELECT updated_at INTO current_updated_at FROM subscriptions WHERE company_id=target_id FOR UPDATE;
 IF current_updated_at IS DISTINCT FROM expected_updated_at THEN RAISE EXCEPTION 'Subscription changed; refresh' USING ERRCODE='40001'; END IF;
 IF (SELECT count(*) FROM vehicles WHERE company_id=target_id AND deleted_at IS NULL)>p.vehicle_limit OR (SELECT count(*) FROM users WHERE company_id=target_id AND deleted_at IS NULL)>p.user_limit THEN RAISE EXCEPTION 'Usage exceeds plan limits' USING ERRCODE='22023'; END IF;
 UPDATE subscriptions SET plan_id=p.id,paid_until=now()+make_interval(days=>days),vehicle_limit=p.vehicle_limit,user_limit=p.user_limit,updated_at=now() WHERE company_id=target_id;
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,'grant_access',target_id::text,justification,jsonb_build_object('plan_id',p.id,'days',days,'payment_recorded',false));
END $$;

CREATE FUNCTION platform_finance(actor uuid,start_date date,end_date date,state text,term text,cursor_id text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result jsonb;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF start_date IS NULL OR end_date IS NULL OR end_date<start_date OR end_date-start_date>366 OR state NOT IN ('','pending','paid','overdue','cancelled','refunded') OR length(term)>150 THEN RAISE EXCEPTION 'Invalid filter' USING ERRCODE='22023'; END IF;
 WITH invoices AS (
  SELECT i.id,i.company_id,c.name company_name,c.cnpj,i.plan_id,i.price_cents,i.due_date,i.paid_at,i.created_at,i.checkout_url,i.source,i.reference,i.version,i.period_end,
   CASE WHEN i.status IN ('creating','pending') AND i.due_date<(now() AT TIME ZONE 'America/Sao_Paulo')::date THEN 'overdue' ELSE i.status END status
  FROM billing_invoices i JOIN companies c ON c.id=i.company_id WHERE i.environment='production' AND i.due_date BETWEEN start_date AND end_date
   AND (term='' OR c.name ILIKE '%'||term||'%' OR c.cnpj ILIKE '%'||term||'%')
 ),page AS (SELECT * FROM invoices WHERE (state='' OR status=state) AND (cursor_id='' OR id<cursor_id::uuid) ORDER BY id DESC LIMIT 51),visible AS (SELECT * FROM page ORDER BY id DESC LIMIT 50)
 SELECT jsonb_build_object('data',coalesce((SELECT jsonb_agg(to_jsonb(v) ORDER BY id DESC) FROM visible v),'[]'),
 'next_cursor',CASE WHEN (SELECT count(*) FROM page)>50 THEN (SELECT id::text FROM visible ORDER BY id LIMIT 1) ELSE '' END,
 'summary',jsonb_build_object(
  'received_cents',coalesce((SELECT sum(i.price_cents) FROM billing_invoices i JOIN companies c ON c.id=i.company_id WHERE i.environment='production' AND i.status='paid' AND (i.paid_at AT TIME ZONE 'America/Sao_Paulo')::date BETWEEN start_date AND end_date AND (term='' OR c.name ILIKE '%'||term||'%' OR c.cnpj ILIKE '%'||term||'%')),0),
  'pending_cents',coalesce((SELECT sum(price_cents) FROM invoices WHERE status IN ('pending','creating')),0),
  'overdue_cents',coalesce((SELECT sum(price_cents) FROM invoices WHERE status='overdue'),0),
  'open_count',(SELECT count(*) FROM invoices WHERE status IN ('creating','pending','overdue')),
  'active_contracts',(SELECT count(*) FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE c.deleted_at IS NULL AND NOT s.suspended AND s.paid_until>now() AND NOT EXISTS(SELECT 1 FROM users u JOIN platform_admins a ON a.user_id=u.id WHERE u.company_id=c.id))
 ),'monthly',coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY month) FROM (
  SELECT to_char(m,'YYYY-MM') month,coalesce((SELECT sum(i.price_cents) FROM billing_invoices i WHERE i.environment='production' AND i.status='paid' AND i.paid_at>=m AND i.paid_at<m+interval '1 month'),0) received_cents
  FROM generate_series(date_trunc('month',now())-interval '5 months',date_trunc('month',now()),interval '1 month') m
 ) x),'[]')) INTO result;
 RETURN result;
END $$;

CREATE FUNCTION platform_create_receivable(actor uuid,invoice_id uuid,target_company uuid,target_plan text,cents integer,due date,payment_url text,justification text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE p billing_plans;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF cents NOT BETWEEN 1 AND 100000000 OR due IS NULL OR length(trim(justification)) NOT BETWEEN 5 AND 500 OR length(payment_url)>2000 OR (payment_url<>'' AND payment_url!~'^https://[^ /]+') THEN RAISE EXCEPTION 'Invalid receivable' USING ERRCODE='22023'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('plan:'||target_plan,19));
 PERFORM pg_advisory_xact_lock(hashtextextended(target_company::text,7));
 IF EXISTS(SELECT 1 FROM billing_invoices WHERE id=invoice_id) THEN RAISE EXCEPTION 'Invoice already exists' USING ERRCODE='40001'; END IF;
 IF NOT EXISTS(SELECT 1 FROM companies WHERE id=target_company AND deleted_at IS NULL) THEN RAISE EXCEPTION 'Unknown company' USING ERRCODE='P0002'; END IF;
 IF EXISTS(SELECT 1 FROM users u JOIN platform_admins a ON a.user_id=u.id WHERE u.company_id=target_company) THEN RAISE EXCEPTION 'Administrative company' USING ERRCODE='22023'; END IF;
 SELECT * INTO p FROM billing_plans WHERE id=target_plan;
 IF NOT FOUND THEN RAISE EXCEPTION 'Unknown plan' USING ERRCODE='22023'; END IF;
 INSERT INTO billing_invoices(id,company_id,plan_id,price_cents,vehicle_limit,user_limit,provider_id,due_date,status,environment,source,checkout_url)
 VALUES(invoice_id,target_company,target_plan,cents,p.vehicle_limit,p.user_limit,'manual:'||invoice_id::text,due,'pending','production','manual',nullif(payment_url,''));
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,'create_receivable',invoice_id::text,justification,jsonb_build_object('company_id',target_company,'plan_id',target_plan,'price_cents',cents,'due_date',due));
 RETURN invoice_id;
END $$;

CREATE FUNCTION platform_settle_receivable(actor uuid,target_invoice uuid,expected_version bigint,operation text,payment_reference text,payment_date date,justification text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE inv billing_invoices; company uuid;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF operation NOT IN ('pay','cancel','refund') OR length(trim(justification)) NOT BETWEEN 5 AND 500 OR length(payment_reference)>200 THEN RAISE EXCEPTION 'Invalid settlement' USING ERRCODE='22023'; END IF;
 SELECT company_id INTO company FROM billing_invoices WHERE id=target_invoice;
 IF NOT FOUND THEN RAISE EXCEPTION 'Unknown invoice' USING ERRCODE='P0002'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(company::text,7));
 SELECT * INTO inv FROM billing_invoices WHERE id=target_invoice FOR UPDATE;
 IF inv.source<>'manual' OR inv.environment<>'production' THEN RAISE EXCEPTION 'Provider-managed invoice' USING ERRCODE='22023'; END IF;
 IF inv.version<>expected_version THEN RAISE EXCEPTION 'Invoice changed; refresh' USING ERRCODE='40001'; END IF;
 IF operation='pay' THEN
  IF inv.status NOT IN ('pending','overdue') OR length(trim(payment_reference))<3 OR payment_date IS NULL OR payment_date>(now() AT TIME ZONE 'America/Sao_Paulo')::date THEN RAISE EXCEPTION 'Invalid payment' USING ERRCODE='22023'; END IF;
  IF NOT EXISTS(SELECT 1 FROM companies WHERE id=company AND deleted_at IS NULL) THEN RAISE EXCEPTION 'Unknown company' USING ERRCODE='P0002'; END IF;
  IF (SELECT count(*) FROM vehicles WHERE company_id=company AND deleted_at IS NULL)>inv.vehicle_limit OR (SELECT count(*) FROM users WHERE company_id=company AND deleted_at IS NULL)>inv.user_limit THEN RAISE EXCEPTION 'Usage exceeds plan limits' USING ERRCODE='22023'; END IF;
  PERFORM billing_settle('manual-pay:'||target_invoice::text,inv.provider_id,'RECEIVED',inv.price_cents,'production');
  UPDATE billing_invoices SET paid_at=(payment_date::timestamp AT TIME ZONE 'America/Sao_Paulo'),reference=trim(payment_reference),version=version+1 WHERE id=target_invoice;
 ELSIF operation='cancel' THEN
  IF inv.status NOT IN ('pending','overdue') THEN RAISE EXCEPTION 'Invalid invoice state' USING ERRCODE='22023'; END IF;
  UPDATE billing_invoices SET status='cancelled',version=version+1 WHERE id=target_invoice;
 ELSE
  IF inv.status<>'paid' THEN RAISE EXCEPTION 'Invalid invoice state' USING ERRCODE='22023'; END IF;
  PERFORM billing_settle('manual-refund:'||target_invoice::text,inv.provider_id,'REFUNDED',inv.price_cents,'production');
  UPDATE billing_invoices SET version=version+1 WHERE id=target_invoice;
 END IF;
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,'receivable_'||operation,target_invoice::text,justification,jsonb_build_object('company_id',company,'price_cents',inv.price_cents,'reference',payment_reference,'paid_on',payment_date));
 RETURN jsonb_build_object('ok',true,'company_id',company);
END $$;

CREATE FUNCTION platform_control_snapshot(actor uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result jsonb;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 result:=platform_snapshot(actor)-'companies';
 RETURN result||jsonb_build_object('profile',(SELECT jsonb_build_object('name',name,'email',email,'managed_identity',neon_subject IS NOT NULL) FROM users WHERE id=actor),
  'operations',jsonb_build_object(
   'expired',(SELECT count(*) FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE c.deleted_at IS NULL AND NOT s.suspended AND coalesce(s.paid_until,'-infinity')<=now() AND c.trial_ends_at<=now()),
   'suspended',(SELECT count(*) FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE c.deleted_at IS NULL AND s.suspended),
   'expiring',(SELECT count(*) FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE c.deleted_at IS NULL AND NOT s.suspended AND coalesce(s.paid_until,c.trial_ends_at) BETWEEN now() AND now()+interval '7 days'),
   'open_tickets',(SELECT count(*) FROM support_tickets WHERE status NOT IN ('resolved','closed')),
   'critical_tickets',(SELECT count(*) FROM support_tickets WHERE status NOT IN ('resolved','closed') AND priority='critical')
  ));
END $$;

REVOKE ALL ON FUNCTION platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text),platform_finance(uuid,date,date,text,text,text),platform_create_receivable(uuid,uuid,uuid,text,integer,date,text,text),platform_settle_receivable(uuid,uuid,bigint,text,text,date,text),platform_control_snapshot(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text),platform_finance(uuid,date,date,text,text,text),platform_create_receivable(uuid,uuid,uuid,text,integer,date,text,text),platform_settle_receivable(uuid,uuid,bigint,text,text,date,text),platform_control_snapshot(uuid) TO geevheit_runtime;
