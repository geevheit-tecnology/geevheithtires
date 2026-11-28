CREATE FUNCTION platform_audit_profile(actor uuid,password_changed boolean) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 INSERT INTO platform_audit(actor_id,action,target,reason,data)
 VALUES(actor,'update_profile',actor::text,'Alteração pelo próprio administrador',jsonb_build_object('password_changed',password_changed));
END $$;
REVOKE ALL ON FUNCTION platform_audit_profile(uuid,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_audit_profile(uuid,boolean) TO geevheit_runtime;
CREATE INDEX IF NOT EXISTS support_tickets_company_status ON support_tickets(company_id,status);
CREATE INDEX IF NOT EXISTS subscriptions_expiry ON subscriptions(paid_until) WHERE NOT suspended;
CREATE INDEX IF NOT EXISTS refresh_tokens_active_user ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS account_tokens_active_user ON account_tokens(user_id) WHERE used_at IS NULL;
