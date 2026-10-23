ALTER TABLE inspections ADD COLUMN vehicle_id uuid, ADD COLUMN vehicle_km integer CHECK(vehicle_km>=0), ADD COLUMN temperature_c numeric(4,1) CHECK(temperature_c BETWEEN -50 AND 200);
ALTER TABLE inspections ADD CONSTRAINT inspections_vehicle_tenant FOREIGN KEY(company_id,vehicle_id) REFERENCES vehicles(company_id,id);
ALTER TABLE tires ADD COLUMN latest_inspected_at timestamptz;
UPDATE tires t SET latest_inspected_at=(SELECT max(inspected_at) FROM inspections i WHERE i.company_id=t.company_id AND i.tire_id=t.id);
CREATE TABLE checklist_batches(id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id), user_id uuid NOT NULL, vehicle_id uuid NOT NULL, client_id uuid NOT NULL, payload_hash text NOT NULL, response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(company_id,client_id), FOREIGN KEY(company_id,user_id) REFERENCES users(company_id,id), FOREIGN KEY(company_id,vehicle_id) REFERENCES vehicles(company_id,id));
CREATE TABLE sync_log(id uuid PRIMARY KEY,company_id uuid NOT NULL REFERENCES companies(id),user_id uuid NOT NULL,inspection_id uuid NOT NULL,client_id uuid NOT NULL,outcome text NOT NULL CHECK(outcome IN ('applied','historical','replayed')),device_at timestamptz NOT NULL,received_at timestamptz NOT NULL DEFAULT now(),FOREIGN KEY(company_id,user_id) REFERENCES users(company_id,id));
CREATE INDEX sync_log_tenant ON sync_log(company_id,received_at DESC);
DO $$ DECLARE tab text; BEGIN
 FOREACH tab IN ARRAY ARRAY['checklist_batches','sync_log'] LOOP
 EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tab);
 EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',tab);
 EXECUTE format($p$CREATE POLICY tenant_isolation ON %I USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid) WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid)$p$,tab);
 EXECUTE format('GRANT SELECT,INSERT,UPDATE,DELETE ON %I TO geevheit_runtime',tab);
 END LOOP;
END $$;
