-- +migrate Up
CREATE FUNCTION eterealink_notify_folder(folder_id uuid) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    IF folder_id IS NOT NULL THEN
        PERFORM pg_notify('eterealink_folder_events', folder_id::text);
    END IF;
END;
$$;

CREATE FUNCTION eterealink_files_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.upload_status = 'READY' THEN
            PERFORM eterealink_notify_folder(NEW.folder_id);
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.upload_status = 'READY' THEN
            PERFORM eterealink_notify_folder(OLD.folder_id);
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.folder_id IS DISTINCT FROM NEW.folder_id THEN
        IF OLD.upload_status = 'READY' THEN
            PERFORM eterealink_notify_folder(OLD.folder_id);
        END IF;
        IF NEW.upload_status = 'READY' THEN
            PERFORM eterealink_notify_folder(NEW.folder_id);
        END IF;
    ELSIF NEW.upload_status = 'READY' AND OLD.upload_status IS DISTINCT FROM NEW.upload_status THEN
        PERFORM eterealink_notify_folder(NEW.folder_id);
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER files_folder_changed
AFTER INSERT OR UPDATE OR DELETE ON files
FOR EACH ROW EXECUTE FUNCTION eterealink_files_changed();

CREATE FUNCTION eterealink_folders_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        PERFORM eterealink_notify_folder(NEW.parent_folder_id);
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        PERFORM eterealink_notify_folder(OLD.id);
        PERFORM eterealink_notify_folder(OLD.parent_folder_id);
        RETURN OLD;
    END IF;

    PERFORM eterealink_notify_folder(NEW.id);
    PERFORM eterealink_notify_folder(OLD.parent_folder_id);
    PERFORM eterealink_notify_folder(NEW.parent_folder_id);
    RETURN NEW;
END;
$$;

CREATE TRIGGER folders_changed
AFTER INSERT OR UPDATE OR DELETE ON folders
FOR EACH ROW EXECUTE FUNCTION eterealink_folders_changed();

CREATE FUNCTION eterealink_folder_members_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM eterealink_notify_folder(CASE WHEN TG_OP = 'DELETE' THEN OLD.folder_id ELSE NEW.folder_id END);
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER folder_members_changed
AFTER INSERT OR UPDATE OR DELETE ON folder_members
FOR EACH ROW EXECUTE FUNCTION eterealink_folder_members_changed();

CREATE FUNCTION eterealink_folder_invites_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM eterealink_notify_folder(CASE WHEN TG_OP = 'DELETE' THEN OLD.folder_id ELSE NEW.folder_id END);
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER folder_invites_changed
AFTER INSERT OR UPDATE OR DELETE ON folder_invites
FOR EACH ROW EXECUTE FUNCTION eterealink_folder_invites_changed();

CREATE FUNCTION eterealink_file_shares_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    target_file_id uuid;
    target_folder_id uuid;
BEGIN
    target_file_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.file_id ELSE NEW.file_id END;
    IF target_file_id IS NOT NULL THEN
        SELECT folder_id INTO target_folder_id FROM files WHERE id = target_file_id;
        PERFORM eterealink_notify_folder(target_folder_id);
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER file_shares_changed
AFTER INSERT OR UPDATE OR DELETE ON share_links
FOR EACH ROW EXECUTE FUNCTION eterealink_file_shares_changed();

-- +migrate Down
DROP TRIGGER IF EXISTS file_shares_changed ON share_links;
DROP FUNCTION IF EXISTS eterealink_file_shares_changed();
DROP TRIGGER IF EXISTS folder_invites_changed ON folder_invites;
DROP FUNCTION IF EXISTS eterealink_folder_invites_changed();
DROP TRIGGER IF EXISTS folder_members_changed ON folder_members;
DROP FUNCTION IF EXISTS eterealink_folder_members_changed();
DROP TRIGGER IF EXISTS folders_changed ON folders;
DROP FUNCTION IF EXISTS eterealink_folders_changed();
DROP TRIGGER IF EXISTS files_folder_changed ON files;
DROP FUNCTION IF EXISTS eterealink_files_changed();
DROP FUNCTION IF EXISTS eterealink_notify_folder(uuid);
