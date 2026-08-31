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

func (p *Postgres) CompleteUpload(ctx context.Context, fileID string, now time.Time) (domain.File, error) {
	row := p.pool.QueryRow(ctx, `
		UPDATE files
		SET upload_status = 'READY', completed_at = COALESCE(completed_at, $2)
		WHERE id = $1
		  AND upload_status IN ('PENDING', 'READY')
		  AND (expires_at IS NULL OR expires_at > $2)
		RETURNING id, owner_id, folder_id, storage_key, original_name, mime_type,
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
			s.id, s.short_code, s.file_id, s.folder_id, s.created_by,
			s.created_at, s.expires_at, s.revoked_at,
			f.id, f.owner_id, f.folder_id, f.storage_key, f.original_name,
			f.mime_type, f.size_bytes, f.upload_status, f.created_at,
			f.completed_at, f.expires_at
		FROM share_links s
		JOIN files f ON f.id = s.file_id
		WHERE s.short_code = $1`, code)

	var shared domain.SharedFile
	err := row.Scan(
		&shared.Share.ID, &shared.Share.ShortCode, &shared.Share.FileID,
		&shared.Share.FolderID, &shared.Share.CreatedBy, &shared.Share.CreatedAt,
		&shared.Share.ExpiresAt, &shared.Share.RevokedAt,
		&shared.File.ID, &shared.File.OwnerID, &shared.File.FolderID,
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (domain.File, error) {
	var file domain.File
	err := row.Scan(
		&file.ID, &file.OwnerID, &file.FolderID, &file.StorageKey,
		&file.OriginalName, &file.MIMEType, &file.SizeBytes, &file.Status,
		&file.CreatedAt, &file.CompletedAt, &file.ExpiresAt,
	)
	return file, err
}
