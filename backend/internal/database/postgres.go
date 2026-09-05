package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	config.MaxConns = 8
	config.MinConns = 0
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	database := &Postgres{pool: pool}
	if err := database.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return database, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) Ping(ctx context.Context) error {
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func (p *Postgres) UpsertUser(ctx context.Context, user domain.User) (domain.User, error) {
	row := p.pool.QueryRow(ctx, `
		INSERT INTO users (id, firebase_uid, email, display_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (firebase_uid) DO UPDATE
		SET email = EXCLUDED.email, display_name = EXCLUDED.display_name
		RETURNING id, firebase_uid, email, display_name, created_at`,
		user.ID, user.FirebaseUID, user.Email, user.DisplayName, user.CreatedAt,
	)
	if err := row.Scan(&user.ID, &user.FirebaseUID, &user.Email, &user.DisplayName, &user.CreatedAt); err != nil {
		return domain.User{}, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

func (p *Postgres) CreateAnonymousUpload(ctx context.Context, file domain.File, share domain.ShareLink) error {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO files (
			id, owner_id, folder_id, storage_key, original_name, mime_type,
			size_bytes, upload_status, created_at, expires_at
		) VALUES ($1, NULL, NULL, $2, $3, $4, $5, $6, $7, $8)`,
		file.ID, file.StorageKey, file.OriginalName, file.MIMEType,
		file.SizeBytes, file.Status, file.CreatedAt, file.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert file: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO share_links (
			id, short_code, file_id, folder_id, created_by, expires_at, created_at
		) VALUES ($1, $2, $3, NULL, NULL, $4, $5)`,
		share.ID, share.ShortCode, share.FileID, share.ExpiresAt, share.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert share link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (p *Postgres) GetUpload(ctx context.Context, fileID string, now time.Time) (domain.File, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		       size_bytes, upload_status, created_at, completed_at, expires_at
		FROM files
		WHERE id = $1
		  AND upload_status IN ('PENDING', 'READY')
		  AND (expires_at IS NULL OR expires_at > $2)`,
		fileID, now,
	)

	file, err := scanFile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("get upload: %w", err)
	}
	return file, nil
}

func (p *Postgres) CompleteUpload(ctx context.Context, fileID string, now time.Time) (domain.File, error) {
	row := p.pool.QueryRow(ctx, `
		UPDATE files
		SET upload_status = 'READY', completed_at = COALESCE(completed_at, $2)
		WHERE id = $1
		  AND upload_status IN ('PENDING', 'READY')
		  AND (expires_at IS NULL OR expires_at > $2)
		RETURNING id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		          size_bytes, upload_status, created_at, completed_at, expires_at`,
		fileID, now,
	)

	file, err := scanFile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("complete upload: %w", err)
	}
	return file, nil
}

func (p *Postgres) CreateOwnedFile(ctx context.Context, file domain.File) error {
	var tag pgconn.CommandTag
	var err error
	if file.FolderID == nil {
		tag, err = p.pool.Exec(ctx, `
		INSERT INTO files (
			id, owner_id, folder_id, storage_key, original_name, mime_type,
			size_bytes, upload_status, created_at, expires_at
		) VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $8, NULL)`,
			file.ID, file.OwnerID, file.StorageKey, file.OriginalName, file.MIMEType,
			file.SizeBytes, file.Status, file.CreatedAt,
		)
	} else {
		tag, err = p.pool.Exec(ctx, `
			INSERT INTO files (
				id, owner_id, folder_id, storage_key, original_name, mime_type,
				size_bytes, upload_status, created_at, expires_at
			)
			SELECT $1, $2, f.id, $4, $5, $6, $7, $8, $9, NULL
			FROM folders f WHERE f.id = $3 AND f.owner_id = $2`,
			file.ID, file.OwnerID, file.FolderID, file.StorageKey, file.OriginalName, file.MIMEType,
			file.SizeBytes, file.Status, file.CreatedAt)
	}
	if databaseConflict(err) {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("insert owned file: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateOwnedFileWithinQuota(ctx context.Context, file domain.File, maxAccountBytes int64) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin quota reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerExists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM users WHERE id = $1 FOR UPDATE`, file.OwnerID).Scan(&ownerExists); errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("lock file owner: %w", err)
	}
	var reservedBytes int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(size_bytes), 0) FROM files WHERE owner_id = $1`, file.OwnerID).Scan(&reservedBytes); err != nil {
		return fmt.Errorf("get reserved file usage: %w", err)
	}
	if reservedBytes > maxAccountBytes-file.SizeBytes {
		return domain.ErrQuotaExceeded
	}

	var tag pgconn.CommandTag
	if file.FolderID == nil {
		tag, err = tx.Exec(ctx, `
			INSERT INTO files (
				id, owner_id, folder_id, storage_key, original_name, mime_type,
				size_bytes, upload_status, created_at, expires_at
			) VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $8, NULL)`,
			file.ID, file.OwnerID, file.StorageKey, file.OriginalName, file.MIMEType,
			file.SizeBytes, file.Status, file.CreatedAt)
	} else {
		tag, err = tx.Exec(ctx, `
			INSERT INTO files (
				id, owner_id, folder_id, storage_key, original_name, mime_type,
				size_bytes, upload_status, created_at, expires_at
			)
			SELECT $1, $2, f.id, $4, $5, $6, $7, $8, $9, NULL
			FROM folders f WHERE f.id = $3 AND f.owner_id = $2`,
			file.ID, file.OwnerID, file.FolderID, file.StorageKey, file.OriginalName, file.MIMEType,
			file.SizeBytes, file.Status, file.CreatedAt)
	}
	if databaseConflict(err) {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("insert quota-reserved file: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit quota reservation: %w", err)
	}
	return nil
}

func (p *Postgres) GetOwnedFile(ctx context.Context, ownerID, fileID string) (domain.File, error) {
	file, err := scanFile(p.pool.QueryRow(ctx, `
		SELECT id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		       size_bytes, upload_status, created_at, completed_at, expires_at
		FROM files
		WHERE id = $1 AND owner_id = $2`, fileID, ownerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("get owned file: %w", err)
	}
	return file, nil
}

func (p *Postgres) CompleteOwnedFile(ctx context.Context, ownerID, fileID string, now time.Time) (domain.File, error) {
	file, err := scanFile(p.pool.QueryRow(ctx, `
		UPDATE files
		SET upload_status = 'READY', completed_at = COALESCE(completed_at, $3)
		WHERE id = $1 AND owner_id = $2 AND upload_status IN ('PENDING', 'READY')
		RETURNING id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		          size_bytes, upload_status, created_at, completed_at, expires_at`,
		fileID, ownerID, now,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("complete owned file: %w", err)
	}
	return file, nil
}

func (p *Postgres) ListOwnedFiles(ctx context.Context, ownerID string, now time.Time) ([]domain.OwnedFile, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		       size_bytes, upload_status, created_at, completed_at, expires_at
		FROM files
		WHERE owner_id = $1 AND upload_status = 'READY'
		ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list owned files: %w", err)
	}
	defer rows.Close()

	files := make([]domain.OwnedFile, 0)
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan owned file: %w", err)
		}
		files = append(files, domain.OwnedFile{File: file})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list owned files: %w", err)
	}
	rows.Close()

	shareRows, err := p.pool.Query(ctx, `
		SELECT s.id, s.short_code, s.file_id, s.folder_id, s.transfer_id, s.created_by,
		       s.created_at, s.expires_at, s.revoked_at
		FROM share_links s
		JOIN files f ON f.id = s.file_id
		WHERE f.owner_id = $1
		  AND s.revoked_at IS NULL
		  AND (s.expires_at IS NULL OR s.expires_at > $2)
		ORDER BY s.created_at DESC`, ownerID, now)
	if err != nil {
		return nil, fmt.Errorf("list owned file shares: %w", err)
	}
	defer shareRows.Close()
	sharesByFile := make(map[string]domain.ShareLink)
	for shareRows.Next() {
		var share domain.ShareLink
		if err := shareRows.Scan(
			&share.ID, &share.ShortCode, &share.FileID, &share.FolderID, &share.TransferID,
			&share.CreatedBy, &share.CreatedAt, &share.ExpiresAt, &share.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan owned file share: %w", err)
		}
		if share.FileID != nil {
			if _, exists := sharesByFile[*share.FileID]; !exists {
				sharesByFile[*share.FileID] = share
			}
		}
	}
	if err := shareRows.Err(); err != nil {
		return nil, fmt.Errorf("list owned file shares: %w", err)
	}
	for index := range files {
		if share, ok := sharesByFile[files[index].File.ID]; ok {
			files[index].Share = &share
		}
	}
	return files, nil
}

func (p *Postgres) GetOwnedFileUsage(ctx context.Context, ownerID string) (domain.FileLibrarySummary, error) {
	var summary domain.FileLibrarySummary
	if err := p.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(size_bytes), 0)
		FROM files
		WHERE owner_id = $1 AND upload_status = 'READY'`, ownerID).Scan(&summary.FileCount, &summary.TotalBytes); err != nil {
		return domain.FileLibrarySummary{}, fmt.Errorf("get owned file usage: %w", err)
	}
	return summary, nil
}

func (p *Postgres) CreateOwnedFileShare(ctx context.Context, ownerID, fileID string, share domain.ShareLink, now time.Time) error {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin owned file share transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status domain.FileStatus
	err = tx.QueryRow(ctx, `
		SELECT upload_status FROM files
		WHERE id = $1 AND owner_id = $2
		FOR UPDATE`, fileID, ownerID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock owned file for sharing: %w", err)
	}
	if status != domain.FileStatusReady {
		return domain.ErrConflict
	}

	var hasActiveShare bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM share_links
			WHERE file_id = $1 AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > $2)
		)`, fileID, now).Scan(&hasActiveShare); err != nil {
		return fmt.Errorf("check active owned file share: %w", err)
	}
	if hasActiveShare {
		return domain.ErrConflict
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO share_links (
			id, short_code, file_id, folder_id, transfer_id, created_by, expires_at, created_at
		) VALUES ($1, $2, $3, NULL, NULL, $4, $5, $6)`,
		share.ID, share.ShortCode, fileID, ownerID, share.ExpiresAt, share.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert owned file share: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit owned file share: %w", err)
	}
	return nil
}

func (p *Postgres) RevokeOwnedFileShare(ctx context.Context, ownerID, fileID, shareID string, now time.Time) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE share_links AS s
		SET revoked_at = $4
		FROM files AS f
		WHERE s.id = $1 AND s.file_id = $2
		  AND f.id = s.file_id AND f.owner_id = $3
		  AND s.revoked_at IS NULL
		  AND (s.expires_at IS NULL OR s.expires_at > $4)`,
		shareID, fileID, ownerID, now)
	if err != nil {
		return fmt.Errorf("revoke owned file share: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteOwnedFile(ctx context.Context, ownerID, fileID string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM files WHERE id = $1 AND owner_id = $2`, fileID, ownerID)
	if err != nil {
		return fmt.Errorf("delete owned file: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func databaseConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503" || pgErr.Code == "23514")
}

func (p *Postgres) CreateFolder(ctx context.Context, folder domain.Folder) error {
	var tag pgconn.CommandTag
	var err error
	if folder.ParentFolderID == nil {
		tag, err = p.pool.Exec(ctx, `
			INSERT INTO folders (id, owner_id, parent_folder_id, name, created_at)
			VALUES ($1, $2, NULL, $3, $4)`, folder.ID, folder.OwnerID, folder.Name, folder.CreatedAt)
	} else {
		tag, err = p.pool.Exec(ctx, `
			INSERT INTO folders (id, owner_id, parent_folder_id, name, created_at)
			SELECT $1, $2, id, $4, $5 FROM folders WHERE id = $3 AND owner_id = $2`,
			folder.ID, folder.OwnerID, folder.ParentFolderID, folder.Name, folder.CreatedAt)
	}
	if databaseConflict(err) {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) GetRootContents(ctx context.Context, userID, scope string, now time.Time, query domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	result := domain.FolderContents{
		Breadcrumbs: []domain.Folder{}, Folders: []domain.FolderAccess{}, Files: []domain.OwnedFile{},
	}
	if scope == "shared" {
		rows, err := p.pool.Query(ctx, `
			SELECT f.id, f.owner_id, f.parent_folder_id, f.name, f.created_at,
			       u.id, u.firebase_uid, u.email, u.display_name, u.created_at
			FROM folder_members m
			JOIN folders f ON f.id = m.folder_id
			JOIN users u ON u.id = f.owner_id
			WHERE m.user_id = $1
			ORDER BY lower(f.name), f.id`, userID)
		if err != nil {
			return result, false, fmt.Errorf("list shared folders: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var access domain.FolderAccess
			access.Role = domain.FolderRoleViewer
			if err := rows.Scan(&access.Folder.ID, &access.Folder.OwnerID, &access.Folder.ParentFolderID, &access.Folder.Name, &access.Folder.CreatedAt,
				&access.Owner.ID, &access.Owner.FirebaseUID, &access.Owner.Email, &access.Owner.DisplayName, &access.Owner.CreatedAt); err != nil {
				return result, false, fmt.Errorf("scan shared folder: %w", err)
			}
			result.Folders = append(result.Folders, access)
		}
		return result, false, rows.Err()
	}

	owner, err := p.userByID(ctx, userID)
	if err != nil {
		return result, false, err
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, owner_id, parent_folder_id, name, created_at
		FROM folders WHERE owner_id = $1 AND parent_folder_id IS NULL
		ORDER BY lower(name), id`, userID)
	if err != nil {
		return result, false, fmt.Errorf("list root folders: %w", err)
	}
	for rows.Next() {
		var folder domain.Folder
		if err := rows.Scan(&folder.ID, &folder.OwnerID, &folder.ParentFolderID, &folder.Name, &folder.CreatedAt); err != nil {
			rows.Close()
			return result, false, fmt.Errorf("scan root folder: %w", err)
		}
		result.Folders = append(result.Folders, domain.FolderAccess{Folder: folder, Role: domain.FolderRoleOwner, Owner: owner})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, false, fmt.Errorf("list root folders: %w", err)
	}
	rows.Close()
	files, hasMore, err := p.listFolderFiles(ctx, userID, nil, now, query)
	if err != nil {
		return result, false, err
	}
	result.Files = files
	result.Summary, err = p.GetOwnedFileUsage(ctx, userID)
	return result, hasMore, err
}

func (p *Postgres) GetFolderContents(ctx context.Context, userID, folderID string, now time.Time, query domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	result := domain.FolderContents{Breadcrumbs: []domain.Folder{}, Folders: []domain.FolderAccess{}, Files: []domain.OwnedFile{}}
	rows, err := p.pool.Query(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT f.id, f.owner_id, f.parent_folder_id, f.name, f.created_at, 0 AS depth
			FROM folders f WHERE f.id = $1
			UNION ALL
			SELECT p.id, p.owner_id, p.parent_folder_id, p.name, p.created_at, a.depth + 1
			FROM folders p JOIN ancestors a ON a.parent_folder_id = p.id
		)
		SELECT a.id, a.owner_id, a.parent_folder_id, a.name, a.created_at, a.depth,
		       EXISTS (SELECT 1 FROM folder_members m WHERE m.folder_id = a.id AND m.user_id = $2)
		FROM ancestors a ORDER BY a.depth DESC`, folderID, userID)
	if err != nil {
		return result, false, fmt.Errorf("get folder ancestry: %w", err)
	}
	type ancestor struct {
		folder domain.Folder
		shared bool
	}
	ancestors := make([]ancestor, 0)
	for rows.Next() {
		var item ancestor
		var depth int
		if err := rows.Scan(&item.folder.ID, &item.folder.OwnerID, &item.folder.ParentFolderID, &item.folder.Name, &item.folder.CreatedAt, &depth, &item.shared); err != nil {
			rows.Close()
			return result, false, fmt.Errorf("scan folder ancestry: %w", err)
		}
		ancestors = append(ancestors, item)
	}
	rows.Close()
	if len(ancestors) == 0 {
		return result, false, domain.ErrNotFound
	}
	owner, err := p.userByID(ctx, ancestors[len(ancestors)-1].folder.OwnerID)
	if err != nil {
		return result, false, err
	}
	role := domain.FolderRoleViewer
	breadcrumbStart := -1
	if owner.ID == userID {
		role = domain.FolderRoleOwner
		breadcrumbStart = 0
	} else {
		for index, item := range ancestors {
			if item.shared {
				breadcrumbStart = index
				break
			}
		}
	}
	if breadcrumbStart < 0 {
		return result, false, domain.ErrNotFound
	}
	for _, item := range ancestors[breadcrumbStart:] {
		result.Breadcrumbs = append(result.Breadcrumbs, item.folder)
	}
	current := ancestors[len(ancestors)-1].folder
	result.Current = &domain.FolderAccess{Folder: current, Role: role, Owner: owner}

	childRows, err := p.pool.Query(ctx, `
		SELECT id, owner_id, parent_folder_id, name, created_at
		FROM folders WHERE parent_folder_id = $1 ORDER BY lower(name), id`, folderID)
	if err != nil {
		return result, false, fmt.Errorf("list child folders: %w", err)
	}
	for childRows.Next() {
		var folder domain.Folder
		if err := childRows.Scan(&folder.ID, &folder.OwnerID, &folder.ParentFolderID, &folder.Name, &folder.CreatedAt); err != nil {
			childRows.Close()
			return result, false, fmt.Errorf("scan child folder: %w", err)
		}
		result.Folders = append(result.Folders, domain.FolderAccess{Folder: folder, Role: role, Owner: owner})
	}
	childRows.Close()
	var hasMore bool
	result.Files, hasMore, err = p.listFolderFiles(ctx, owner.ID, &folderID, now, query)
	if err != nil {
		return result, false, err
	}
	if role == domain.FolderRoleOwner {
		result.Summary, err = p.GetOwnedFileUsage(ctx, owner.ID)
		return result, hasMore, err
	}
	for _, file := range result.Files {
		result.Summary.FileCount++
		result.Summary.TotalBytes += file.File.SizeBytes
	}
	return result, hasMore, nil
}

func (p *Postgres) userByID(ctx context.Context, userID string) (domain.User, error) {
	var user domain.User
	err := p.pool.QueryRow(ctx, `SELECT id, firebase_uid, email, display_name, created_at FROM users WHERE id = $1`, userID).
		Scan(&user.ID, &user.FirebaseUID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (p *Postgres) listFolderFiles(ctx context.Context, ownerID string, folderID *string, now time.Time, query domain.FileLibraryQuery) ([]domain.OwnedFile, bool, error) {
	statement := `
		SELECT f.id, f.owner_id, f.folder_id, f.transfer_id, f.storage_key, f.original_name, f.mime_type,
		       f.size_bytes, f.upload_status, f.created_at, f.completed_at, f.expires_at,
		       s.id, s.short_code, s.file_id, s.folder_id, s.transfer_id, s.created_by, s.created_at, s.expires_at, s.revoked_at
		FROM files f
		LEFT JOIN LATERAL (
			SELECT * FROM share_links sl WHERE sl.file_id = f.id AND sl.revoked_at IS NULL
			  AND (sl.expires_at IS NULL OR sl.expires_at > @now) ORDER BY sl.created_at DESC LIMIT 1
		) s ON true
		WHERE f.owner_id = @owner_id AND f.folder_id IS NOT DISTINCT FROM @folder_id AND f.upload_status = 'READY'
		  AND (@search = '' OR f.original_name ILIKE '%' || @search || '%')
		  AND (NOT @shared_only OR s.id IS NOT NULL)`
	arguments := pgx.NamedArgs{
		"owner_id": ownerID, "folder_id": folderID, "now": now, "search": query.Search,
		"shared_only": query.SharedOnly, "limit": query.Limit + 1,
		"cursor_id": query.CursorID, "cursor_time": query.CursorTime, "cursor_name": query.CursorName, "cursor_size": query.CursorSize,
	}
	switch query.Sort {
	case "oldest":
		if query.CursorID != "" {
			statement += ` AND (f.created_at, f.id) > (@cursor_time, @cursor_id)`
		}
		statement += ` ORDER BY f.created_at ASC, f.id ASC`
	case "name":
		if query.CursorID != "" {
			statement += ` AND (lower(f.original_name), f.id) > (@cursor_name, @cursor_id)`
		}
		statement += ` ORDER BY lower(f.original_name) ASC, f.id ASC`
	case "size":
		if query.CursorID != "" {
			statement += ` AND (f.size_bytes, f.id) < (@cursor_size, @cursor_id)`
		}
		statement += ` ORDER BY f.size_bytes DESC, f.id DESC`
	default:
		if query.CursorID != "" {
			statement += ` AND (f.created_at, f.id) < (@cursor_time, @cursor_id)`
		}
		statement += ` ORDER BY f.created_at DESC, f.id DESC`
	}
	statement += ` LIMIT @limit`
	rows, err := p.pool.Query(ctx, statement, arguments)
	if err != nil {
		return nil, false, fmt.Errorf("list folder files: %w", err)
	}
	defer rows.Close()
	result := make([]domain.OwnedFile, 0)
	for rows.Next() {
		var item domain.OwnedFile
		var shareID, shortCode, shareFileID, shareFolderID, shareTransferID, createdBy *string
		var shareCreated, shareExpires, shareRevoked *time.Time
		if err := rows.Scan(&item.File.ID, &item.File.OwnerID, &item.File.FolderID, &item.File.TransferID, &item.File.StorageKey,
			&item.File.OriginalName, &item.File.MIMEType, &item.File.SizeBytes, &item.File.Status, &item.File.CreatedAt,
			&item.File.CompletedAt, &item.File.ExpiresAt, &shareID, &shortCode, &shareFileID, &shareFolderID,
			&shareTransferID, &createdBy, &shareCreated, &shareExpires, &shareRevoked); err != nil {
			return nil, false, fmt.Errorf("scan folder file: %w", err)
		}
		if shareID != nil {
			item.Share = &domain.ShareLink{ID: *shareID, ShortCode: *shortCode, FileID: shareFileID, FolderID: shareFolderID,
				TransferID: shareTransferID, CreatedBy: createdBy, CreatedAt: *shareCreated, ExpiresAt: shareExpires, RevokedAt: shareRevoked}
			item.SharePath = "/s/" + *shortCode
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(result) > query.Limit
	if hasMore {
		result = result[:query.Limit]
	}
	return result, hasMore, nil
}

func (p *Postgres) UpdateFolder(ctx context.Context, ownerID, folderID, name string, parentFolderID *string) (domain.Folder, error) {
	if parentFolderID != nil {
		var invalid bool
		err := p.pool.QueryRow(ctx, `
			WITH RECURSIVE descendants AS (
				SELECT id FROM folders WHERE id = $1 AND owner_id = $2
				UNION ALL SELECT f.id FROM folders f JOIN descendants d ON f.parent_folder_id = d.id
			)
			SELECT EXISTS (SELECT 1 FROM descendants WHERE id = $3)
			    OR NOT EXISTS (SELECT 1 FROM folders WHERE id = $3 AND owner_id = $2)`, folderID, ownerID, parentFolderID).Scan(&invalid)
		if err != nil {
			return domain.Folder{}, fmt.Errorf("validate folder move: %w", err)
		}
		if invalid {
			return domain.Folder{}, serviceInvalidFolderMove()
		}
	}
	var folder domain.Folder
	err := p.pool.QueryRow(ctx, `
		UPDATE folders SET name = $3, parent_folder_id = $4
		WHERE id = $1 AND owner_id = $2
		RETURNING id, owner_id, parent_folder_id, name, created_at`, folderID, ownerID, name, parentFolderID).
		Scan(&folder.ID, &folder.OwnerID, &folder.ParentFolderID, &folder.Name, &folder.CreatedAt)
	if databaseConflict(err) {
		return domain.Folder{}, domain.ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Folder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Folder{}, fmt.Errorf("update folder: %w", err)
	}
	return folder, nil
}

// This local sentinel avoids making the database package depend on the service package.
func serviceInvalidFolderMove() error { return domain.ErrConflict }

func (p *Postgres) DeleteFolder(ctx context.Context, ownerID, folderID string) error {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM folders f WHERE f.id = $1 AND f.owner_id = $2
		  AND NOT EXISTS (SELECT 1 FROM folders c WHERE c.parent_folder_id = f.id)
		  AND NOT EXISTS (SELECT 1 FROM files x WHERE x.folder_id = f.id)`, folderID, ownerID)
	if err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM folders WHERE id = $1 AND owner_id = $2)`, folderID, ownerID).Scan(&exists); err != nil {
		return fmt.Errorf("check folder deletion: %w", err)
	}
	if exists {
		return domain.ErrConflict
	}
	return domain.ErrNotFound
}

func (p *Postgres) ListFolderMembers(ctx context.Context, ownerID, folderID string) ([]domain.FolderMember, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT u.id, u.firebase_uid, u.email, u.display_name, u.created_at, m.role, m.created_at
		FROM folder_members m JOIN users u ON u.id = m.user_id JOIN folders f ON f.id = m.folder_id
		WHERE m.folder_id = $1 AND f.owner_id = $2 ORDER BY lower(u.email)`, folderID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list folder members: %w", err)
	}
	defer rows.Close()
	result := make([]domain.FolderMember, 0)
	for rows.Next() {
		var member domain.FolderMember
		if err := rows.Scan(&member.User.ID, &member.User.FirebaseUID, &member.User.Email, &member.User.DisplayName,
			&member.User.CreatedAt, &member.Role, &member.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan folder member: %w", err)
		}
		result = append(result, member)
	}
	return result, rows.Err()
}

func (p *Postgres) AddFolderMember(ctx context.Context, ownerID, folderID, email string, createdAt time.Time) (domain.FolderMember, error) {
	var member domain.FolderMember
	err := p.pool.QueryRow(ctx, `
		INSERT INTO folder_members (folder_id, user_id, role, created_at)
		SELECT f.id, u.id, 'VIEWER', $4 FROM folders f JOIN users u ON lower(u.email) = lower($3)
		WHERE f.id = $1 AND f.owner_id = $2 AND u.id <> $2
		ON CONFLICT (folder_id, user_id) DO UPDATE SET role = 'VIEWER'
		RETURNING user_id, role, created_at`, folderID, ownerID, email, createdAt).
		Scan(&member.User.ID, &member.Role, &member.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.FolderMember{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.FolderMember{}, fmt.Errorf("add folder member: %w", err)
	}
	member.User, err = p.userByID(ctx, member.User.ID)
	return member, err
}

func (p *Postgres) RemoveFolderMember(ctx context.Context, ownerID, folderID, userID string) error {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM folder_members m USING folders f
		WHERE m.folder_id = $1 AND m.user_id = $2 AND f.id = m.folder_id AND f.owner_id = $3`, folderID, userID, ownerID)
	if err != nil {
		return fmt.Errorf("remove folder member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) MoveOwnedFiles(ctx context.Context, ownerID string, fileIDs []string, folderID *string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin move files: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if folderID != nil {
		var owns bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM folders WHERE id = $1 AND owner_id = $2)`, folderID, ownerID).Scan(&owns); err != nil {
			return fmt.Errorf("validate move destination: %w", err)
		}
		if !owns {
			return domain.ErrNotFound
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE files SET folder_id = $3 WHERE owner_id = $1 AND id = ANY($2)`, ownerID, fileIDs, folderID)
	if databaseConflict(err) {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("move files: %w", err)
	}
	if tag.RowsAffected() != int64(len(fileIDs)) {
		return domain.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit move files: %w", err)
	}
	return nil
}

func (p *Postgres) GetAccessibleFile(ctx context.Context, userID, fileID string) (domain.File, error) {
	file, err := scanFile(p.pool.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT fo.id, fo.parent_folder_id FROM folders fo JOIN files fi ON fi.folder_id = fo.id WHERE fi.id = $1
			UNION ALL SELECT p.id, p.parent_folder_id FROM folders p JOIN ancestors a ON a.parent_folder_id = p.id
		)
		SELECT f.id, f.owner_id, f.folder_id, f.transfer_id, f.storage_key, f.original_name, f.mime_type,
		       f.size_bytes, f.upload_status, f.created_at, f.completed_at, f.expires_at
		FROM files f WHERE f.id = $1 AND (f.owner_id = $2 OR EXISTS (
			SELECT 1 FROM folder_members m JOIN ancestors a ON a.id = m.folder_id WHERE m.user_id = $2
		))`, fileID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("get accessible file: %w", err)
	}
	return file, nil
}

func (p *Postgres) ResolveFileShare(ctx context.Context, code string, now time.Time) (domain.SharedFile, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT
			s.id, s.short_code, s.file_id, s.folder_id, s.transfer_id, s.created_by,
			s.created_at, s.expires_at, s.revoked_at,
			f.id, f.owner_id, f.folder_id, f.transfer_id, f.storage_key, f.original_name,
			f.mime_type, f.size_bytes, f.upload_status, f.created_at,
			f.completed_at, f.expires_at
		FROM share_links s
		JOIN files f ON f.id = s.file_id
		WHERE s.short_code = $1`, code)

	var shared domain.SharedFile
	err := row.Scan(
		&shared.Share.ID, &shared.Share.ShortCode, &shared.Share.FileID,
		&shared.Share.FolderID, &shared.Share.TransferID, &shared.Share.CreatedBy, &shared.Share.CreatedAt,
		&shared.Share.ExpiresAt, &shared.Share.RevokedAt,
		&shared.File.ID, &shared.File.OwnerID, &shared.File.FolderID, &shared.File.TransferID,
		&shared.File.StorageKey, &shared.File.OriginalName, &shared.File.MIMEType,
		&shared.File.SizeBytes, &shared.File.Status, &shared.File.CreatedAt,
		&shared.File.CompletedAt, &shared.File.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SharedFile{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SharedFile{}, fmt.Errorf("resolve share: %w", err)
	}
	if shared.Share.RevokedAt != nil {
		return domain.SharedFile{}, domain.ErrRevoked
	}
	if shared.Share.ExpiresAt != nil && !shared.Share.ExpiresAt.After(now) {
		return domain.SharedFile{}, domain.ErrExpired
	}
	if shared.File.ExpiresAt != nil && !shared.File.ExpiresAt.After(now) {
		return domain.SharedFile{}, domain.ErrExpired
	}
	if shared.File.Status != domain.FileStatusReady {
		return domain.SharedFile{}, domain.ErrConflict
	}
	return shared, nil
}

func (p *Postgres) CreateAnonymousTransfer(ctx context.Context, transfer domain.AnonymousTransfer, files []domain.File, share domain.ShareLink) error {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transfer transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO anonymous_transfers (
			id, upload_status, archive_status, archive_storage_key, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		transfer.ID, transfer.Status, transfer.ArchiveStatus, transfer.ArchiveStorageKey, transfer.CreatedAt, transfer.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert anonymous transfer: %w", err)
	}
	for _, file := range files {
		_, err = tx.Exec(ctx, `
			INSERT INTO files (
				id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
				size_bytes, upload_status, created_at, expires_at
			) VALUES ($1, NULL, NULL, $2, $3, $4, $5, $6, $7, $8, $9)`,
			file.ID, transfer.ID, file.StorageKey, file.OriginalName, file.MIMEType,
			file.SizeBytes, file.Status, file.CreatedAt, file.ExpiresAt,
		)
		if err != nil {
			return fmt.Errorf("insert transfer file: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO share_links (
			id, short_code, file_id, folder_id, transfer_id, created_by, expires_at, created_at
		) VALUES ($1, $2, NULL, NULL, $3, NULL, $4, $5)`,
		share.ID, share.ShortCode, transfer.ID, share.ExpiresAt, share.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transfer share link: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transfer transaction: %w", err)
	}
	return nil
}

func (p *Postgres) GetTransferUploadFile(ctx context.Context, transferID, fileID string, now time.Time) (domain.File, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		       size_bytes, upload_status, created_at, completed_at, expires_at
		FROM files
		WHERE id = $1 AND transfer_id = $2
		  AND upload_status IN ('PENDING', 'READY')
		  AND expires_at > $3`, fileID, transferID, now)
	file, err := scanFile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("get transfer upload file: %w", err)
	}
	return file, nil
}

func (p *Postgres) CompleteTransferFile(ctx context.Context, transferID, fileID string, now time.Time) (domain.File, domain.AnonymousTransfer, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("begin completion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	transfer, err := scanTransfer(tx.QueryRow(ctx, `
		SELECT id, upload_status, archive_status, archive_storage_key, archive_size_bytes,
		       created_at, completed_at, expires_at
		FROM anonymous_transfers
		WHERE id = $1 AND expires_at > $2
		FOR UPDATE`, transferID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.AnonymousTransfer{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("lock transfer: %w", err)
	}

	file, err := scanFile(tx.QueryRow(ctx, `
		UPDATE files
		SET upload_status = 'READY', completed_at = COALESCE(completed_at, $3)
		WHERE id = $1 AND transfer_id = $2 AND upload_status IN ('PENDING', 'READY')
		RETURNING id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		          size_bytes, upload_status, created_at, completed_at, expires_at`, fileID, transferID, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.File{}, domain.AnonymousTransfer{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("complete transfer file: %w", err)
	}

	var pending int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM files WHERE transfer_id = $1 AND upload_status = 'PENDING'`, transferID).Scan(&pending); err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("count pending transfer files: %w", err)
	}
	if pending == 0 {
		transfer, err = scanTransfer(tx.QueryRow(ctx, `
			UPDATE anonymous_transfers
			SET upload_status = 'READY', archive_status = CASE WHEN archive_status = 'WAITING' THEN 'PENDING' ELSE archive_status END,
			    completed_at = COALESCE(completed_at, $2)
			WHERE id = $1
			RETURNING id, upload_status, archive_status, archive_storage_key, archive_size_bytes,
			          created_at, completed_at, expires_at`, transferID, now))
		if err != nil {
			return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("complete transfer: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("commit file completion: %w", err)
	}
	return file, transfer, nil
}

func (p *Postgres) ResolveTransferShare(ctx context.Context, code string, now time.Time) (domain.SharedTransfer, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT s.id, s.short_code, s.file_id, s.folder_id, s.transfer_id, s.created_by,
		       s.created_at, s.expires_at, s.revoked_at,
		       t.id, t.upload_status, t.archive_status, t.archive_storage_key, t.archive_size_bytes,
		       t.created_at, t.completed_at, t.expires_at
		FROM share_links s
		JOIN anonymous_transfers t ON t.id = s.transfer_id
		WHERE s.short_code = $1`, code)
	var shared domain.SharedTransfer
	err := row.Scan(
		&shared.Share.ID, &shared.Share.ShortCode, &shared.Share.FileID, &shared.Share.FolderID,
		&shared.Share.TransferID, &shared.Share.CreatedBy, &shared.Share.CreatedAt,
		&shared.Share.ExpiresAt, &shared.Share.RevokedAt,
		&shared.Transfer.ID, &shared.Transfer.Status, &shared.Transfer.ArchiveStatus,
		&shared.Transfer.ArchiveStorageKey, &shared.Transfer.ArchiveSizeBytes,
		&shared.Transfer.CreatedAt, &shared.Transfer.CompletedAt, &shared.Transfer.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SharedTransfer{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SharedTransfer{}, fmt.Errorf("resolve transfer share: %w", err)
	}
	if shared.Share.RevokedAt != nil {
		return domain.SharedTransfer{}, domain.ErrRevoked
	}
	if (shared.Share.ExpiresAt != nil && !shared.Share.ExpiresAt.After(now)) || !shared.Transfer.ExpiresAt.After(now) {
		return domain.SharedTransfer{}, domain.ErrExpired
	}

	rows, err := p.pool.Query(ctx, `
		SELECT id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		       size_bytes, upload_status, created_at, completed_at, expires_at
		FROM files WHERE transfer_id = $1 ORDER BY lower(original_name), id`, shared.Transfer.ID)
	if err != nil {
		return domain.SharedTransfer{}, fmt.Errorf("list shared transfer files: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return domain.SharedTransfer{}, fmt.Errorf("scan shared transfer file: %w", err)
		}
		shared.Files = append(shared.Files, file)
	}
	if err := rows.Err(); err != nil {
		return domain.SharedTransfer{}, fmt.Errorf("list shared transfer files: %w", err)
	}
	return shared, nil
}

func (p *Postgres) ClaimPendingArchive(ctx context.Context, now time.Time) (domain.AnonymousTransfer, []domain.File, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AnonymousTransfer{}, nil, fmt.Errorf("begin archive claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	transfer, err := scanTransfer(tx.QueryRow(ctx, `
		SELECT id, upload_status, archive_status, archive_storage_key, archive_size_bytes,
		       created_at, completed_at, expires_at
		FROM anonymous_transfers
		WHERE archive_status = 'PENDING' AND upload_status = 'READY' AND expires_at > $1
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED LIMIT 1`, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AnonymousTransfer{}, nil, domain.ErrNotFound
	}
	if err != nil {
		return domain.AnonymousTransfer{}, nil, fmt.Errorf("claim pending archive: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE anonymous_transfers SET archive_status = 'BUILDING' WHERE id = $1`, transfer.ID); err != nil {
		return domain.AnonymousTransfer{}, nil, fmt.Errorf("mark archive building: %w", err)
	}
	transfer.ArchiveStatus = domain.ArchiveStatusBuilding

	rows, err := tx.Query(ctx, `
		SELECT id, owner_id, folder_id, transfer_id, storage_key, original_name, mime_type,
		       size_bytes, upload_status, created_at, completed_at, expires_at
		FROM files WHERE transfer_id = $1 AND upload_status = 'READY' ORDER BY lower(original_name), id`, transfer.ID)
	if err != nil {
		return domain.AnonymousTransfer{}, nil, fmt.Errorf("list archive files: %w", err)
	}
	defer rows.Close()
	var files []domain.File
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return domain.AnonymousTransfer{}, nil, fmt.Errorf("scan archive file: %w", err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return domain.AnonymousTransfer{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AnonymousTransfer{}, nil, fmt.Errorf("commit archive claim: %w", err)
	}
	return transfer, files, nil
}

func (p *Postgres) CompleteArchive(ctx context.Context, transferID string, sizeBytes int64, now time.Time) error {
	command, err := p.pool.Exec(ctx, `
		UPDATE anonymous_transfers SET archive_status = 'READY', archive_size_bytes = $2
		WHERE id = $1 AND archive_status = 'BUILDING' AND expires_at > $3`, transferID, sizeBytes, now)
	if err != nil {
		return fmt.Errorf("complete archive: %w", err)
	}
	if command.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) FailArchive(ctx context.Context, transferID string) error {
	_, err := p.pool.Exec(ctx, `UPDATE anonymous_transfers SET archive_status = 'FAILED' WHERE id = $1`, transferID)
	if err != nil {
		return fmt.Errorf("fail archive: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (domain.File, error) {
	var file domain.File
	err := row.Scan(
		&file.ID, &file.OwnerID, &file.FolderID, &file.TransferID, &file.StorageKey,
		&file.OriginalName, &file.MIMEType, &file.SizeBytes, &file.Status,
		&file.CreatedAt, &file.CompletedAt, &file.ExpiresAt,
	)
	return file, err
}

func scanTransfer(row rowScanner) (domain.AnonymousTransfer, error) {
	var transfer domain.AnonymousTransfer
	err := row.Scan(
		&transfer.ID, &transfer.Status, &transfer.ArchiveStatus, &transfer.ArchiveStorageKey,
		&transfer.ArchiveSizeBytes, &transfer.CreatedAt, &transfer.CompletedAt, &transfer.ExpiresAt,
	)
	return transfer, err
}
