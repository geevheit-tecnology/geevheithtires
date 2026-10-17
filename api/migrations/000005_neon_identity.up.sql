ALTER TABLE users ADD COLUMN neon_subject text;
CREATE UNIQUE INDEX users_neon_subject ON users(neon_subject) WHERE neon_subject IS NOT NULL;
CREATE FUNCTION auth_find_neon_user(target_subject text) RETURNS SETOF users LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT * FROM users WHERE neon_subject=target_subject AND deleted_at IS NULL $$;
REVOKE ALL ON FUNCTION auth_find_neon_user(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION auth_find_neon_user(text) TO geevheit_runtime;
