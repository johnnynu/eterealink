-- +migrate Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    firebase_uid text NOT NULL UNIQUE,
    email text NOT NULL UNIQUE,
    display_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE folders (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_folder_id uuid REFERENCES folders(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT folders_no_self_parent CHECK (id <> parent_folder_id)
);

CREATE INDEX folders_owner_parent_idx ON folders (owner_id, parent_folder_id);

CREATE TABLE files (
    id uuid PRIMARY KEY,
    owner_id uuid REFERENCES users(id) ON DELETE CASCADE,
    folder_id uuid REFERENCES folders(id) ON DELETE SET NULL,
    storage_key text NOT NULL UNIQUE,
    original_name text NOT NULL CHECK (char_length(original_name) BETWEEN 1 AND 1024),
    mime_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    upload_status text NOT NULL DEFAULT 'PENDING'
        CHECK (upload_status IN ('PENDING', 'READY')),
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    expires_at timestamptz,
    CONSTRAINT anonymous_files_expire CHECK (owner_id IS NOT NULL OR expires_at IS NOT NULL),
    CONSTRAINT ready_files_completed CHECK (upload_status <> 'READY' OR completed_at IS NOT NULL)
);

CREATE INDEX files_owner_created_idx ON files (owner_id, created_at DESC);
CREATE INDEX files_folder_created_idx ON files (folder_id, created_at DESC);
CREATE INDEX files_expiration_idx ON files (expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE folder_members (
    folder_id uuid NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'VIEWER' CHECK (role = 'VIEWER'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (folder_id, user_id)
);

CREATE INDEX folder_members_user_idx ON folder_members (user_id);

CREATE TABLE share_links (
    id uuid PRIMARY KEY,
    short_code text NOT NULL UNIQUE CHECK (char_length(short_code) BETWEEN 8 AND 32),
    file_id uuid REFERENCES files(id) ON DELETE CASCADE,
    folder_id uuid REFERENCES folders(id) ON DELETE CASCADE,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT share_links_one_target CHECK ((file_id IS NULL) <> (folder_id IS NULL))
);

CREATE INDEX share_links_file_idx ON share_links (file_id) WHERE file_id IS NOT NULL;
CREATE INDEX share_links_folder_idx ON share_links (folder_id) WHERE folder_id IS NOT NULL;
CREATE INDEX share_links_expiration_idx ON share_links (expires_at) WHERE expires_at IS NOT NULL;

-- +migrate Down
DROP TABLE IF EXISTS share_links;
DROP TABLE IF EXISTS folder_members;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS users;

