ALTER TABLE event_logs
  ADD COLUMN status jsonb,
  ADD COLUMN log_key uuid;