ALTER TABLE environments ADD COLUMN errored_at TIMESTAMPTZ;
ALTER TABLE environments ADD COLUMN error TEXT;
