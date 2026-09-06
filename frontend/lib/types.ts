export type FileRecord = {
	id: string;
	ownerId?: string;
	folderId?: string;
  transferId?: string;
  originalName: string;
  mimeType: string;
  sizeBytes: number;
  status: "PENDING" | "READY";
  createdAt: string;
  completedAt?: string;
  expiresAt?: string;
};

export type UserRecord = {
  id: string;
  email: string;
  displayName: string;
  identityDisplayName?: string;
  customDisplayName?: string | null;
  createdAt: string;
};

export type ShareRecord = {
  id: string;
  shortCode: string;
	fileId?: string;
	folderId?: string;
  transferId?: string;
  createdAt: string;
  expiresAt?: string;
  revokedAt?: string;
};

export type UploadTarget = {
  url: string;
  method: string;
  headers: Record<string, string>;
  expiresAt: string;
};

export type CreateUploadResult = {
  file: FileRecord;
  share: ShareRecord;
  sharePath: string;
  uploadTarget: UploadTarget;
};

export type CreatePersistentUploadResult = {
  file: FileRecord;
  uploadTarget: UploadTarget;
};

export type OwnedFileRecord = {
  file: FileRecord;
	uploaderName?: string;
  share?: ShareRecord;
  sharePath?: string;
};

export type FileLibrarySummary = {
	fileCount: number;
	totalBytes: number;
	accountTotalBytes?: number;
	quotaBytes?: number;
};

export type FileLibraryResult = {
  files: OwnedFileRecord[];
  summary: FileLibrarySummary;
};

export type FolderRole = "OWNER" | "CONTRIBUTOR" | "VIEWER";

export type FolderRecord = {
	id: string;
	ownerId: string;
	parentFolderId?: string;
	name: string;
	createdAt: string;
};

export type FolderAccess = {
	folder: FolderRecord;
	role: FolderRole;
	owner: UserRecord;
};

export type FolderMember = {
	user: UserRecord;
	role: "VIEWER" | "CONTRIBUTOR";
	createdAt: string;
	expiresAt?: string;
	inherited?: boolean;
	sourceFolderId?: string;
	sourceFolderName?: string;
};

export type FolderInvite = {
	id: string;
	folderId: string;
	shortCode: string;
	role: "VIEWER" | "CONTRIBUTOR";
	createdAt: string;
	expiresAt?: string;
};

export type FolderInvitePreview = {
	folderName: string;
	ownerName: string;
	role: "VIEWER" | "CONTRIBUTOR";
	expiresAt?: string;
};

export type FolderContents = {
	current?: FolderAccess;
	breadcrumbs: FolderRecord[];
	folders: FolderAccess[];
	files: OwnedFileRecord[];
	summary: FileLibrarySummary;
	totalCount: number;
	nextCursor?: string;
};

export type PersistentShareExpiration = "24h" | "7d" | "30d" | "never";

export type CreatePersistentShareResult = {
  share: ShareRecord;
  sharePath: string;
};

export type FileDownloadResult = {
  file: FileRecord;
  downloadTarget: {
    url: string;
    expiresAt: string;
  };
	preview?: FilePreviewRecord;
};

export type FilePreviewKind = "image" | "pdf" | "video" | "audio" | "text";

export type FilePreviewRecord = {
	kind: FilePreviewKind;
	url: string;
	expiresAt: string;
};

export type ResolveShareResult = {
  file: FileRecord;
  share: ShareRecord;
  downloadTarget: {
    url: string;
    expiresAt: string;
  };
	preview?: FilePreviewRecord;
};

export type AnonymousTransfer = {
  id: string;
  status: "PENDING" | "READY";
  archiveStatus: "WAITING" | "PENDING" | "BUILDING" | "READY" | "FAILED";
  archiveSizeBytes?: number;
  createdAt: string;
  completedAt?: string;
  expiresAt: string;
};

export type CreateTransferResult = {
  transfer: AnonymousTransfer;
  share: ShareRecord;
  sharePath: string;
  uploads: Array<{ file: FileRecord; uploadTarget: UploadTarget }>;
};

export type ResolveTransferResult = {
  transfer: AnonymousTransfer;
  share: ShareRecord;
  files: Array<{
    file: FileRecord;
    downloadTarget: { url: string; expiresAt: string };
	preview?: FilePreviewRecord;
  }>;
  archive: {
    status: AnonymousTransfer["archiveStatus"];
    sizeBytes?: number;
    downloadTarget?: { url: string; expiresAt: string };
  };
};

export type ShareResult = ResolveShareResult | ResolveTransferResult;

export function isTransferResult(result: ShareResult): result is ResolveTransferResult {
  return "transfer" in result;
}
