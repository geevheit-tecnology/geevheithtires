ALTER TABLE companies
 ADD COLUMN legal_name text,
 ADD COLUMN cnpj_registration_status smallint,
 ADD COLUMN cnpj_verified_at timestamptz,
 ADD COLUMN cnpj_verification_source text,
 ADD COLUMN address jsonb NOT NULL DEFAULT '{}'::jsonb,
 ADD CONSTRAINT company_address_object CHECK (jsonb_typeof(address)='object');
