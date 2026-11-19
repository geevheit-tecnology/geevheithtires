ALTER TABLE tires
  ADD COLUMN IF NOT EXISTS photo_data text,
  ADD COLUMN IF NOT EXISTS photo_thumb text,
  ADD COLUMN IF NOT EXISTS pressure_psi numeric(5,1),
  ADD COLUMN IF NOT EXISTS purchase_supplier text,
  ADD COLUMN IF NOT EXISTS notes text;

ALTER TABLE tires
  DROP CONSTRAINT IF EXISTS tires_photo_data_len,
  DROP CONSTRAINT IF EXISTS tires_photo_thumb_len,
  DROP CONSTRAINT IF EXISTS tires_pressure_psi_range,
  DROP CONSTRAINT IF EXISTS tires_purchase_supplier_len,
  DROP CONSTRAINT IF EXISTS tires_notes_len;

ALTER TABLE tires
  ADD CONSTRAINT tires_photo_data_len CHECK (photo_data IS NULL OR length(photo_data) <= 200000),
  ADD CONSTRAINT tires_photo_thumb_len CHECK (photo_thumb IS NULL OR length(photo_thumb) <= 20000),
  ADD CONSTRAINT tires_pressure_psi_range CHECK (pressure_psi IS NULL OR (pressure_psi >= 0 AND pressure_psi <= 250)),
  ADD CONSTRAINT tires_purchase_supplier_len CHECK (purchase_supplier IS NULL OR length(purchase_supplier) <= 150),
  ADD CONSTRAINT tires_notes_len CHECK (notes IS NULL OR length(notes) <= 500);
