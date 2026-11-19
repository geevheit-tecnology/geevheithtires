CREATE FUNCTION platform_save_plan(
  actor uuid,
  target_id text,
  expected_version bigint,
  cents integer,
  vehicle_count integer,
  user_count integer,
  is_published boolean,
  apply_limits boolean,
  justification text,
  plan_name text
) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE
  affected integer := 0;
  tenant uuid;
  cleaned_name text := NULLIF(trim(coalesce(plan_name, '')), '');
  final_name text;
BEGIN
  IF NOT platform_allowed(actor) THEN
    RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501';
  END IF;
  IF length(trim(justification)) NOT BETWEEN 5 AND 500
     OR cents NOT BETWEEN 0 AND 100000000
     OR vehicle_count NOT BETWEEN 1 AND 100000
     OR user_count NOT BETWEEN 1 AND 10000
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

  UPDATE billing_plans
  SET name = coalesce(cleaned_name, name),
      price_cents = cents,
      vehicle_limit = vehicle_count,
      user_limit = user_count,
      published = is_published,
      version = version + 1
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

  RETURN jsonb_build_object('ok', true, 'name', final_name, 'clients_updated', affected);
END $$;

REVOKE ALL ON FUNCTION platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text,text) TO geevheit_runtime;
