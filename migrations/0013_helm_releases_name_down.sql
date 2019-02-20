ALTER TABLE helm_releases
DROP COLUMN name,
ADD COLUMN repo text;
