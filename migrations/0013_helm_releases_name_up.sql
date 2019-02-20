ALTER TABLE helm_releases
DROP COLUMN repo,
ADD COLUMN name text;
