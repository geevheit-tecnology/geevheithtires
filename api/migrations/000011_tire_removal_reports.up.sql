-- Structured assessment lives on the immutable movement and inherits its tenant policy.
ALTER TABLE tire_movements ADD COLUMN removal_report jsonb;
ALTER TABLE tire_movements ADD CONSTRAINT removal_report_object
 CHECK (removal_report IS NULL OR jsonb_typeof(removal_report)='object');
CREATE INDEX tire_movements_reporting ON tire_movements(company_id,created_at DESC,id DESC)
 WHERE action IN ('REMOVE','DISCARD','RECAP_SEND','RECAP_RETURN');
