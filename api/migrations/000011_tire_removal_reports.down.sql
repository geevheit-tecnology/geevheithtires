DROP INDEX IF EXISTS tire_movements_reporting;
ALTER TABLE tire_movements DROP CONSTRAINT removal_report_object;
ALTER TABLE tire_movements DROP COLUMN removal_report;
