-- +migrate Up
ALTER TABLE folder_members DROP CONSTRAINT folder_members_role_check;
ALTER TABLE folder_members
    ADD CONSTRAINT folder_members_role_check CHECK (role IN ('VIEWER', 'CONTRIBUTOR'));

CREATE TABLE folder_invites (
    id uuid PRIMARY KEY,
    folder_id uuid NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    short_code text NOT NULL UNIQUE CHECK (char_length(short_code) BETWEEN 8 AND 32),
    role text NOT NULL CHECK (role IN ('VIEWER', 'CONTRIBUTOR')),
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX folder_invites_folder_created_idx ON folder_invites (folder_id, created_at DESC);
CREATE INDEX folder_invites_code_active_idx ON folder_invites (short_code) WHERE revoked_at IS NULL;

ALTER TABLE folder_members
    ADD COLUMN expires_at timestamptz,
    ADD COLUMN invite_id uuid REFERENCES folder_invites(id) ON DELETE CASCADE;

CREATE INDEX folder_members_active_idx ON folder_members (user_id, folder_id, expires_at);

-- +migrate Down
DROP INDEX IF EXISTS folder_members_active_idx;
ALTER TABLE folder_members DROP COLUMN IF EXISTS invite_id;
ALTER TABLE folder_members DROP COLUMN IF EXISTS expires_at;
DROP TABLE IF EXISTS folder_invites;
ALTER TABLE folder_members DROP CONSTRAINT folder_members_role_check;
ALTER TABLE folder_members
    ADD CONSTRAINT folder_members_role_check CHECK (role = 'VIEWER');
