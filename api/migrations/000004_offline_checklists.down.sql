DROP TABLE sync_log,checklist_batches;
ALTER TABLE tires DROP COLUMN latest_inspected_at;
ALTER TABLE inspections DROP CONSTRAINT inspections_vehicle_tenant,DROP COLUMN vehicle_id,DROP COLUMN vehicle_km,DROP COLUMN temperature_c;
