-- +migrate Up
CREATE UNIQUE INDEX folders_owner_root_name_unique
    ON folders (owner_id, lower(name))
    WHERE parent_folder_id IS NULL;

CREATE UNIQUE INDEX folders_owner_parent_name_unique
    ON folders (owner_id, parent_folder_id, lower(name))
    WHERE parent_folder_id IS NOT NULL;

CREATE INDEX folder_members_folder_created_idx ON folder_members (folder_id, created_at);

-- +migrate Down
DROP INDEX IF EXISTS folder_members_folder_created_idx;
DROP INDEX IF EXISTS folders_owner_parent_name_unique;
DROP INDEX IF EXISTS folders_owner_root_name_unique;
