CREATE TABLE helm_releases (
  id serial PRIMARY KEY,
  created timestamptz NOT NULL DEFAULT now(),
  env_name text REFERENCES qa_environments (name),
  k8s_namespace text,
  release text,
  repo text,
  revision_sha text
);

CREATE TABLE kubernetes_environments (
  env_name text PRIMARY KEY REFERENCES qa_environments (name),
  created timestamptz NOT NULL DEFAULT now(),
  updated timestamptz,
  namespace text,
  repo_config_yaml bytea,
  config_signature bytea,
  ref_map_json text,
  tiller_addr text
);

-- Trigger to automatically set the updated timestamp

CREATE OR REPLACE FUNCTION trigger_set_updated_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated_timestamp_k8s_envs
BEFORE UPDATE ON kubernetes_environments
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_updated_timestamp();
