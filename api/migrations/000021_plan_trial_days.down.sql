DROP FUNCTION IF EXISTS platform_save_plan(uuid,text,bigint,integer,integer,integer,boolean,boolean,text,text,integer);
ALTER TABLE billing_plans DROP COLUMN IF EXISTS trial_days;
