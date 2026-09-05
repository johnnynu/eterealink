import type {
  CreatePersistentUploadResult,
  CreatePersistentShareResult,
  CreateTransferResult,
  CreateUploadResult,
  FileDownloadResult,
  FileLibraryResult,
  FileRecord,
	FolderContents,
	FolderMember,
	FolderInvite,
	FolderInvitePreview,
	FolderAccess,
	FolderRecord,
  PersistentShareExpiration,
  ShareResult,
  UploadTarget,
  UserRecord,
} from "@/lib/types";

type APIErrorBody = {
  error?: {
    code?: string;
    message?: string;
  };
};

export class APIError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code = "request_failed",
  ) {
    super(message);
    this.name = "APIError";
  }
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (response.ok) return (await response.json()) as T;

  let body: APIErrorBody = {};
  try {
    body = (await response.json()) as APIErrorBody;
  } catch {
    // The status still gives callers a useful fallback when a proxy fails.
  }

  throw new APIError(
    body.error?.message ?? "Eterealink could not complete that request.",
    response.status,
    body.error?.code,
  );
}

export async function createAnonymousUpload(file: File): Promise<CreateUploadResult> {
  const response = await fetch("/api/v1/uploads", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      originalName: file.name,
      mimeType: file.type || "application/octet-stream",
      sizeBytes: file.size,
    }),
  });
  return parseResponse<CreateUploadResult>(response);
}

export async function getCurrentUser(idToken: string): Promise<UserRecord> {
  const response = await fetch("/api/v1/me", {
    headers: { Authorization: `Bearer ${idToken}` },
    cache: "no-store",
  });
  const result = await parseResponse<{ user: UserRecord }>(response);
  return result.user;
}

function bearerHeaders(idToken: string, contentType = false) {
  return {
    Authorization: `Bearer ${idToken}`,
    ...(contentType ? { "Content-Type": "application/json" } : {}),
  };
}

export async function createPersistentUpload(file: File, idToken: string, folderID?: string): Promise<CreatePersistentUploadResult> {
  const response = await fetch("/api/v1/files", {
    method: "POST",
    headers: bearerHeaders(idToken, true),
    body: JSON.stringify({
      originalName: file.name,
      mimeType: file.type || "application/octet-stream",
      sizeBytes: file.size,
			folderId: folderID,
    }),
  });
  return parseResponse<CreatePersistentUploadResult>(response);
}

export type FolderListQuery = {
	search?: string;
	sort?: "newest" | "oldest" | "name" | "size";
	filter?: "all" | "shared";
	limit?: number;
	cursor?: string;
};

export async function listFolderContents(idToken: string, folderID?: string, scope: "owned" | "shared" = "owned", query: FolderListQuery = {}): Promise<FolderContents> {
	const parameters = new URLSearchParams();
	if (!folderID) parameters.set("scope", scope);
	if (query.search) parameters.set("q", query.search);
	if (query.sort) parameters.set("sort", query.sort);
	if (query.filter && query.filter !== "all") parameters.set("filter", query.filter);
	if (query.limit) parameters.set("limit", String(query.limit));
	if (query.cursor) parameters.set("cursor", query.cursor);
	const base = folderID ? `/api/v1/folders/${encodeURIComponent(folderID)}` : "/api/v1/folders";
	const path = `${base}?${parameters.toString()}`;
	const response = await fetch(path, { headers: bearerHeaders(idToken), cache: "no-store" });
	return parseResponse<FolderContents>(response);
}

export async function createFolder(name: string, parentFolderID: string | undefined, idToken: string): Promise<FolderRecord> {
	const response = await fetch("/api/v1/folders", {
		method: "POST", headers: bearerHeaders(idToken, true), body: JSON.stringify({ name, parentFolderId: parentFolderID }),
	});
	const result = await parseResponse<{ folder: FolderRecord }>(response);
	return result.folder;
}

export async function updateFolder(folderID: string, name: string, parentFolderID: string | undefined, idToken: string): Promise<FolderRecord> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}`, {
		method: "PATCH", headers: bearerHeaders(idToken, true), body: JSON.stringify({ name, parentFolderId: parentFolderID }),
	});
	const result = await parseResponse<{ folder: FolderRecord }>(response);
	return result.folder;
}

export async function deleteFolder(folderID: string, idToken: string): Promise<void> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}`, { method: "DELETE", headers: bearerHeaders(idToken) });
	if (!response.ok) await parseResponse(response);
}

export async function movePersistentFiles(fileIDs: string[], folderID: string | undefined, idToken: string): Promise<void> {
	const response = await fetch("/api/v1/files/move", {
		method: "PATCH", headers: bearerHeaders(idToken, true), body: JSON.stringify({ fileIds: fileIDs, folderId: folderID }),
	});
	if (!response.ok) await parseResponse(response);
}

export async function listFolderMembers(folderID: string, idToken: string): Promise<FolderMember[]> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/members`, { headers: bearerHeaders(idToken), cache: "no-store" });
	const result = await parseResponse<{ members: FolderMember[] }>(response);
	return result.members;
}

export async function addFolderMember(folderID: string, email: string, role: "VIEWER" | "CONTRIBUTOR", idToken: string): Promise<FolderMember> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/members`, {
		method: "POST", headers: bearerHeaders(idToken, true), body: JSON.stringify({ email, role }),
	});
	const result = await parseResponse<{ member: FolderMember }>(response);
	return result.member;
}

export async function createFolderInvite(folderID: string, role: "VIEWER" | "CONTRIBUTOR", expiresIn: PersistentShareExpiration, idToken: string): Promise<{ invite: FolderInvite; invitePath: string }> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/invites`, {
		method: "POST", headers: bearerHeaders(idToken, true), body: JSON.stringify({ role, expiresIn }),
	});
	return parseResponse(response);
}

export async function listFolderInvites(folderID: string, idToken: string): Promise<FolderInvite[]> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/invites`, { headers: bearerHeaders(idToken), cache: "no-store" });
	const result = await parseResponse<{ invites: FolderInvite[] }>(response);
	return result.invites;
}

export async function revokeFolderInvite(folderID: string, inviteID: string, idToken: string): Promise<void> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/invites/${encodeURIComponent(inviteID)}`, { method: "DELETE", headers: bearerHeaders(idToken) });
	if (!response.ok) await parseResponse(response);
}

export async function acceptFolderInvite(code: string, idToken: string): Promise<FolderAccess> {
	const response = await fetch(`/api/v1/folder-invites/${encodeURIComponent(code)}/accept`, { method: "POST", headers: bearerHeaders(idToken) });
	return parseResponse(response);
}

export async function getFolderInvitePreview(code: string): Promise<FolderInvitePreview> {
	const response = await fetch(`/api/v1/folder-invites/${encodeURIComponent(code)}`, { cache: "no-store" });
	const result = await parseResponse<{ invite: FolderInvitePreview }>(response);
	return result.invite;
}

export async function removeContributedFile(folderID: string, fileID: string, idToken: string): Promise<void> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/files/${encodeURIComponent(fileID)}`, { method: "DELETE", headers: bearerHeaders(idToken) });
	if (!response.ok) await parseResponse(response);
}

export async function removeFolderMember(folderID: string, userID: string, idToken: string): Promise<void> {
	const response = await fetch(`/api/v1/folders/${encodeURIComponent(folderID)}/members/${encodeURIComponent(userID)}`, {
		method: "DELETE", headers: bearerHeaders(idToken),
	});
	if (!response.ok) await parseResponse(response);
}

export async function completePersistentUpload(fileID: string, idToken: string): Promise<FileRecord> {
  const response = await fetch(`/api/v1/files/${encodeURIComponent(fileID)}/complete`, {
    method: "POST",
    headers: bearerHeaders(idToken),
  });
  const result = await parseResponse<{ file: FileRecord }>(response);
  return result.file;
}

export async function listPersistentFiles(idToken: string): Promise<FileLibraryResult> {
  const response = await fetch("/api/v1/files", {
    headers: bearerHeaders(idToken),
    cache: "no-store",
  });
  return parseResponse<FileLibraryResult>(response);
}

export async function createPersistentFileShare(
  fileID: string,
  expiresIn: PersistentShareExpiration,
  idToken: string,
): Promise<CreatePersistentShareResult> {
  const response = await fetch(`/api/v1/files/${encodeURIComponent(fileID)}/shares`, {
    method: "POST",
    headers: bearerHeaders(idToken, true),
    body: JSON.stringify({ expiresIn }),
  });
  return parseResponse<CreatePersistentShareResult>(response);
}

export async function revokePersistentFileShare(fileID: string, shareID: string, idToken: string): Promise<void> {
  const response = await fetch(
    `/api/v1/files/${encodeURIComponent(fileID)}/shares/${encodeURIComponent(shareID)}`,
    { method: "DELETE", headers: bearerHeaders(idToken) },
  );
  if (response.ok) return;
  await parseResponse(response);
}

export async function getPersistentFileDownload(fileID: string, idToken: string): Promise<FileDownloadResult> {
  const response = await fetch(`/api/v1/files/${encodeURIComponent(fileID)}/download`, {
    headers: bearerHeaders(idToken),
    cache: "no-store",
  });
  return parseResponse<FileDownloadResult>(response);
}

export async function deletePersistentFile(fileID: string, idToken: string): Promise<void> {
  const response = await fetch(`/api/v1/files/${encodeURIComponent(fileID)}`, {
    method: "DELETE",
    headers: bearerHeaders(idToken),
  });
  if (response.ok) return;
  await parseResponse(response);
}

export async function createAnonymousTransfer(files: File[]): Promise<CreateTransferResult> {
  const response = await fetch("/api/v1/transfers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      files: files.map((file) => ({
        originalName: file.name,
        mimeType: file.type || "application/octet-stream",
        sizeBytes: file.size,
      })),
    }),
  });
  return parseResponse<CreateTransferResult>(response);
}

export function uploadToStorage(
  file: File,
  target: UploadTarget,
  onProgress: (percent: number) => void,
): { promise: Promise<void>; abort: () => void } {
  const request = new XMLHttpRequest();
  const promise = new Promise<void>((resolve, reject) => {
    request.open(target.method, target.url);
    Object.entries(target.headers ?? {}).forEach(([name, value]) => request.setRequestHeader(name, value));
    request.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100));
    });
    request.addEventListener("load", () => {
      if (request.status >= 200 && request.status < 300) {
        onProgress(100);
        resolve();
      } else {
        reject(new APIError("The storage service rejected the upload. Please try again.", request.status));
      }
    });
    request.addEventListener("error", () => reject(new APIError("The upload was interrupted by a network error.", 0)));
    request.addEventListener("abort", () => reject(new APIError("Upload canceled.", 0, "canceled")));
    request.send(file);
  });

  return { promise, abort: () => request.abort() };
}

export async function completeAnonymousUpload(fileID: string): Promise<void> {
  const response = await fetch(`/api/v1/uploads/${encodeURIComponent(fileID)}/complete`, {
    method: "POST",
  });
  await parseResponse(response);
}

function uploadedOffset(response: Response): number | null {
  const range = response.headers.get("Range");
  if (!range) return null;
  const match = /bytes=0-(\d+)/i.exec(range);
  return match ? Number(match[1]) + 1 : null;
}

async function resumableOffset(sessionURL: string, size: number, signal: AbortSignal): Promise<number> {
  const response = await fetch(sessionURL, {
    method: "PUT",
    headers: { "Content-Range": `bytes */${size}` },
    body: null,
    signal,
  });
  if (response.status === 308) return uploadedOffset(response) ?? 0;
  if (response.ok) return size;
  throw new APIError("Cloud Storage could not resume the upload.", response.status);
}

export function uploadResumable(
  file: File,
  target: UploadTarget,
  onProgress: (percent: number) => void,
): { promise: Promise<void>; abort: () => void } {
  const controller = new AbortController();
  const promise = (async () => {
    const initiation = await fetch(target.url, {
      method: target.method,
      headers: target.headers,
      body: null,
      signal: controller.signal,
    });
    if (!initiation.ok) throw new APIError("Cloud Storage could not start the resumable upload.", initiation.status);
    const sessionURL = initiation.headers.get("Location");
    if (!sessionURL) throw new APIError("Cloud Storage did not return an upload session.", initiation.status);

    const chunkSize = 8 * 1024 * 1024;
    let offset = 0;
    while (offset < file.size) {
      const end = Math.min(offset + chunkSize, file.size);
      let completedChunk = false;
      for (let attempt = 0; attempt < 3; attempt += 1) {
        try {
          const response = await fetch(sessionURL, {
            method: "PUT",
            headers: {
              "Content-Type": file.type || "application/octet-stream",
              "Content-Range": `bytes ${offset}-${end - 1}/${file.size}`,
            },
            body: file.slice(offset, end),
            signal: controller.signal,
          });
          if (response.status === 308) {
            offset = uploadedOffset(response) ?? end;
            onProgress(Math.round((offset / file.size) * 100));
            completedChunk = true;
            break;
          }
          if (response.ok && end === file.size) {
            onProgress(100);
            return;
          }
          if (response.status !== 429 && response.status < 500) {
            throw new APIError("Cloud Storage rejected an upload chunk.", response.status);
          }
        } catch (error) {
          if (controller.signal.aborted) throw error;
          if (error instanceof APIError && error.status > 0 && error.status !== 429 && error.status < 500) throw error;
        }

        try {
          offset = await resumableOffset(sessionURL, file.size, controller.signal);
        } catch (error) {
          if (controller.signal.aborted || attempt === 2) throw error;
          continue;
        }
        onProgress(Math.round((offset / file.size) * 100));
        if (offset >= file.size) return;
        if (offset >= end) {
          completedChunk = true;
          break;
        }
      }
      if (!completedChunk) throw new APIError("The upload was interrupted after three attempts.", 0);
    }
  })();
  return { promise, abort: () => controller.abort() };
}

export async function completeTransferFile(transferID: string, fileID: string): Promise<void> {
  const response = await fetch(
    `/api/v1/transfers/${encodeURIComponent(transferID)}/files/${encodeURIComponent(fileID)}/complete`,
    { method: "POST" },
  );
  await parseResponse(response);
}

export async function resolveShare(code: string): Promise<ShareResult> {
  const response = await fetch(`/api/v1/shares/${encodeURIComponent(code)}`, {
    cache: "no-store",
  });
  return parseResponse<ShareResult>(response);
}
