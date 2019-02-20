ALTER TABLE environments DROP CONSTRAINT environments_name_key;
ALTER TABLE environments ADD COLUMN active BOOLEAN;
ALTER TABLE environments ADD CONSTRAINT unique_active UNIQUE (name, active);
ALTER TABLE environments ALTER COLUMN kube_namespace DROP NOT NULL;

ALTER TABLE env_created_events DROP CONSTRAINT env_created_events_name_key;
