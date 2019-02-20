CREATE TABLE environments (
       id SERIAL PRIMARY KEY,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ,
       name VARCHAR(512) UNIQUE,
       state VARCHAR(50),
       kube_namespace VARCHAR(512)
);

CREATE TABLE env_created_events (
       id SERIAL PRIMARY KEY,
       environment_id INTEGER REFERENCES environments (id) UNIQUE NOT NULL,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ,
       name VARCHAR(512) UNIQUE NOT NULL,
       kube_namespace VARCHAR(512) NOT NULL
);

CREATE TABLE env_destroyed_events (
       id SERIAL PRIMARY KEY,
       environment_id INTEGER REFERENCES environments (id) UNIQUE NOT NULL,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ
);

CREATE TABLE dns_records (
       id SERIAL PRIMARY KEY,
       environment_id INTEGER REFERENCES environments (id) NOT NULL,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ,
       deleted_at TIMESTAMPTZ,
       name VARCHAR(512) NOT NULL,
       type VARCHAR(512) NOT NULL,
       value VARCHAR(512) NOT NULL,
       zone_id VARCHAR(512) NOT NULL
);
