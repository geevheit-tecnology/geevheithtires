DROP TRIGGER billing_plan_version ON billing_plans;
DROP FUNCTION billing_plan_version(),auth_account_active(uuid,uuid);
DROP FUNCTION account_close(uuid,text),platform_close_client(uuid,uuid,text,text),close_company_internal(uuid),platform_client_access(uuid,uuid,text,integer,timestamptz,text),platform_configure_plan(uuid,text,bigint,integer,integer,integer,boolean,text),platform_client_detail(uuid,uuid),platform_clients(uuid,text,text,text);
ALTER TABLE billing_plans DROP COLUMN version, DROP CONSTRAINT billing_plan_publish_price, DROP CONSTRAINT billing_plans_price_cents_check;
ALTER TABLE billing_plans ADD CONSTRAINT billing_plans_price_cents_check CHECK(price_cents>0) NOT VALID;
