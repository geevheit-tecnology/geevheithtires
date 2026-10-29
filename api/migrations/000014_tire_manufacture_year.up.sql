-- Manufacturing year is the year encoded by the mandatory four-digit DOT suffix.
-- A generated field prevents year and DOT drifting apart, including old clients.
ALTER TABLE tires ADD COLUMN manufacture_year integer GENERATED ALWAYS AS
 (CASE WHEN dot_code ~ '[0-9]{4}$' THEN 2000 + right(dot_code,2)::integer ELSE NULL END) STORED;
