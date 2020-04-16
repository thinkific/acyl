DROP INDEX IF EXISTS github_delivery_id_idx;

ALTER TABLE event_logs
    DROP COLUMN github_delivery_id;
