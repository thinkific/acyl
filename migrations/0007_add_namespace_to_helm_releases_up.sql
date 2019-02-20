ALTER TABLE helm_releases ADD COLUMN tiller_namespace VARCHAR(512) DEFAULT 'kube-system';
