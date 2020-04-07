ALTER TABLE event_logs
  ADD COLUMN github_delivery_id uuid;

CREATE INDEX IF NOT EXISTS github_delivery_id_idx ON event_logs
( github_delivery_id );
