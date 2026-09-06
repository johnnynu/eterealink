package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrExpired          = errors.New("resource expired")
	ErrRevoked          = errors.New("resource revoked")
	ErrConflict         = errors.New("resource conflict")
	ErrQuotaExceeded    = errors.New("storage quota exceeded")
	ErrDisplayNameTaken = errors.New("display name is already in use")
)

type User struct {
	ID                  string    `json:"id"`
	FirebaseUID         string    `json:"-"`
	Email               string    `json:"email"`
	DisplayName         string    `json:"displayName"`
	IdentityDisplayName string    `json:"identityDisplayName,omitempty"`
	CustomDisplayName   *string   `json:"customDisplayName"`
	CreatedAt           time.Time `json:"createdAt"`
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

type OwnedFile struct {
	File         File       `json:"file"`
	UploaderName string     `json:"uploaderName,omitempty"`
	Share        *ShareLink `json:"share,omitempty"`
	SharePath    string     `json:"sharePath,omitempty"`
}

type FileLibrarySummary struct {
	FileCount  int64 `json:"fileCount"`
	TotalBytes int64 `json:"totalBytes"`
	QuotaBytes int64 `json:"quotaBytes,omitempty"`
}

type FolderRole string

const (
	FolderRoleOwner       FolderRole = "OWNER"
	FolderRoleContributor FolderRole = "CONTRIBUTOR"
	FolderRoleViewer      FolderRole = "VIEWER"
)

type Folder struct {
	ID             string    `json:"id"`
	OwnerID        string    `json:"ownerId"`
	ParentFolderID *string   `json:"parentFolderId,omitempty"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"createdAt"`
}

type FolderAccess struct {
	Folder Folder     `json:"folder"`
	Role   FolderRole `json:"role"`
	Owner  User       `json:"owner"`
}

type FolderMember struct {
	User             User       `json:"user"`
	Role             FolderRole `json:"role"`
	CreatedAt        time.Time  `json:"createdAt"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	Inherited        bool       `json:"inherited"`
	SourceFolderID   string     `json:"sourceFolderId"`
	SourceFolderName string     `json:"sourceFolderName"`
}

type FolderInvite struct {
	ID        string     `json:"id"`
	FolderID  string     `json:"folderId"`
	CreatedBy string     `json:"createdBy"`
	ShortCode string     `json:"shortCode"`
	Role      FolderRole `json:"role"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

type FolderInvitePreview struct {
	FolderName string     `json:"folderName"`
	OwnerName  string     `json:"ownerName"`
	Role       FolderRole `json:"role"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
}

type FolderContents struct {
	Current     *FolderAccess      `json:"current,omitempty"`
	Breadcrumbs []Folder           `json:"breadcrumbs"`
	Folders     []FolderAccess     `json:"folders"`
	Files       []OwnedFile        `json:"files"`
	Summary     FileLibrarySummary `json:"summary"`
	TotalCount  int64              `json:"totalCount"`
	NextCursor  string             `json:"nextCursor,omitempty"`
}

type FileLibraryQuery struct {
	Search     string
	SharedOnly bool
	Sort       string
	Limit      int
	CursorID   string
	CursorTime time.Time
	CursorName string
	CursorSize int64
}
