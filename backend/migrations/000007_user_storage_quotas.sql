-- +migrate Up
ALTER TABLE users
    ADD COLUMN storage_quota_bytes bigint,
    ADD COLUMN is_admin boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT users_storage_quota_bytes_positive CHECK (
        storage_quota_bytes IS NULL OR storage_quota_bytes > 0
    );

-- +migrate Down
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_storage_quota_bytes_positive,
    DROP COLUMN IF EXISTS is_admin,
    DROP COLUMN IF EXISTS storage_quota_bytes;
