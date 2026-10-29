DROP FUNCTION maintenance_companies();
DROP INDEX alert_active_rule;
DELETE FROM alerts WHERE tire_id IS NULL;
ALTER TABLE alerts ALTER COLUMN tire_id SET NOT NULL, DROP COLUMN kind,DROP COLUMN rule_key,DROP COLUMN resolved_at,DROP COLUMN updated_at;
DROP TABLE tire_movements,tire_operations,tire_policies;
ALTER TABLE tires DROP CONSTRAINT tire_lifecycle_state,DROP CONSTRAINT tire_lifecycle_assignment,DROP CONSTRAINT tire_mileage_assignment,
 DROP COLUMN measurement_cycle_at,DROP COLUMN version,DROP COLUMN stock_location,DROP COLUMN tracked_km,DROP COLUMN mounted_at_km,DROP COLUMN rotation_baseline_km,DROP COLUMN retread_count;
