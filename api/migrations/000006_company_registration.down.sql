ALTER TABLE companies DROP CONSTRAINT company_address_object,
 DROP COLUMN address, DROP COLUMN cnpj_verification_source,
 DROP COLUMN cnpj_verified_at, DROP COLUMN cnpj_registration_status, DROP COLUMN legal_name;
