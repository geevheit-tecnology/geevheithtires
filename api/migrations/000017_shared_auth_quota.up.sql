-- Short-lived, hash-only quotas shared by API replicas; no raw email or IP.
CREATE TABLE auth_quotas(bucket text PRIMARY KEY CHECK(bucket ~ '^[a-f0-9]{64}$'),window_started timestamptz NOT NULL,attempts integer NOT NULL CHECK(attempts>0));
CREATE INDEX auth_quotas_expiry ON auth_quotas(window_started);
CREATE FUNCTION auth_consume_quota(bucket_key text,max_attempts integer) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE used integer;
BEGIN
 IF bucket_key!~'^[a-f0-9]{64}$' OR max_attempts NOT BETWEEN 1 AND 1000 THEN RAISE EXCEPTION 'Invalid quota' USING ERRCODE='22023'; END IF;
 INSERT INTO auth_quotas(bucket,window_started,attempts) VALUES(bucket_key,now(),1)
 ON CONFLICT(bucket) DO UPDATE SET attempts=CASE WHEN auth_quotas.window_started<now()-interval '1 minute' THEN 1 ELSE auth_quotas.attempts+1 END,
 window_started=CASE WHEN auth_quotas.window_started<now()-interval '1 minute' THEN now() ELSE auth_quotas.window_started END RETURNING attempts INTO used;
 IF random()<0.02 THEN DELETE FROM auth_quotas WHERE bucket IN(SELECT bucket FROM auth_quotas WHERE window_started<now()-interval '2 minutes' ORDER BY window_started LIMIT 1000); END IF;
 RETURN used<=max_attempts;
END $$;
REVOKE ALL ON auth_quotas FROM PUBLIC,geevheit_runtime;
REVOKE ALL ON FUNCTION auth_consume_quota(text,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_consume_quota(text,integer) TO geevheit_runtime;
