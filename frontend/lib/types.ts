export type FileRecord = {
  id: string;
  transferId?: string;
  originalName: string;
  mimeType: string;
  sizeBytes: number;
  status: "PENDING" | "READY";
  createdAt: string;
  completedAt?: string;
  expiresAt?: string;
};

export type ShareRecord = {
  id: string;
  shortCode: string;
  fileId?: string;
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

export type ResolveShareResult = {
  file: FileRecord;
  share: ShareRecord;
  downloadTarget: {
    url: string;
    expiresAt: string;
  };
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
