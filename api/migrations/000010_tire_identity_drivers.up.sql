-- Legacy records keep unknown DOT/expiry instead of inventing manufacturing data.
ALTER TABLE tires ADD COLUMN dot_code text,
 ADD COLUMN serial_number text CHECK(serial_number IS NULL OR length(serial_number) BETWEEN 2 AND 80),
 ADD COLUMN expires_on date,
 ADD COLUMN road_use text NOT NULL DEFAULT 'NAO_INFORMADO'
   CHECK(road_use IN ('NAO_INFORMADO','ASFALTO','TERRA','MISTO')),
 ADD COLUMN initial_retread_count integer NOT NULL DEFAULT 0 CHECK(initial_retread_count BETWEEN 0 AND 10),
 ADD CONSTRAINT tire_retread_entry_count CHECK(retread_count>=initial_retread_count),
 ADD CONSTRAINT tire_dot_format CHECK(dot_code IS NULL OR dot_code ~ '^[A-Z0-9]{0,28}[0-9]{4}$');
CREATE INDEX tires_expiration ON tires(company_id,expires_on) WHERE deleted_at IS NULL AND state<>'DESCARTADO';
CREATE TABLE drivers (
 id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies(id),
 name text NOT NULL CHECK(length(trim(name)) BETWEEN 2 AND 150),
 phone text NOT NULL DEFAULT '' CHECK(length(phone)<=30),
 license_number text NOT NULL DEFAULT '' CHECK(license_number='' OR license_number ~ '^[0-9]{11}$'),
 license_expires_on date, active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(company_id,id), CHECK(license_number='' OR license_expires_on IS NOT NULL)
);
CREATE UNIQUE INDEX driver_license_unique ON drivers(company_id,license_number) WHERE license_number<>'';
ALTER TABLE drivers ENABLE ROW LEVEL SECURITY;
ALTER TABLE drivers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON drivers
 USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime')
 WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime');
GRANT SELECT,INSERT,UPDATE ON drivers TO geevheit_runtime;
ALTER TABLE vehicles ADD COLUMN driver_id uuid,
 ADD CONSTRAINT vehicle_driver_tenant FOREIGN KEY(company_id,driver_id) REFERENCES drivers(company_id,id);
-- Driver names and life numbers are snapshots: later reassignment must not rewrite history.
ALTER TABLE tire_movements ADD COLUMN life_number integer,
 ADD COLUMN road_use text,
 ADD COLUMN from_driver_name text,
 ADD COLUMN to_driver_name text;
