package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/jackc/pgx/v5"
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
