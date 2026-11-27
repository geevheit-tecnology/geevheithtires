ALTER TABLE billing_plans ADD COLUMN IF NOT EXISTS trial_days integer NOT NULL DEFAULT 14;
ALTER TABLE billing_plans DROP CONSTRAINT IF EXISTS billing_plans_trial_days_check;
ALTER TABLE billing_plans ADD CONSTRAINT billing_plans_trial_days_check CHECK (trial_days BETWEEN 1 AND 365);

CREATE OR REPLACE FUNCTION platform_save_plan(
  actor uuid,
  target_id text,
  expected_version bigint,
  cents integer,
  vehicle_count integer,
  user_count integer,
  is_published boolean,
  apply_limits boolean,
  justification text,
  plan_name text,
  plan_trial_days integer DEFAULT 14
) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE
  affected integer := 0;
  tenant uuid;
  cleaned_name text := NULLIF(trim(coalesce(plan_name, '')), '');
  final_name text;
  effective_trial_days integer := coalesce(plan_trial_days, 14);
BEGIN
  IF NOT platform_allowed(actor) THEN
    RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501';
  END IF;
  IF length(trim(justification)) NOT BETWEEN 5 AND 500
     OR cents NOT BETWEEN 0 AND 100000000
     OR vehicle_count NOT BETWEEN 1 AND 100000
     OR user_count NOT BETWEEN 1 AND 10000
     OR effective_trial_days NOT BETWEEN 1 AND 365
     OR (is_published AND cents = 0)
     OR (cleaned_name IS NOT NULL AND length(cleaned_name) NOT BETWEEN 2 AND 60) THEN
    RAISE EXCEPTION 'Invalid configuration' USING ERRCODE='22023';
  END IF;

  PERFORM pg_advisory_xact_lock(hashtextextended('plan:'||target_id, 19));

  IF apply_limits THEN
    FOR tenant IN
      SELECT s.company_id
      FROM subscriptions s
      JOIN companies c ON c.id = s.company_id
      WHERE s.plan_id = target_id AND c.deleted_at IS NULL
      ORDER BY s.company_id
    LOOP
      PERFORM pg_advisory_xact_lock(hashtextextended(tenant::text, 7));
    END LOOP;
    IF EXISTS(
      SELECT 1
      FROM subscriptions s
      JOIN companies c ON c.id = s.company_id
      WHERE s.plan_id = target_id AND c.deleted_at IS NULL AND (
        (SELECT count(*) FROM vehicles v WHERE v.company_id = c.id AND v.deleted_at IS NULL) > vehicle_count
        OR (SELECT count(*) FROM users u WHERE u.company_id = c.id AND u.deleted_at IS NULL) > user_count
      )
    ) THEN
      RAISE EXCEPTION 'Usage exceeds plan limits' USING ERRCODE='22023';
    END IF;
  END IF;

  -- Note: billing_plan_version BEFORE UPDATE trigger automatically increments version by 1.
  UPDATE billing_plans
  SET name = coalesce(cleaned_name, name),
      price_cents = cents,
      vehicle_limit = vehicle_count,
      user_limit = user_count,
      trial_days = effective_trial_days,
      published = is_published
  WHERE id = target_id AND version = expected_version
  RETURNING name INTO final_name;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'Plan changed; refresh' USING ERRCODE='40001';
  END IF;

  INSERT INTO platform_audit(actor_id, action, target, reason, data)
  VALUES (
    actor,
    'configure_plan',
    target_id,
    justification,
    jsonb_build_object(
      'name', final_name,
      'price_cents', cents,
      'vehicle_limit', vehicle_count,
      'user_limit', user_count,
      'trial_days', effective_trial_days,
      'published', is_published
    )
  );

  IF apply_limits THEN
    UPDATE subscriptions s
    SET vehicle_limit = vehicle_count,
        user_limit = user_count,
        updated_at = now()
    FROM companies c
    WHERE c.id = s.company_id AND c.deleted_at IS NULL AND s.plan_id = target_id;
    GET DIAGNOSTICS affected = ROW_COUNT;
    INSERT INTO platform_audit(actor_id, action, target, reason, data)
    VALUES (
      actor,
      'sync_plan_limits',
      target_id,
      justification,
      jsonb_build_object(
        'clients_updated', affected,
        'vehicle_limit', vehicle_count,
        'user_limit', user_count
      )
    );
  END IF;

  RETURN jsonb_build_object('ok', true, 'name', final_name, 'trial_days', effective_trial_days, 'clients_updated', affected);
END $$;

REVOKE ALL ON FUNCTION platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text,text,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text,text,integer) TO geevheit_runtime;

CREATE OR REPLACE FUNCTION platform_change(actor uuid, target text, operation text, payload jsonb, justification text) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE days integer;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF length(trim(justification))<5 OR length(justification)>500 THEN RAISE EXCEPTION 'Reason required' USING ERRCODE='22023'; END IF;
 IF operation IN ('suspend','restore','trial') THEN
 PERFORM pg_advisory_xact_lock(hashtextextended(target,7));
 IF NOT EXISTS(SELECT 1 FROM companies WHERE id=target::uuid AND deleted_at IS NULL) THEN RAISE EXCEPTION 'Unknown company' USING ERRCODE='22023'; END IF;
 IF operation='trial' THEN
 days:=(payload->>'days')::integer;
 IF days IS NULL OR days<1 OR days>365 THEN RAISE EXCEPTION 'Invalid days' USING ERRCODE='22023'; END IF;
 UPDATE companies SET trial_ends_at=greatest(now(),trial_ends_at)+make_interval(days=>days),updated_at=now() WHERE id=target::uuid;
 ELSE UPDATE subscriptions SET suspended=(operation='suspend'),updated_at=now() WHERE company_id=target::uuid; END IF;
 ELSIF operation='publish' THEN
 UPDATE billing_plans SET published=(payload->>'published')::boolean WHERE id=target;
 IF NOT FOUND THEN RAISE EXCEPTION 'Unknown plan' USING ERRCODE='22023'; END IF;
 ELSE RAISE EXCEPTION 'Invalid operation' USING ERRCODE='22023'; END IF;
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,operation,target,justification,payload);
END $$;

CREATE OR REPLACE FUNCTION enforce_commercial_limit() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE s subscriptions; trial_end timestamptz; used bigint; max_allowed integer; starter_vehicles integer; starter_users integer;
BEGIN
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.company_id::text,7));
 SELECT * INTO s FROM subscriptions WHERE company_id=NEW.company_id;
 SELECT trial_ends_at INTO trial_end FROM companies WHERE id=NEW.company_id;
 IF s.suspended OR (coalesce(s.paid_until,'-infinity')<=now() AND trial_end<=now()) THEN
 RAISE EXCEPTION 'Assinatura indisponível. Consulte Plano e cobrança.' USING ERRCODE='P0001'; END IF;
 SELECT coalesce(vehicle_limit,15), coalesce(user_limit,3) INTO starter_vehicles, starter_users FROM billing_plans WHERE id='starter';
 IF TG_TABLE_NAME='vehicles' THEN
 SELECT count(*) INTO used FROM vehicles WHERE company_id=NEW.company_id AND deleted_at IS NULL;
 max_allowed:=CASE WHEN s.paid_until>now() THEN s.vehicle_limit ELSE coalesce(starter_vehicles,15) END;
 ELSE
 SELECT count(*) INTO used FROM users WHERE company_id=NEW.company_id AND deleted_at IS NULL;
 max_allowed:=CASE WHEN s.paid_until>now() THEN s.user_limit ELSE coalesce(starter_users,3) END;
 END IF;
 IF used>=max_allowed THEN RAISE EXCEPTION 'Limite do plano atingido. Consulte Plano e cobrança.' USING ERRCODE='P0001'; END IF;
 RETURN NEW;
END $$;

