DROP TRIGGER users_commercial ON users;
DROP TRIGGER vehicles_commercial ON vehicles;
DROP TRIGGER company_subscription ON companies;
DROP FUNCTION enforce_commercial_limit(),initialize_subscription(),platform_allowed(uuid),platform_snapshot(uuid),platform_change(uuid,text,text,jsonb,text),billing_lookup(text),billing_settle(text,text,text,integer,text);
DROP TABLE billing_events,billing_invoices,subscriptions,platform_audit,platform_admins,billing_plans;
