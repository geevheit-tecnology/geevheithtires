DROP FUNCTION auth_find_user(text),auth_refresh(text),auth_logout(text),auth_account_token(text);
DROP POLICY company_scope ON companies;
ALTER TABLE companies DISABLE ROW LEVEL SECURITY;
DROP POLICY identity_scope ON users;
DROP POLICY identity_scope ON refresh_tokens;
DROP POLICY identity_scope ON account_tokens;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens DISABLE ROW LEVEL SECURITY;
ALTER TABLE account_tokens DISABLE ROW LEVEL SECURITY;
