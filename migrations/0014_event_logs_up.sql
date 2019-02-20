CREATE TABLE event_logs (
    id uuid PRIMARY KEY,
    created timestamptz NOT NULL DEFAULT now(),
    updated timestamptz,
    env_name text,  -- intentionally not setting foreign key constraint to prevent this table from interfering with qa_environments DROPs
    repo text,
    pull_request integer,
    webhook_payload bytea,
    log text[]
);

-- Trigger to automatically set the updated timestamp

CREATE TRIGGER set_updated_timestamp_event_logs
BEFORE UPDATE ON event_logs
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_updated_timestamp();