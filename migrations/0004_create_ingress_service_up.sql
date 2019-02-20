CREATE TABLE ingress_services (
       id SERIAL PRIMARY KEY,
       environment_id INTEGER REFERENCES environments (id) NOT NULL,
       dns_record_id INTEGER REFERENCES dns_records (id) NOT NULL,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ,
       active BOOLEAN DEFAULT true,
       service_name VARCHAR(512) NOT NULL,
       service_ip VARCHAR(512) NOT NULL,
       service_port INTEGER NOT NULL,
       host VARCHAR(512) NOT NULL
);
