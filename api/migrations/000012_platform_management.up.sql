ALTER TABLE billing_plans DROP CONSTRAINT billing_plans_price_cents_check;
ALTER TABLE billing_plans ADD CONSTRAINT billing_plans_price_cents_check CHECK(price_cents>=0),
 ADD CONSTRAINT billing_plan_publish_price CHECK(NOT published OR price_cents>0),
 ADD COLUMN version bigint NOT NULL DEFAULT 1;

CREATE FUNCTION platform_clients(actor uuid, term text, state text, cursor_id text) RETURNS jsonb
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

CREATE FUNCTION platform_client_detail(actor uuid, target_id uuid) RETURNS jsonb
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

CREATE FUNCTION platform_configure_plan(actor uuid, target_id text, expected_version bigint, cents integer, vehicle_count integer, user_count integer, is_published boolean, justification text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF length(trim(justification)) NOT BETWEEN 5 AND 500 OR cents NOT BETWEEN 0 AND 100000000 OR vehicle_count NOT BETWEEN 1 AND 100000 OR user_count NOT BETWEEN 1 AND 10000 OR (is_published AND cents=0) THEN RAISE EXCEPTION 'Invalid configuration' USING ERRCODE='22023'; END IF;
 UPDATE billing_plans SET price_cents=cents,vehicle_limit=vehicle_count,user_limit=user_count,published=is_published,version=version+1 WHERE id=target_id AND version=expected_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'Plan changed; refresh' USING ERRCODE='40001'; END IF;
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,'configure_plan',target_id,justification,jsonb_build_object('price_cents',cents,'vehicle_limit',vehicle_count,'user_limit',user_count,'published',is_published));
END $$;

CREATE FUNCTION platform_client_access(actor uuid, target_id uuid, target_plan text, days integer, expected_updated_at timestamptz, justification text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE p billing_plans; current_updated_at timestamptz;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF length(trim(justification)) NOT BETWEEN 5 AND 500 OR days NOT BETWEEN 1 AND 366 THEN RAISE EXCEPTION 'Invalid access period' USING ERRCODE='22023'; END IF;
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

CREATE FUNCTION close_company_internal(target_id uuid) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(target_id::text,7));
 IF EXISTS(SELECT 1 FROM platform_admins a JOIN users u ON u.id=a.user_id WHERE u.company_id=target_id AND u.deleted_at IS NULL) THEN RAISE EXCEPTION 'Administrative account cannot be closed here' USING ERRCODE='22023'; END IF;
 IF EXISTS(SELECT 1 FROM billing_invoices WHERE company_id=target_id AND status IN ('creating','pending','overdue')) THEN RAISE EXCEPTION 'Resolve open invoices before closing' USING ERRCODE='22023'; END IF;
 UPDATE companies SET deleted_at=now(),updated_at=now() WHERE id=target_id AND deleted_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'Company already closed' USING ERRCODE='P0002'; END IF;
 UPDATE users SET deleted_at=now(),updated_at=now() WHERE company_id=target_id AND deleted_at IS NULL;
 UPDATE refresh_tokens SET revoked_at=now() WHERE company_id=target_id AND revoked_at IS NULL;
 UPDATE account_tokens SET used_at=now() WHERE company_id=target_id AND used_at IS NULL;
 UPDATE subscriptions SET suspended=true,updated_at=now() WHERE company_id=target_id;
END $$;
CREATE FUNCTION platform_close_client(actor uuid,target_id uuid, confirmation text, justification text) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF length(trim(justification)) NOT BETWEEN 5 AND 500 OR NOT EXISTS(SELECT 1 FROM companies WHERE id=target_id AND cnpj=confirmation) THEN RAISE EXCEPTION 'Confirmation required' USING ERRCODE='22023'; END IF;
 PERFORM close_company_internal(target_id);
 INSERT INTO platform_audit(actor_id,action,target,reason) VALUES(actor,'close_company',target_id::text,justification);
END $$;
CREATE FUNCTION account_close(actor uuid,confirmation text) RETURNS text LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE u users;
BEGIN
 SELECT * INTO u FROM users WHERE id=actor AND company_id=nullif(current_setting('app.company_id',true),'')::uuid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND OR platform_allowed(actor) THEN RAISE EXCEPTION 'Account cannot be closed here' USING ERRCODE='42501'; END IF;
 IF u.role='owner' THEN
 IF NOT EXISTS(SELECT 1 FROM companies WHERE id=u.company_id AND cnpj=confirmation) THEN RAISE EXCEPTION 'Confirm company CNPJ' USING ERRCODE='22023'; END IF;
 PERFORM close_company_internal(u.company_id);RETURN 'company';
 END IF;
 IF confirmation<>'ENCERRAR MINHA CONTA' THEN RAISE EXCEPTION 'Confirmation required' USING ERRCODE='22023'; END IF;
 UPDATE users SET deleted_at=now(),updated_at=now() WHERE id=actor;
 UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=actor AND revoked_at IS NULL;
 UPDATE account_tokens SET used_at=now() WHERE user_id=actor AND used_at IS NULL;
 RETURN 'user';
END $$;
REVOKE ALL ON FUNCTION platform_clients(uuid,text,text,text),platform_client_detail(uuid,uuid),platform_configure_plan(uuid,text,bigint,integer,integer,integer,boolean,text),platform_client_access(uuid,uuid,text,integer,timestamptz,text),close_company_internal(uuid),platform_close_client(uuid,uuid,text,text),account_close(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_clients(uuid,text,text,text),platform_client_detail(uuid,uuid),platform_configure_plan(uuid,text,bigint,integer,integer,integer,boolean,text),platform_client_access(uuid,uuid,text,integer,timestamptz,text),platform_close_client(uuid,uuid,text,text),account_close(uuid,text) TO geevheit_runtime;
CREATE FUNCTION auth_account_active(actor uuid,tenant uuid) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$
 SELECT EXISTS(SELECT 1 FROM users u JOIN companies c ON c.id=u.company_id WHERE u.id=actor AND u.company_id=tenant AND u.deleted_at IS NULL AND c.deleted_at IS NULL)
$$;
REVOKE ALL ON FUNCTION auth_account_active(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_account_active(uuid,uuid) TO geevheit_runtime;
CREATE FUNCTION billing_plan_version() RETURNS trigger LANGUAGE plpgsql SET search_path=public,pg_temp AS $$ BEGIN NEW.version:=OLD.version+1;RETURN NEW;END $$;
CREATE TRIGGER billing_plan_version BEFORE UPDATE ON billing_plans FOR EACH ROW EXECUTE FUNCTION billing_plan_version();
