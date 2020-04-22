CREATE TABLE ui_sessions (
    id bigserial PRIMARY KEY,
    key bytea,
    data bytea,
    state bytea,
    created timestamptz NOT NULL DEFAULT now(),
    updated timestamptz,
    expires timestamptz NOT NULL
);

-- Trigger to automatically set the updated timestamp

CREATE TRIGGER set_updated_timestamp_ui_sessions
    BEFORE UPDATE ON ui_sessions
    FOR EACH ROW
EXECUTE PROCEDURE trigger_set_updated_timestamp();