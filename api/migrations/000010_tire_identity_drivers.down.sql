ALTER TABLE tire_movements DROP COLUMN to_driver_name,DROP COLUMN from_driver_name,DROP COLUMN road_use,DROP COLUMN life_number;
ALTER TABLE vehicles DROP CONSTRAINT vehicle_driver_tenant,DROP COLUMN driver_id;
DROP TABLE drivers;
DROP INDEX tires_expiration;
ALTER TABLE tires DROP CONSTRAINT tire_retread_entry_count,DROP CONSTRAINT tire_dot_format,DROP COLUMN dot_code,DROP COLUMN serial_number,DROP COLUMN expires_on,DROP COLUMN road_use,DROP COLUMN initial_retread_count;
