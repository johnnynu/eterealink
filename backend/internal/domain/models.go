package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrExpired  = errors.New("resource expired")
	ErrRevoked  = errors.New("resource revoked")
	ErrConflict = errors.New("resource conflict")
)

type User struct {
	ID          string    `json:"id"`
	FirebaseUID string    `json:"-"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

type FileStatus string

const (
	FileStatusPending FileStatus = "PENDING"
	FileStatusReady   FileStatus = "READY"
)

type File struct {
	ID           string     `json:"id"`
	OwnerID      *string    `json:"ownerId,omitempty"`
	FolderID     *string    `json:"folderId,omitempty"`
	TransferID   *string    `json:"transferId,omitempty"`
	StorageKey   string     `json:"-"`
	OriginalName string     `json:"originalName"`
	MIMEType     string     `json:"mimeType"`
	SizeBytes    int64      `json:"sizeBytes"`
	Status       FileStatus `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

type ShareLink struct {
	ID         string     `json:"id"`
	ShortCode  string     `json:"shortCode"`
	FileID     *string    `json:"fileId,omitempty"`
	FolderID   *string    `json:"folderId,omitempty"`
	TransferID *string    `json:"transferId,omitempty"`
	CreatedBy  *string    `json:"createdBy,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

type TransferStatus string

const (
	TransferStatusPending TransferStatus = "PENDING"
	TransferStatusReady   TransferStatus = "READY"
)

type ArchiveStatus string

const (
	ArchiveStatusWaiting  ArchiveStatus = "WAITING"
	ArchiveStatusPending  ArchiveStatus = "PENDING"
	ArchiveStatusBuilding ArchiveStatus = "BUILDING"
	ArchiveStatusReady    ArchiveStatus = "READY"
	ArchiveStatusFailed   ArchiveStatus = "FAILED"
)

type AnonymousTransfer struct {
	ID                string         `json:"id"`
	Status            TransferStatus `json:"status"`
	ArchiveStatus     ArchiveStatus  `json:"archiveStatus"`
	ArchiveStorageKey string         `json:"-"`
	ArchiveSizeBytes  *int64         `json:"archiveSizeBytes,omitempty"`
	CreatedAt         time.Time      `json:"createdAt"`
	CompletedAt       *time.Time     `json:"completedAt,omitempty"`
	ExpiresAt         time.Time      `json:"expiresAt"`
}

type SharedTransfer struct {
	Share    ShareLink         `json:"share"`
	Transfer AnonymousTransfer `json:"transfer"`
	Files    []File            `json:"files"`
}

type SharedFile struct {
	Share ShareLink `json:"share"`
	File  File      `json:"file"`
}
