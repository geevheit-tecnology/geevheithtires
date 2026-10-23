-- Refuse rollback when a new configuration is in use (foreign key protection).
DELETE FROM axle_templates WHERE id NOT IN ('tractor-6x2','trailer-3');
ALTER TABLE axle_templates DROP COLUMN category;
