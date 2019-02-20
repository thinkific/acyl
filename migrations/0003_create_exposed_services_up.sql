CREATE TABLE exposed_services (
       id SERIAL PRIMARY KEY,
       environment_id INTEGER REFERENCES environments (id) NOT NULL,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ,
       service_name VARCHAR(512) NOT NULL,
       port INTEGER NOT NULL
);
