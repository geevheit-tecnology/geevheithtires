DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='geevheit_runtime') THEN CREATE ROLE geevheit_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS; END IF;
END $$;
GRANT USAGE ON SCHEMA public TO geevheit_runtime;
GRANT SELECT,INSERT,UPDATE,DELETE ON companies,users,refresh_tokens,account_tokens,vehicles,tires,inspections,alerts,audit_log TO geevheit_runtime;
GRANT SELECT ON axle_templates TO geevheit_runtime;
DO $$ BEGIN EXECUTE format('GRANT geevheit_runtime TO %I',current_user); END $$;
ALTER TABLE tires ADD COLUMN structural_damage boolean NOT NULL DEFAULT false;
