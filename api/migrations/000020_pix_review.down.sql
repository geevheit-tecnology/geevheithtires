DROP INDEX audit_pix_payment_notified;

CREATE OR REPLACE FUNCTION platform_client_detail(actor uuid, target_id uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result jsonb;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 SELECT jsonb_build_object('company',to_jsonb(c),'subscription',to_jsonb(s),
 'users',coalesce((SELECT jsonb_agg(jsonb_build_object('id',u.id,'name',u.name,'email',u.email,'role',u.role,'deleted_at',u.deleted_at) ORDER BY u.created_at) FROM users u WHERE u.company_id=c.id),'[]'),
 'invoices',coalesce((SELECT jsonb_agg(to_jsonb(i)) FROM (SELECT id,plan_id,price_cents,status,due_date,paid_at,environment FROM billing_invoices WHERE company_id=c.id ORDER BY created_at DESC LIMIT 24) i),'[]')) INTO result
 FROM companies c JOIN subscriptions s ON s.company_id=c.id WHERE c.id=target_id;
 IF result IS NULL THEN RAISE EXCEPTION 'Unknown company' USING ERRCODE='P0002'; END IF;
 RETURN result;
END $$;

CREATE OR REPLACE FUNCTION platform_control_snapshot(actor uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
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

CREATE OR REPLACE FUNCTION platform_clients(actor uuid, term text, state text, cursor_id text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result jsonb;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF state NOT IN ('','active','trial','suspended','expired','closed') OR length(term)>150 THEN RAISE EXCEPTION 'Invalid filter' USING ERRCODE='22023'; END IF;
 WITH candidates AS (
 SELECT c.id,c.name,c.cnpj,c.created_at,c.deleted_at,s.plan_id,s.paid_until,s.suspended,s.vehicle_limit,s.user_limit,s.updated_at,
 CASE WHEN c.deleted_at IS NOT NULL THEN 'closed' WHEN s.suspended THEN 'suspended' WHEN s.paid_until>now() THEN 'active' WHEN c.trial_ends_at>now() THEN 'trial' ELSE 'expired' END status,c.trial_ends_at,
 (SELECT count(*) FROM vehicles v WHERE v.company_id=c.id AND v.deleted_at IS NULL) vehicles,
 (SELECT count(*) FROM users u WHERE u.company_id=c.id AND u.deleted_at IS NULL) users,
 (SELECT email FROM users u WHERE u.company_id=c.id AND u.role='owner' ORDER BY u.created_at LIMIT 1) owner_email
 FROM companies c JOIN subscriptions s ON s.company_id=c.id
 WHERE (CASE WHEN state='closed' THEN c.deleted_at IS NOT NULL ELSE c.deleted_at IS NULL END)
 AND (term='' OR c.name ILIKE '%'||term||'%' OR c.cnpj ILIKE '%'||term||'%' OR EXISTS(SELECT 1 FROM users u WHERE u.company_id=c.id AND u.email ILIKE '%'||term||'%'))
 AND (cursor_id='' OR c.id<cursor_id::uuid)
 ), page AS (SELECT * FROM candidates WHERE state='' OR status=state ORDER BY id DESC LIMIT 51), visible AS (SELECT * FROM page ORDER BY id DESC LIMIT 50)
 SELECT jsonb_build_object('data',coalesce((SELECT jsonb_agg(to_jsonb(v) ORDER BY id DESC) FROM visible v),'[]'),
 'next_cursor',CASE WHEN (SELECT count(*) FROM page)>50 THEN (SELECT id::text FROM visible ORDER BY id LIMIT 1) ELSE '' END) INTO result;
 RETURN result;
END $$;
