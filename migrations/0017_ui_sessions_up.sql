CREATE TABLE ui_sessions (
    id bigserial PRIMARY KEY,
    github_user text, -- user will be empty prior to successful auth
    target_route text, -- the URL that the user attempted to access prior to authentication
    authenticated boolean, -- has the session been successfully authenticated & authorized?
    state bytea, -- random state data for oauth redirect
    client_ip text, -- not inet because github.com/lib/pq doesn't support that type
    user_agent text,
    created timestamptz NOT NULL DEFAULT now(),
    updated timestamptz,
    expires timestamptz NOT NULL
);

-- Trigger to automatically set the updated timestamp

CREATE TRIGGER set_updated_timestamp_ui_sessions
    BEFORE UPDATE ON ui_sessions
    FOR EACH ROW
EXECUTE PROCEDURE trigger_set_updated_timestamp();