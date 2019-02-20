CREATE EXTENSION IF NOT EXISTS hstore;

CREATE TABLE qa_environments (
  name text PRIMARY KEY,
  created timestamptz NOT NULL,
  raw_events text[],
  hostname text,
  qa_type text,
  username text,
  repo text,
  pull_request integer,
  source_sha text,
  base_sha text,
  source_branch text,
  base_branch text,
  source_ref text,
  status integer,
  ref_map hstore,
  commit_sha_map hstore,
  amino_service_to_port hstore,
  amino_kubernetes_namespace text,
  amino_environment_id integer
);
