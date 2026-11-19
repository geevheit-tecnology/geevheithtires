ALTER TABLE tires ADD COLUMN version bigint NOT NULL DEFAULT 1,
 ADD COLUMN measurement_cycle_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN stock_location text NOT NULL DEFAULT 'Principal',
 ADD COLUMN tracked_km bigint NOT NULL DEFAULT 0 CHECK(tracked_km>=0),
 ADD COLUMN mounted_at_km integer CHECK(mounted_at_km>=0),
 ADD COLUMN rotation_baseline_km bigint NOT NULL DEFAULT 0 CHECK(rotation_baseline_km>=0),
 ADD COLUMN retread_count integer NOT NULL DEFAULT 0 CHECK(retread_count>=0);
UPDATE tires SET measurement_cycle_at=created_at;
-- Mileage before this migration is unknown. Start tracking from the current odometer.
UPDATE tires t SET mounted_at_km=v.current_km FROM vehicles v WHERE t.company_id=v.company_id AND t.vehicle_id=v.id;
ALTER TABLE tires ADD CONSTRAINT tire_lifecycle_state CHECK(state IN ('EM_ESTOQUE','INSTALADO','EM_RECAPAGEM','DESCARTADO')),
 ADD CONSTRAINT tire_lifecycle_assignment CHECK((state='INSTALADO')=(vehicle_id IS NOT NULL)),
 ADD CONSTRAINT tire_mileage_assignment CHECK((state='INSTALADO')=(mounted_at_km IS NOT NULL));
CREATE TABLE tire_operations (
 company_id uuid NOT NULL REFERENCES companies(id), client_id uuid NOT NULL, actor_id uuid NOT NULL,
 payload_hash text NOT NULL, response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(company_id,client_id), FOREIGN KEY(company_id,actor_id) REFERENCES users(company_id,id)
);
CREATE TABLE tire_movements (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), tire_id uuid NOT NULL, actor_id uuid,
 client_id uuid, action text NOT NULL, from_state text, to_state text NOT NULL,
 from_vehicle_id uuid, from_position text, to_vehicle_id uuid, to_position text,
 source_km integer, destination_km integer, tracked_km bigint NOT NULL DEFAULT 0,
 stock_location text NOT NULL DEFAULT '', reason text NOT NULL, supplier text NOT NULL DEFAULT '',
 cost_cents integer NOT NULL DEFAULT 0 CHECK(cost_cents>=0), tread_mm numeric(4,1) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(company_id,tire_id) REFERENCES tires(company_id,id),
 FOREIGN KEY(company_id,actor_id) REFERENCES users(company_id,id),
 FOREIGN KEY(company_id,from_vehicle_id) REFERENCES vehicles(company_id,id),
 FOREIGN KEY(company_id,to_vehicle_id) REFERENCES vehicles(company_id,id),
 UNIQUE(company_id,client_id,tire_id)
);
CREATE INDEX tire_movements_history ON tire_movements(company_id,tire_id,created_at DESC,id DESC);
INSERT INTO tire_movements(id,company_id,tire_id,action,to_state,to_vehicle_id,to_position,destination_km,stock_location,reason,tread_mm)
 SELECT gen_random_uuid(),company_id,id,'BASELINE',state,vehicle_id,position,mounted_at_km,stock_location,
 'Situação existente ao ativar o ciclo de vida; quilometragem e movimentações anteriores não foram estimadas.',tread_mm FROM tires;
CREATE TABLE tire_policies (
 company_id uuid PRIMARY KEY REFERENCES companies(id), warning_mm numeric(4,1) NOT NULL DEFAULT 5,
 critical_mm numeric(4,1) NOT NULL DEFAULT 3, rotation_km integer NOT NULL DEFAULT 10000,
 inspection_days integer NOT NULL DEFAULT 30, minimum_stock integer NOT NULL DEFAULT 2,
 max_retreads integer NOT NULL DEFAULT 2,
 CHECK(critical_mm>=1 AND warning_mm>critical_mm AND warning_mm<=20),
 CHECK(rotation_km BETWEEN 100 AND 500000), CHECK(inspection_days BETWEEN 1 AND 365),
 CHECK(minimum_stock BETWEEN 0 AND 10000), CHECK(max_retreads BETWEEN 0 AND 10)
);
ALTER TABLE alerts ALTER COLUMN tire_id DROP NOT NULL,
 ADD COLUMN kind text NOT NULL DEFAULT 'inspection', ADD COLUMN rule_key text,
 ADD COLUMN resolved_at timestamptz, ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
UPDATE alerts SET kind='legacy_inspection',resolved_at=now(),read=true;
CREATE UNIQUE INDEX alert_active_rule ON alerts(company_id,rule_key) WHERE resolved_at IS NULL AND rule_key IS NOT NULL;
DO $$ DECLARE tab text; BEGIN
 FOREACH tab IN ARRAY ARRAY['tire_operations','tire_movements','tire_policies'] LOOP
 EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tab);
 EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tab);
 EXECUTE format($p$CREATE POLICY tenant_isolation ON %I USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime') WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime')$p$,tab);
 END LOOP;
END $$;
GRANT SELECT,INSERT ON tire_operations,tire_movements TO geevheit_runtime;
GRANT SELECT,INSERT,UPDATE ON tire_policies TO geevheit_runtime;
-- Only the internal worker uses this tenant list; no public endpoint exposes it.
CREATE FUNCTION maintenance_companies() RETURNS TABLE(id uuid) LANGUAGE sql SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT id FROM companies WHERE deleted_at IS NULL $$;
REVOKE ALL ON FUNCTION maintenance_companies() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION maintenance_companies() TO geevheit_runtime;
