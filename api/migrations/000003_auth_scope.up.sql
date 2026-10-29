-- The API's runtime role is tenant-scoped even for identity and token tables.
-- Narrow SECURITY DEFINER functions resolve identity before a tenant is known.
ALTER TABLE companies ENABLE ROW LEVEL SECURITY;
ALTER TABLE companies FORCE ROW LEVEL SECURITY;
CREATE POLICY company_scope ON companies USING(id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime') WITH CHECK(id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime');
DO $$ DECLARE tab text; BEGIN
 FOREACH tab IN ARRAY ARRAY['users','refresh_tokens','account_tokens'] LOOP
 EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tab);
 EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tab);
 EXECUTE format($p$CREATE POLICY identity_scope ON %I USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime') WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime')$p$,tab);
 END LOOP;
END $$;
CREATE FUNCTION auth_find_user(target_email text) RETURNS SETOF users LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT * FROM users WHERE email=target_email AND deleted_at IS NULL $$;
CREATE FUNCTION auth_refresh(target_hash text) RETURNS SETOF users LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
 DECLARE target_user uuid;
 BEGIN
 UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=target_hash AND revoked_at IS NULL AND expires_at>now() RETURNING user_id INTO target_user;
 RETURN QUERY SELECT * FROM users WHERE id=target_user AND deleted_at IS NULL;
 END $$;
CREATE FUNCTION auth_logout(target_hash text) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=target_hash $$;
CREATE FUNCTION auth_account_token(target_hash text) RETURNS SETOF account_tokens LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT * FROM account_tokens WHERE token_hash=target_hash AND used_at IS NULL AND expires_at>now() FOR UPDATE $$;
REVOKE ALL ON FUNCTION auth_find_user(text),auth_refresh(text),auth_logout(text),auth_account_token(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_find_user(text),auth_refresh(text),auth_logout(text),auth_account_token(text) TO geevheit_runtime;
