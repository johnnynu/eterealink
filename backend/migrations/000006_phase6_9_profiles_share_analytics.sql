-- +migrate Up
ALTER TABLE users
    ADD COLUMN custom_display_name text,
    ADD CONSTRAINT users_custom_display_name_valid CHECK (
        custom_display_name IS NULL OR (
            char_length(custom_display_name) BETWEEN 3 AND 40
            AND custom_display_name = regexp_replace(btrim(custom_display_name), '[[:space:]]+', ' ', 'g')
            AND custom_display_name !~ '[[:cntrl:]]'
        )
    );

CREATE UNIQUE INDEX users_custom_display_name_unique
    ON users (lower(regexp_replace(btrim(custom_display_name), '[[:space:]]+', ' ', 'g')))
    WHERE custom_display_name IS NOT NULL;

CREATE FUNCTION eterealink_user_profile_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    affected_folder_id uuid;
BEGIN
    IF OLD.custom_display_name IS NOT DISTINCT FROM NEW.custom_display_name THEN
        RETURN NEW;
    END IF;

    FOR affected_folder_id IN
        SELECT id FROM folders WHERE owner_id = NEW.id
        UNION
        SELECT folder_id FROM folder_members WHERE user_id = NEW.id
        UNION
        SELECT folder_id FROM files WHERE owner_id = NEW.id AND folder_id IS NOT NULL
    LOOP
        PERFORM eterealink_notify_folder(affected_folder_id);
    END LOOP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_profile_changed
AFTER UPDATE OF custom_display_name ON users
FOR EACH ROW EXECUTE FUNCTION eterealink_user_profile_changed();

-- +migrate Down
DROP TRIGGER IF EXISTS users_profile_changed ON users;
DROP FUNCTION IF EXISTS eterealink_user_profile_changed();
DROP INDEX IF EXISTS users_custom_display_name_unique;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_custom_display_name_valid;
ALTER TABLE users DROP COLUMN IF EXISTS custom_display_name;
