CREATE TABLE companies (
 id uuid PRIMARY KEY, name text NOT NULL, cnpj text NOT NULL UNIQUE,
 status text NOT NULL DEFAULT 'trial', trial_ends_at timestamptz NOT NULL DEFAULT now()+interval '14 days',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);
CREATE TABLE users (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), name text NOT NULL,
 email text NOT NULL UNIQUE, password_hash text NOT NULL,
 role text NOT NULL CHECK(role IN ('owner','manager','tire_tech','driver','inspector')),
 email_verified_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
 UNIQUE(company_id,id)
);
CREATE TABLE refresh_tokens (id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), user_id uuid NOT NULL, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, revoked_at timestamptz, FOREIGN KEY(company_id,user_id) REFERENCES users(company_id,id));
CREATE TABLE account_tokens (id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), user_id uuid NOT NULL, token_hash text NOT NULL UNIQUE, kind text NOT NULL CHECK(kind IN ('verify','reset','invite')), expires_at timestamptz NOT NULL, used_at timestamptz, FOREIGN KEY(company_id,user_id) REFERENCES users(company_id,id));
CREATE TABLE axle_templates (id text PRIMARY KEY, name text NOT NULL, positions jsonb NOT NULL);
INSERT INTO axle_templates VALUES
 ('tractor-6x2','Cavalo mecânico 6x2','["1EE","1DE","2EE","2EI","2DI","2DE","3EE","3EI","3DI","3DE","STEP1"]'),
 ('trailer-3','Carreta 3 eixos','["1EE","1EI","1DI","1DE","2EE","2EI","2DI","2DE","3EE","3EI","3DI","3DE","STEP1","STEP2"]');
CREATE TABLE vehicles (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), plate text NOT NULL, model text NOT NULL,
 template_id text NOT NULL REFERENCES axle_templates(id), current_km integer NOT NULL DEFAULT 0 CHECK(current_km>=0),
 status text NOT NULL DEFAULT 'ativo', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
 UNIQUE(company_id,id)
);
CREATE UNIQUE INDEX vehicles_plate ON vehicles(company_id,plate) WHERE deleted_at IS NULL;
CREATE TABLE tires (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), code text NOT NULL, brand text NOT NULL, model text NOT NULL DEFAULT '', size text NOT NULL,
 tread_mm numeric(4,1) NOT NULL CHECK(tread_mm BETWEEN 0 AND 40), acquisition_cost numeric(12,2) NOT NULL CHECK(acquisition_cost>=0),
 state text NOT NULL DEFAULT 'EM_ESTOQUE', vehicle_id uuid, position text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz,
 UNIQUE(company_id,id), FOREIGN KEY(company_id,vehicle_id) REFERENCES vehicles(company_id,id), CHECK((vehicle_id IS NULL)=(position IS NULL))
);
CREATE UNIQUE INDEX tires_code ON tires(company_id,code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tires_position ON tires(company_id,vehicle_id,position) WHERE deleted_at IS NULL AND vehicle_id IS NOT NULL;
CREATE TABLE inspections (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), tire_id uuid NOT NULL, user_id uuid NOT NULL, client_id uuid NOT NULL,
 tread_mm numeric(4,1) NOT NULL CHECK(tread_mm BETWEEN 0 AND 40), pressure_psi numeric(5,1) NOT NULL CHECK(pressure_psi BETWEEN 0 AND 200), structural_damage boolean NOT NULL,
 status text NOT NULL, level integer NOT NULL, recommendation text NOT NULL, inspected_at timestamptz NOT NULL DEFAULT now(), created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(company_id,tire_id) REFERENCES tires(company_id,id), FOREIGN KEY(company_id,user_id) REFERENCES users(company_id,id), UNIQUE(company_id,client_id)
);
CREATE TABLE alerts (id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), tire_id uuid NOT NULL, message text NOT NULL, severity text NOT NULL, read boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now(), FOREIGN KEY(company_id,tire_id) REFERENCES tires(company_id,id));
CREATE TABLE audit_log (id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), user_id uuid NOT NULL, entity text NOT NULL, entity_id uuid NOT NULL, action text NOT NULL, after_data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), FOREIGN KEY(company_id,user_id) REFERENCES users(company_id,id));
CREATE INDEX inspections_history ON inspections(company_id,tire_id,inspected_at DESC);
CREATE INDEX alerts_unread ON alerts(company_id,read,created_at DESC);
-- Authentication resolves the tenant before entering the scoped transaction. Private token/user
-- tables are accessed only by the trusted API connection; never grant that role to clients.
DO $$ DECLARE tab text; BEGIN
 FOREACH tab IN ARRAY ARRAY['vehicles','tires','inspections','alerts','audit_log'] LOOP
 EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tab);
 EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tab);
 EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (company_id = nullif(current_setting(''app.company_id'',true),)::uuid) WITH CHECK (company_id = nullif(current_setting(app.company_id,true),)::uuid)',tab);
 END LOOP;
END $$;
