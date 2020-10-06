CREATE TYPE permissions_level AS ENUM ('read-only', 'read-write', 'admin');
CREATE TABLE api_keys (
    api_key uuid PRIMARY KEY,
    created_at TIMESTAMPTZ,
    last_used TIMESTAMPTZ,
    permissions permissions_level,
    name text,
    description text,
    github_user text
);
