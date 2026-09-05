"use client";

import { ChangeEvent, DragEvent, useEffect, useState } from "react";
import { useAuth } from "@/components/auth-context";
import { CopyIcon, DownloadIcon, FileIcon, LinkIcon, UploadIcon } from "@/components/icons";
import {
  APIError,
  completePersistentUpload,
  createPersistentFileShare,
  createPersistentUpload,
  deletePersistentFile,
  getPersistentFileDownload,
  listPersistentFiles,
  revokePersistentFileShare,
  uploadResumable,
} from "@/lib/api";
import { formatBytes, formatExpiry, formatFileType, formatRelativeDate } from "@/lib/format";
import type { FileLibrarySummary, FileRecord, OwnedFileRecord, PersistentShareExpiration } from "@/lib/types";

const MAX_FILE_BYTES = Number(process.env.NEXT_PUBLIC_MAX_PERSISTENT_FILE_BYTES ?? 5 * 1024 ** 3);
const MAX_FILES_PER_SELECTION = 10;
const FILES_PER_PAGE = 10;

type FileSort = "newest" | "oldest" | "name" | "size";
type FileFilter = "all" | "shared";

function errorMessage(error: unknown, fallback: string) {
  return error instanceof APIError ? error.message : fallback;
}

export function PersistentFileLibrary() {
  const { getIDToken } = useAuth();
  const [files, setFiles] = useState<OwnedFileRecord[] | null>(null);
  const [summary, setSummary] = useState<FileLibrarySummary | null>(null);
  const [error, setError] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [activeFile, setActiveFile] = useState("");
  const [deletingID, setDeletingID] = useState("");
  const [confirmDeleteID, setConfirmDeleteID] = useState("");
  const [openShareID, setOpenShareID] = useState("");
  const [shareExpiration, setShareExpiration] = useState<PersistentShareExpiration>("7d");
  const [shareBusyID, setShareBusyID] = useState("");
  const [copiedShareID, setCopiedShareID] = useState("");
  const [page, setPage] = useState(1);
  const [searchQuery, setSearchQuery] = useState("");
  const [sort, setSort] = useState<FileSort>("newest");
  const [filter, setFilter] = useState<FileFilter>("all");
  const [dragging, setDragging] = useState(false);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const token = await getIDToken();
        const result = await listPersistentFiles(token);
        if (active) {
          setFiles(result.files);
          setSummary(result.summary ?? {
            fileCount: result.files.length,
            totalBytes: result.files.reduce((total, entry) => total + entry.file.sizeBytes, 0),
          });
        }
      } catch (loadError) {
        if (active) {
          setFiles([]);
          setSummary({ fileCount: 0, totalBytes: 0 });
          setError(errorMessage(loadError, "Your files could not be loaded. Please try again."));
        }
      }
    })();
    return () => { active = false; };
  }, [getIDToken]);

  function addCompletedFiles(completed: FileRecord[]) {
    if (completed.length === 0) return;
    const newest = completed.slice().reverse();
    setFiles((current) => [...newest.map((file) => ({ file })), ...(current ?? [])]);
    setSummary((current) => ({
      fileCount: (current?.fileCount ?? 0) + completed.length,
      totalBytes: (current?.totalBytes ?? 0) + completed.reduce((total, file) => total + file.sizeBytes, 0),
    }));
    setPage(1);
  }

  async function uploadSelectedFiles(selected: File[]) {
    if (selected.length === 0 || uploading) return;
    if (selected.length > MAX_FILES_PER_SELECTION) {
      setError(`Choose no more than ${MAX_FILES_PER_SELECTION} files at once.`);
      return;
    }
    if (selected.some((file) => file.size <= 0 || file.size > MAX_FILE_BYTES)) {
      setError(`Each file must contain data and be no larger than ${formatBytes(MAX_FILE_BYTES)}.`);
      return;
    }

    setUploading(true);
    setUploadProgress(0);
    setError("");
    const completed: FileRecord[] = [];
    let pendingID = "";
    try {
      const token = await getIDToken();
      for (let index = 0; index < selected.length; index += 1) {
        const file = selected[index];
        setActiveFile(file.name);
        const created = await createPersistentUpload(file, token);
        pendingID = created.file.id;
        const upload = uploadResumable(file, created.uploadTarget, (filePercent) => {
          setUploadProgress(Math.round(((index + filePercent / 100) / selected.length) * 100));
        });
        await upload.promise;
        completed.push(await completePersistentUpload(created.file.id, token));
        pendingID = "";
      }
      addCompletedFiles(completed);
      setUploadProgress(100);
    } catch (uploadError) {
      setError(errorMessage(uploadError, "Your upload could not be completed. Please try again."));
      if (completed.length > 0) {
        addCompletedFiles(completed);
      }
      try {
        const token = await getIDToken();
        if (pendingID) await deletePersistentFile(pendingID, token);
      } catch {
        // The incomplete metadata remains hidden from the ready-file listing.
      }
    } finally {
      setUploading(false);
      setActiveFile("");
    }
  }

  async function uploadFiles(event: ChangeEvent<HTMLInputElement>) {
    const selected = Array.from(event.target.files ?? []);
    event.target.value = "";
    await uploadSelectedFiles(selected);
  }

  async function dropFiles(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setDragging(false);
    await uploadSelectedFiles(Array.from(event.dataTransfer.files));
  }

  async function downloadFile(fileID: string) {
    setError("");
    try {
      const token = await getIDToken();
      const result = await getPersistentFileDownload(fileID, token);
      window.location.assign(result.downloadTarget.url);
    } catch (downloadError) {
      setError(errorMessage(downloadError, "The download could not be prepared. Please try again."));
    }
  }

  async function removeFile(fileID: string) {
    const removedFile = files?.find((entry) => entry.file.id === fileID)?.file;
    setDeletingID(fileID);
    setError("");
    try {
      const token = await getIDToken();
      await deletePersistentFile(fileID, token);
      setFiles((current) => current?.filter((entry) => entry.file.id !== fileID) ?? []);
      if (removedFile) {
        setSummary((current) => ({
          fileCount: Math.max(0, (current?.fileCount ?? 0) - 1),
          totalBytes: Math.max(0, (current?.totalBytes ?? 0) - removedFile.sizeBytes),
        }));
      }
      setConfirmDeleteID("");
      setOpenShareID((current) => current === fileID ? "" : current);
    } catch (deleteError) {
      setError(errorMessage(deleteError, "The file could not be deleted. Please try again."));
    } finally {
      setDeletingID("");
    }
  }

  function absoluteShareURL(path: string) {
    if (typeof window === "undefined") return path;
    return new URL(path, window.location.origin).toString();
  }

  async function createShare(fileID: string) {
    setShareBusyID(fileID);
    setError("");
    try {
      const token = await getIDToken();
      const result = await createPersistentFileShare(fileID, shareExpiration, token);
      setFiles((current) => current?.map((entry) => entry.file.id === fileID
        ? { ...entry, share: result.share, sharePath: result.sharePath }
        : entry) ?? []);
    } catch (shareError) {
      setError(errorMessage(shareError, "A share link could not be created. Please try again."));
    } finally {
      setShareBusyID("");
    }
  }

  async function copyShareLink(shareID: string, sharePath: string) {
    setError("");
    try {
      await navigator.clipboard.writeText(absoluteShareURL(sharePath));
      setCopiedShareID(shareID);
      window.setTimeout(() => setCopiedShareID((current) => current === shareID ? "" : current), 1800);
    } catch {
      setError("The link could not be copied. Select it and copy it manually.");
    }
  }

  async function revokeShare(fileID: string, shareID: string) {
    setShareBusyID(fileID);
    setError("");
    try {
      const token = await getIDToken();
      await revokePersistentFileShare(fileID, shareID, token);
      setFiles((current) => current?.map((entry) => entry.file.id === fileID
        ? { file: entry.file }
        : entry) ?? []);
      setCopiedShareID("");
    } catch (shareError) {
      setError(errorMessage(shareError, "The share link could not be revoked. Please try again."));
    } finally {
      setShareBusyID("");
    }
  }

  const normalizedQuery = searchQuery.trim().toLocaleLowerCase();
  const filteredFiles = (files ?? [])
    .filter((entry) => filter === "all" || Boolean(entry.share))
    .filter((entry) => entry.file.originalName.toLocaleLowerCase().includes(normalizedQuery))
    .sort((left, right) => {
      const leftDate = Date.parse(left.file.completedAt ?? left.file.createdAt);
      const rightDate = Date.parse(right.file.completedAt ?? right.file.createdAt);
      switch (sort) {
        case "oldest": return leftDate - rightDate;
        case "name": return left.file.originalName.localeCompare(right.file.originalName, undefined, { sensitivity: "base", numeric: true });
        case "size": return right.file.sizeBytes - left.file.sizeBytes;
        default: return rightDate - leftDate;
      }
    });
  const pageCount = Math.max(1, Math.ceil(filteredFiles.length / FILES_PER_PAGE));
  const currentPage = Math.min(page, pageCount);
  const pageStart = (currentPage - 1) * FILES_PER_PAGE;
  const visibleFiles = filteredFiles.slice(pageStart, pageStart + FILES_PER_PAGE);

  function resetLibraryView() {
    setPage(1);
    setOpenShareID("");
    setConfirmDeleteID("");
    setCopiedShareID("");
  }

  function goToPage(nextPage: number) {
    setPage(Math.max(1, Math.min(nextPage, pageCount)));
    setOpenShareID("");
    setConfirmDeleteID("");
    setCopiedShareID("");
  }

  return (
    <div
      className={`library-content ${dragging ? "is-dragging" : ""}`}
      onDragEnter={(event) => {
        event.preventDefault();
        if (!uploading && event.dataTransfer.types.includes("Files")) setDragging(true);
      }}
      onDragOver={(event) => event.preventDefault()}
      onDragLeave={(event) => {
        const nextTarget = event.relatedTarget;
        if (!(nextTarget instanceof Node) || !event.currentTarget.contains(nextTarget)) setDragging(false);
      }}
      onDrop={dropFiles}
    >
      {dragging && <div className="library-drop-overlay" aria-hidden="true"><UploadIcon /> Drop files to add them</div>}
      <div className="panel-heading">
        <div>
          <p className="eyebrow">Library</p>
          <h2 id="library-title">Your files</h2>
          <p className="library-summary" aria-live="polite">
            {summary === null
              ? "Loading storage usage…"
              : `${summary.fileCount} ${summary.fileCount === 1 ? "file" : "files"} · ${formatBytes(summary.totalBytes)} stored`}
          </p>
        </div>
        <label className={`primary-button library-upload-button ${uploading ? "is-disabled" : ""}`} htmlFor="owned-files-input">
          <UploadIcon /> {uploading ? "Uploading…" : "Upload files"}
        </label>
        <input
          id="owned-files-input"
          className="visually-hidden"
          type="file"
          multiple
          disabled={uploading}
          onChange={uploadFiles}
        />
      </div>

      {uploading && (
        <div className="library-progress" role="status">
          <span>Uploading {activeFile}</span>
          <strong>{uploadProgress}%</strong>
          <div className="progress-track"><span style={{ width: `${uploadProgress}%` }} /></div>
        </div>
      )}
      {error && <p className="error-message library-error" role="alert">{error}</p>}

      {files !== null && files.length > 0 && (
        <div className="library-toolbar">
          <label className="library-search">
            <span className="visually-hidden">Search your files</span>
            <input
              type="search"
              placeholder="Search files"
              value={searchQuery}
              onChange={(event) => {
                setSearchQuery(event.target.value);
                resetLibraryView();
              }}
            />
          </label>
          <div className="library-filter" aria-label="Filter files">
            <button
              type="button"
              aria-pressed={filter === "all"}
              onClick={() => { setFilter("all"); resetLibraryView(); }}
            >
              All
            </button>
            <button
              type="button"
              aria-pressed={filter === "shared"}
              onClick={() => { setFilter("shared"); resetLibraryView(); }}
            >
              Shared
            </button>
          </div>
          <label className="library-sort">
            <span>Sort</span>
            <select
              value={sort}
              onChange={(event) => {
                setSort(event.target.value as FileSort);
                resetLibraryView();
              }}
            >
              <option value="newest">Newest</option>
              <option value="oldest">Oldest</option>
              <option value="name">Name</option>
              <option value="size">Largest</option>
            </select>
          </label>
        </div>
      )}

      {files === null ? (
        <div className="library-loading" role="status">
          <span />
          <span />
          <span />
        </div>
      ) : files.length === 0 ? (
        <div className="library-empty">
          <span className="empty-icon"><FileIcon /></span>
          <div>
            <h3>Your library is empty.</h3>
            <p>Upload files you want to keep. They stay private and do not expire.</p>
          </div>
          <label className="secondary-button file-picker-label" htmlFor="owned-files-input">Choose files</label>
        </div>
      ) : filteredFiles.length === 0 ? (
        <div className="library-no-results">
          <FileIcon />
          <h3>No files found.</h3>
          <p>Try another filename or show all files.</p>
          <button
            type="button"
            onClick={() => {
              setSearchQuery("");
              setFilter("all");
              resetLibraryView();
            }}
          >
            Clear filters
          </button>
        </div>
      ) : (
        <div className="owned-file-list" aria-label="Your files">
          {visibleFiles.map((entry) => {
            const file = entry.file;
            const shareIsOpen = openShareID === file.id;
            return (
            <article className={`owned-file-row ${shareIsOpen ? "share-is-open" : ""}`} key={file.id}>
              <span className="owned-file-icon"><FileIcon /></span>
              <span className="owned-file-name">
                <strong title={file.originalName}>{file.originalName}</strong>
                <span className="owned-file-kind">
                  {formatFileType(file.mimeType, file.originalName)}
                  {entry.share && <em>Link active</em>}
                </span>
              </span>
              <span className="owned-file-meta">{formatBytes(file.sizeBytes)}</span>
              <span className="owned-file-meta" title={formatExpiry(file.completedAt ?? file.createdAt)}>
                {formatRelativeDate(file.completedAt ?? file.createdAt)}
              </span>
              <span className="owned-file-actions">
                {confirmDeleteID === file.id ? (
                  <>
                    <button type="button" className="row-action" onClick={() => setConfirmDeleteID("")}>Cancel</button>
                    <button type="button" className="row-action danger" disabled={deletingID === file.id} onClick={() => removeFile(file.id)}>
                      {deletingID === file.id ? "Deleting…" : "Delete"}
                    </button>
                  </>
                ) : (
                  <>
                    <button type="button" className="row-action download" onClick={() => downloadFile(file.id)}>
                      <DownloadIcon /> Download
                    </button>
                    <button
                      type="button"
                      className={`row-action share ${entry.share ? "is-active" : ""}`}
                      aria-expanded={shareIsOpen}
                      onClick={() => {
                        setOpenShareID((current) => current === file.id ? "" : file.id);
                        setShareExpiration("7d");
                        setCopiedShareID("");
                      }}
                    >
                      <LinkIcon /> Share
                    </button>
                    <button type="button" className="row-action" onClick={() => setConfirmDeleteID(file.id)}>Delete</button>
                  </>
                )}
              </span>
              {shareIsOpen && (
                <div className="file-share-panel">
                  {entry.share && entry.sharePath ? (
                    <>
                      <div className="file-share-copy">
                        <label htmlFor={`share-link-${file.id}`}>Anyone with this link can download</label>
                        <div>
                          <input id={`share-link-${file.id}`} readOnly value={absoluteShareURL(entry.sharePath)} />
                          <button type="button" onClick={() => copyShareLink(entry.share!.id, entry.sharePath!)}>
                            <CopyIcon /> {copiedShareID === entry.share.id ? "Copied" : "Copy"}
                          </button>
                        </div>
                        <small>{entry.share.expiresAt ? `Expires ${formatExpiry(entry.share.expiresAt)}` : "This link does not expire."}</small>
                      </div>
                      <button
                        type="button"
                        className="revoke-share-button"
                        disabled={shareBusyID === file.id}
                        onClick={() => revokeShare(file.id, entry.share!.id)}
                      >
                        {shareBusyID === file.id ? "Revoking…" : "Revoke link"}
                      </button>
                    </>
                  ) : (
                    <>
                      <div>
                        <strong>Create a download link</strong>
                        <p>The file stays in your library. You can revoke the link at any time.</p>
                      </div>
                      <label className="share-expiration-control">
                        <span>Expires</span>
                        <select value={shareExpiration} onChange={(event) => setShareExpiration(event.target.value as PersistentShareExpiration)}>
                          <option value="24h">In 24 hours</option>
                          <option value="7d">In 7 days</option>
                          <option value="30d">In 30 days</option>
                          <option value="never">Never</option>
                        </select>
                      </label>
                      <button
                        type="button"
                        className="create-share-button"
                        disabled={shareBusyID === file.id}
                        onClick={() => createShare(file.id)}
                      >
                        <LinkIcon /> {shareBusyID === file.id ? "Creating…" : "Create link"}
                      </button>
                    </>
                  )}
                </div>
              )}
            </article>
          )})}
          {filteredFiles.length > FILES_PER_PAGE && (
            <nav className="library-pagination" aria-label="File library pages">
              <span>
                {pageStart + 1}–{Math.min(pageStart + FILES_PER_PAGE, filteredFiles.length)} of {filteredFiles.length}
              </span>
              <div>
                <button type="button" disabled={currentPage === 1} onClick={() => goToPage(currentPage - 1)}>
                  Previous
                </button>
                <span>Page {currentPage} of {pageCount}</span>
                <button type="button" disabled={currentPage === pageCount} onClick={() => goToPage(currentPage + 1)}>
                  Next
                </button>
              </div>
            </nav>
          )}
        </div>
      )}
    </div>
  );
}
