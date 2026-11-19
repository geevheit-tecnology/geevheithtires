CREATE TABLE billing_plans (
 id text PRIMARY KEY, name text NOT NULL, price_cents integer NOT NULL CHECK(price_cents>0),
 vehicle_limit integer NOT NULL CHECK(vehicle_limit>0), user_limit integer NOT NULL CHECK(user_limit>0),
 published boolean NOT NULL DEFAULT false
);
INSERT INTO billing_plans VALUES ('starter','Starter',14900,15,3,false),('pro','Pro',44900,60,10,false);
CREATE TABLE platform_admins (user_id uuid PRIMARY KEY REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE platform_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, actor_id uuid NOT NULL REFERENCES users(id),
 action text NOT NULL, target text NOT NULL, reason text NOT NULL, data jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE subscriptions (
 company_id uuid PRIMARY KEY REFERENCES companies(id), plan_id text REFERENCES billing_plans(id),
 paid_until timestamptz, vehicle_limit integer NOT NULL DEFAULT 15, user_limit integer NOT NULL DEFAULT 3,
 suspended boolean NOT NULL DEFAULT false, updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO subscriptions(company_id) SELECT id FROM companies;
CREATE FUNCTION initialize_subscription() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
 BEGIN INSERT INTO subscriptions(company_id) VALUES(NEW.id); RETURN NEW; END $$;
CREATE TRIGGER company_subscription AFTER INSERT ON companies FOR EACH ROW EXECUTE FUNCTION initialize_subscription();
CREATE TABLE billing_invoices (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), plan_id text NOT NULL REFERENCES billing_plans(id),
 price_cents integer NOT NULL CHECK(price_cents>0), vehicle_limit integer NOT NULL, user_limit integer NOT NULL,
 provider_id text UNIQUE, provider_attempted_at timestamptz, customer_id text, checkout_url text, due_date date NOT NULL,
 status text NOT NULL DEFAULT 'creating' CHECK(status IN ('creating','pending','paid','overdue','refunded','cancelled')),
 created_at timestamptz NOT NULL DEFAULT now(), paid_at timestamptz, applied_at timestamptz,
 period_end timestamptz, provider_status text, environment text NOT NULL CHECK(environment IN ('sandbox','production'))
);
CREATE UNIQUE INDEX one_open_invoice ON billing_invoices(company_id) WHERE status IN ('creating','pending','overdue');
CREATE TABLE billing_events (id text PRIMARY KEY, payment_id text NOT NULL, received_at timestamptz NOT NULL DEFAULT now());
GRANT SELECT ON billing_plans TO geevheit_runtime;
GRANT SELECT ON subscriptions TO geevheit_runtime;
GRANT SELECT,INSERT,UPDATE ON billing_invoices TO geevheit_runtime;
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant ON subscriptions USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime');
ALTER TABLE billing_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_invoices FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant ON billing_invoices USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime');
CREATE FUNCTION platform_allowed(actor uuid) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$
 SELECT EXISTS(SELECT 1 FROM platform_admins a JOIN users u ON u.id=a.user_id WHERE a.user_id=actor AND u.deleted_at IS NULL)
$$;
CREATE FUNCTION platform_snapshot(actor uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 RETURN jsonb_build_object(
 'companies',coalesce((SELECT jsonb_agg(row_to_json(x)) FROM (SELECT c.id,c.name,c.cnpj,c.trial_ends_at,c.created_at,s.plan_id,s.paid_until,s.suspended,
 CASE WHEN s.suspended THEN 'suspended' WHEN s.paid_until>now() THEN 'active' WHEN c.trial_ends_at>now() THEN 'trial' ELSE 'expired' END status,
 (SELECT count(*) FROM vehicles v WHERE v.company_id=c.id AND deleted_at IS NULL) vehicles,
 (SELECT count(*) FROM users u WHERE u.company_id=c.id AND deleted_at IS NULL) users
 FROM companies c JOIN subscriptions s ON s.company_id=c.id WHERE c.deleted_at IS NULL ORDER BY c.created_at DESC LIMIT 200) x),'[]'),
 'plans',(SELECT jsonb_agg(to_jsonb(p) ORDER BY price_cents) FROM billing_plans p),
 'metrics',jsonb_build_object('companies',(SELECT count(*) FROM companies WHERE deleted_at IS NULL),
 'active',(SELECT count(*) FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE c.deleted_at IS NULL AND NOT s.suspended AND s.paid_until>now()),
 'trials',(SELECT count(*) FROM subscriptions s JOIN companies c ON c.id=s.company_id WHERE c.deleted_at IS NULL AND NOT s.suspended AND (s.paid_until IS NULL OR s.paid_until<=now()) AND c.trial_ends_at>now()),
 'received_cents',(SELECT coalesce(sum(price_cents),0) FROM billing_invoices WHERE status='paid' AND environment='production')),
 'audit',coalesce((SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT a.id,a.actor_id,a.action,a.target,a.reason,a.created_at,a.data FROM platform_audit a ORDER BY a.id DESC LIMIT 50) x),'[]'));
END $$;
CREATE FUNCTION platform_change(actor uuid, target text, operation text, payload jsonb, justification text) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE days integer;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF length(trim(justification))<5 OR length(justification)>500 THEN RAISE EXCEPTION 'Reason required' USING ERRCODE='22023'; END IF;
 IF operation IN ('suspend','restore','trial') THEN
 PERFORM pg_advisory_xact_lock(hashtextextended(target,7));
 IF NOT EXISTS(SELECT 1 FROM companies WHERE id=target::uuid AND deleted_at IS NULL) THEN RAISE EXCEPTION 'Unknown company' USING ERRCODE='22023'; END IF;
 IF operation='trial' THEN
 days:=(payload->>'days')::integer;
 IF days IS NULL OR days<1 OR days>90 THEN RAISE EXCEPTION 'Invalid days' USING ERRCODE='22023'; END IF;
 UPDATE companies SET trial_ends_at=greatest(now(),trial_ends_at)+make_interval(days=>days),updated_at=now() WHERE id=target::uuid;
 ELSE UPDATE subscriptions SET suspended=(operation='suspend'),updated_at=now() WHERE company_id=target::uuid; END IF;
 ELSIF operation='publish' THEN
 UPDATE billing_plans SET published=(payload->>'published')::boolean WHERE id=target;
 IF NOT FOUND THEN RAISE EXCEPTION 'Unknown plan' USING ERRCODE='22023'; END IF;
 ELSE RAISE EXCEPTION 'Invalid operation' USING ERRCODE='22023'; END IF;
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,operation,target,justification,payload);
END $$;
-- Enforcement lives in PostgreSQL so parallel requests cannot overrun a plan.
CREATE FUNCTION enforce_commercial_limit() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE s subscriptions; trial_end timestamptz; used bigint; max_allowed integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.company_id::text,7));
 SELECT * INTO s FROM subscriptions WHERE company_id=NEW.company_id;
 SELECT trial_ends_at INTO trial_end FROM companies WHERE id=NEW.company_id;
 IF s.suspended OR (coalesce(s.paid_until,'-infinity')<=now() AND trial_end<=now()) THEN
 RAISE EXCEPTION 'Assinatura indisponível. Consulte Plano e cobrança.' USING ERRCODE='P0001'; END IF;
 IF TG_TABLE_NAME='vehicles' THEN
 SELECT count(*) INTO used FROM vehicles WHERE company_id=NEW.company_id AND deleted_at IS NULL;
 max_allowed:=CASE WHEN s.paid_until>now() THEN s.vehicle_limit ELSE 15 END;
 ELSE
 SELECT count(*) INTO used FROM users WHERE company_id=NEW.company_id AND deleted_at IS NULL;
 max_allowed:=CASE WHEN s.paid_until>now() THEN s.user_limit ELSE 3 END;
 END IF;
 IF used>=max_allowed THEN RAISE EXCEPTION 'Limite do plano atingido. Consulte Plano e cobrança.' USING ERRCODE='P0001'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER vehicles_commercial BEFORE INSERT ON vehicles FOR EACH ROW EXECUTE FUNCTION enforce_commercial_limit();
CREATE TRIGGER users_commercial BEFORE INSERT ON users FOR EACH ROW EXECUTE FUNCTION enforce_commercial_limit();
CREATE FUNCTION billing_lookup(payment text) RETURNS SETOF billing_invoices LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT * FROM billing_invoices WHERE provider_id=payment $$;
CREATE FUNCTION billing_settle(event_id text, payment text, canonical_status text, cents integer, mode text) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE inv billing_invoices; ending timestamptz;
BEGIN
 SELECT * INTO inv FROM billing_invoices WHERE provider_id=payment FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'Unknown payment' USING ERRCODE='22023'; END IF;
 IF inv.price_cents<>cents OR inv.environment<>mode THEN RAISE EXCEPTION 'Payment mismatch' USING ERRCODE='22023'; END IF;
 INSERT INTO billing_events(id,payment_id) VALUES(event_id,payment) ON CONFLICT DO NOTHING;
 IF NOT FOUND THEN RETURN; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(inv.company_id::text,7));
 IF canonical_status IN ('RECEIVED','CONFIRMED','RECEIVED_IN_CASH') THEN
 IF inv.applied_at IS NULL AND inv.status NOT IN ('refunded','cancelled') THEN
 SELECT greatest(now(),coalesce(paid_until,now()))+interval '1 month' INTO ending FROM subscriptions WHERE company_id=inv.company_id;
 UPDATE billing_invoices SET status='paid',paid_at=now(),applied_at=now(),period_end=ending,provider_status=canonical_status WHERE id=inv.id;
 UPDATE subscriptions SET plan_id=inv.plan_id,paid_until=ending,vehicle_limit=inv.vehicle_limit,user_limit=inv.user_limit,updated_at=now() WHERE company_id=inv.company_id;
 END IF;
 ELSIF canonical_status IN ('REFUNDED','CHARGEBACK_REQUESTED','CHARGEBACK_DISPUTE','AWAITING_CHARGEBACK_REVERSAL') THEN
 UPDATE billing_invoices SET status='refunded',provider_status=canonical_status WHERE id=inv.id;
 -- Conservatively revoke only when this invoice still owns the latest entitlement.
 UPDATE subscriptions SET paid_until=now(),updated_at=now() WHERE company_id=inv.company_id AND paid_until=inv.period_end;
 ELSIF inv.applied_at IS NULL AND inv.status NOT IN ('refunded','cancelled') THEN
 UPDATE billing_invoices SET status=CASE WHEN canonical_status='OVERDUE' THEN 'overdue' WHEN canonical_status='DELETED' THEN 'cancelled' ELSE 'pending' END,provider_status=canonical_status WHERE id=inv.id;
 END IF;
END $$;
REVOKE ALL ON FUNCTION platform_allowed(uuid),platform_snapshot(uuid),platform_change(uuid,text,text,jsonb,text),billing_lookup(text),billing_settle(text,text,text,integer,text),initialize_subscription(),enforce_commercial_limit() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_allowed(uuid),platform_snapshot(uuid),platform_change(uuid,text,text,jsonb,text),billing_lookup(text),billing_settle(text,text,text,integer,text) TO geevheit_runtime;
