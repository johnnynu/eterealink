-- +migrate Up
CREATE TABLE anonymous_transfers (
    id uuid PRIMARY KEY,
    upload_status text NOT NULL DEFAULT 'PENDING'
        CHECK (upload_status IN ('PENDING', 'READY')),
    archive_status text NOT NULL DEFAULT 'WAITING'
        CHECK (archive_status IN ('WAITING', 'PENDING', 'BUILDING', 'READY', 'FAILED')),
    archive_storage_key text NOT NULL UNIQUE,
    archive_size_bytes bigint CHECK (archive_size_bytes IS NULL OR archive_size_bytes > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    expires_at timestamptz NOT NULL,
    CONSTRAINT ready_transfers_completed CHECK (upload_status <> 'READY' OR completed_at IS NOT NULL),
    CONSTRAINT ready_archives_have_size CHECK (archive_status <> 'READY' OR archive_size_bytes IS NOT NULL)
);

CREATE INDEX anonymous_transfers_archive_queue_idx
    ON anonymous_transfers (created_at)
    WHERE archive_status = 'PENDING';
CREATE INDEX anonymous_transfers_expiration_idx ON anonymous_transfers (expires_at);

ALTER TABLE files
    ADD COLUMN transfer_id uuid REFERENCES anonymous_transfers(id) ON DELETE CASCADE;
CREATE INDEX files_transfer_idx ON files (transfer_id) WHERE transfer_id IS NOT NULL;

ALTER TABLE share_links
    DROP CONSTRAINT share_links_one_target,
    ADD COLUMN transfer_id uuid REFERENCES anonymous_transfers(id) ON DELETE CASCADE,
    ADD CONSTRAINT share_links_one_target
        CHECK (num_nonnulls(file_id, folder_id, transfer_id) = 1);
CREATE INDEX share_links_transfer_idx ON share_links (transfer_id) WHERE transfer_id IS NOT NULL;

-- +migrate Down
-- Bundle rows cannot satisfy the pre-transfer share target constraint, so the
-- rollback intentionally removes Phase 3 bundle data before restoring it.
DELETE FROM anonymous_transfers;
DROP INDEX IF EXISTS share_links_transfer_idx;
ALTER TABLE share_links
    DROP CONSTRAINT share_links_one_target,
    DROP COLUMN transfer_id,
    ADD CONSTRAINT share_links_one_target CHECK ((file_id IS NULL) <> (folder_id IS NULL));
DROP INDEX IF EXISTS files_transfer_idx;
ALTER TABLE files DROP COLUMN transfer_id;
DROP TABLE IF EXISTS anonymous_transfers;
