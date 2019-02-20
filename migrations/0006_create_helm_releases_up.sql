CREATE TABLE helm_releases (
       id SERIAL PRIMARY KEY,
       environment_id INTEGER REFERENCES environments (id) NOT NULL,
       created_at TIMESTAMPTZ,
       updated_at TIMESTAMPTZ,
       deleted_at TIMESTAMPTZ,
       release_name VARCHAR(512) NOT NULL
);
