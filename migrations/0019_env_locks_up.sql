CREATE TABLE env_locks (
  lock_key bigint UNIQUE,
  repo text,
  pull_request integer,
  PRIMARY KEY(repo, pull_request)
);


CREATE OR REPLACE FUNCTION random_bigint()
RETURNS bigint AS $$
BEGIN
  RETURN floor(random()*(9223372036854775807)); -- between 0 - max bigint
END;
$$ LANGUAGE plpgsql;
