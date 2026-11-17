ALTER TABLE tires
  DROP CONSTRAINT IF EXISTS tires_photo_data_len,
  DROP CONSTRAINT IF EXISTS tires_photo_thumb_len,
  DROP CONSTRAINT IF EXISTS tires_pressure_psi_range,
  DROP CONSTRAINT IF EXISTS tires_purchase_supplier_len,
  DROP CONSTRAINT IF EXISTS tires_notes_len;

ALTER TABLE tires
  DROP COLUMN IF EXISTS photo_data,
  DROP COLUMN IF EXISTS photo_thumb,
  DROP COLUMN IF EXISTS pressure_psi,
  DROP COLUMN IF EXISTS purchase_supplier,
  DROP COLUMN IF EXISTS notes;
