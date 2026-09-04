"use client";

import { ChangeEvent, DragEvent, useRef, useState } from "react";
import { APIError, completeTransferFile, createAnonymousTransfer, uploadResumable } from "@/lib/api";
import { formatBytes, formatExpiry } from "@/lib/format";
import { CheckIcon, CopyIcon, FileIcon, UploadIcon } from "@/components/icons";

const MAX_FILE_BYTES = Number(process.env.NEXT_PUBLIC_MAX_UPLOAD_BYTES ?? 1024 ** 3);
const MAX_TRANSFER_BYTES = Number(process.env.NEXT_PUBLIC_MAX_TRANSFER_BYTES ?? 1024 ** 3);
const MAX_FILES = Number(process.env.NEXT_PUBLIC_MAX_FILES ?? 10);
const UPLOAD_CONCURRENCY = 3;

type UploadState = "idle" | "ready" | "creating" | "uploading" | "finalizing" | "success" | "error";

type CompletedTransfer = {
  shareURL: string;
  expiresAt?: string;
};

function totalBytes(files: File[]) {
  return files.reduce((total, file) => total + file.size, 0);
}

function validateFiles(files: File[]): string {
  if (files.length === 0) return "Choose at least one file.";
  if (files.length > MAX_FILES) return `Choose no more than ${MAX_FILES} files per transfer.`;
  if (files.some((file) => file.size <= 0)) return "Every file must contain at least one byte.";
  if (files.some((file) => file.size > MAX_FILE_BYTES)) {
    return `Each file must be no larger than ${formatBytes(MAX_FILE_BYTES)}.`;
  }
  if (totalBytes(files) > MAX_TRANSFER_BYTES) {
    return `Those files exceed the ${formatBytes(MAX_TRANSFER_BYTES)} combined transfer limit.`;
  }
  const normalizedNames = files.map((file) => file.name.toLocaleLowerCase());
  if (new Set(normalizedNames).size !== normalizedNames.length) {
    return "Each file needs a unique name so the ZIP can be created safely.";
  }
  return "";
}

export function UploadWorkspace({ variant = "anonymous" }: { variant?: "anonymous" | "workspace" }) {
  const abortersRef = useRef<Array<() => void>>([]);
  const [files, setFiles] = useState<File[]>([]);
  const [state, setState] = useState<UploadState>("idle");
  const [progress, setProgress] = useState(0);
  const [activeFile, setActiveFile] = useState("");
  const [dragging, setDragging] = useState(false);
  const [message, setMessage] = useState("");
  const [completed, setCompleted] = useState<CompletedTransfer | null>(null);
  const [copied, setCopied] = useState(false);
  const [canCancel, setCanCancel] = useState(false);

  const busy = state === "creating" || state === "uploading" || state === "finalizing";
  const selectedBytes = totalBytes(files);

  function selectFiles(nextFiles: File[]) {
    if (nextFiles.length === 0) return;
    setCompleted(null);
    setCopied(false);
    setProgress(0);
    const validationMessage = validateFiles(nextFiles);
    setFiles(nextFiles);
    setState(validationMessage ? "error" : "ready");
    setMessage(validationMessage);
  }

  function onInputChange(event: ChangeEvent<HTMLInputElement>) {
    selectFiles(Array.from(event.target.files ?? []));
    event.target.value = "";
  }

  function onDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setDragging(false);
    if (!busy) selectFiles(Array.from(event.dataTransfer.files));
  }

  async function startUpload() {
    if (files.length === 0 || busy) return;
    const validationMessage = validateFiles(files);
    if (validationMessage) {
      setState("error");
      setMessage(validationMessage);
      return;
    }

    setMessage("");
    setCompleted(null);
    setState("creating");
    setProgress(1);

    try {
      const created = await createAnonymousTransfer(files);
      const uploaded = files.map(() => 0);
      const uploadBytes = Math.max(1, selectedBytes);
      let nextIndex = 0;
      setState("uploading");
      setCanCancel(true);
      abortersRef.current = [];

      async function worker() {
        while (nextIndex < files.length) {
          const index = nextIndex;
          nextIndex += 1;
          const file = files[index];
          const target = created.uploads[index];
          setActiveFile(file.name);
          const upload = uploadResumable(file, target.uploadTarget, (filePercent) => {
            uploaded[index] = file.size * (filePercent / 100);
            const combined = uploaded.reduce((sum, bytes) => sum + bytes, 0);
            setProgress(Math.min(99, Math.round((combined / uploadBytes) * 100)));
          });
          abortersRef.current.push(upload.abort);
          await upload.promise;
          await completeTransferFile(created.transfer.id, target.file.id);
        }
      }

      await Promise.all(Array.from({ length: Math.min(UPLOAD_CONCURRENCY, files.length) }, () => worker()));
      abortersRef.current = [];
      setCanCancel(false);
      setState("finalizing");

      const shareURL = new URL(created.sharePath, window.location.origin).toString();
      setCompleted({ shareURL, expiresAt: created.share.expiresAt ?? created.transfer.expiresAt });
      setProgress(100);
      setState("success");
    } catch (error) {
      abortersRef.current.forEach((abort) => abort());
      abortersRef.current = [];
      setCanCancel(false);
      setState("error");
      if (error instanceof APIError) {
        setMessage(error.message);
      } else if (error instanceof DOMException && error.name === "AbortError") {
        setMessage("Upload canceled.");
      } else {
        setMessage("Something went wrong while preparing your link. Please try again.");
      }
    }
  }

  function cancelUpload() {
    abortersRef.current.forEach((abort) => abort());
  }

  function reset() {
    setFiles([]);
    setCompleted(null);
    setMessage("");
    setProgress(0);
    setActiveFile("");
    setState("idle");
    setCopied(false);
    setCanCancel(false);
  }

  async function copyLink() {
    if (!completed) return;
    await navigator.clipboard.writeText(completed.shareURL);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  }

  if (state === "success" && completed) {
    return (
      <section className="upload-card success-card" aria-live="polite">
        <div className="success-icon"><CheckIcon /></div>
        <p className="eyebrow">Ready to share</p>
        <h2>Your link is live.</h2>
        <p className="card-copy">
          {files.length} {files.length === 1 ? "file is" : "files are"} ready. The ZIP is being prepared in the background.
        </p>
        <div className="share-link-row">
          <input aria-label="Share link" readOnly value={completed.shareURL} onFocus={(event) => event.currentTarget.select()} />
          <button className="copy-button" type="button" onClick={copyLink}>
            {copied ? <CheckIcon /> : <CopyIcon />}
            {copied ? "Copied" : "Copy"}
          </button>
        </div>
        <div className="expiry-note">
          <span className="status-dot" />
          Expires {formatExpiry(completed.expiresAt)}
        </div>
        <button className="text-button" type="button" onClick={reset}>Share more files</button>
      </section>
    );
  }

  return (
    <section className="upload-card" aria-labelledby="upload-title">
      <div className="card-heading">
        <div>
          <p className="eyebrow">{variant === "workspace" ? "Temporary transfer" : "Anonymous transfer"}</p>
          <h2 id="upload-title">{variant === "workspace" ? "Create a link" : "Send files"}</h2>
        </div>
        <span className="secure-pill"><span /> Private transfer</span>
      </div>

      {files.length === 0 ? (
        <div
          className={`drop-zone ${dragging ? "is-dragging" : ""}`}
          onDragEnter={(event) => { event.preventDefault(); setDragging(true); }}
          onDragOver={(event) => event.preventDefault()}
          onDragLeave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node)) setDragging(false); }}
          onDrop={onDrop}
        >
          <div className="upload-icon"><UploadIcon /></div>
          <p className="drop-title">Drop your files here</p>
          <p className="drop-subtitle">or choose up to {MAX_FILES} from your device</p>
          <label className="secondary-button file-picker-label" htmlFor={`${variant}-files`}>Choose files</label>
          <p className="limit-copy">Up to {formatBytes(MAX_TRANSFER_BYTES)} total · Link expires in 24 hours</p>
        </div>
      ) : (
        <div className="selected-file transfer-selection">
          <div className="transfer-summary">
            <strong>{files.length} {files.length === 1 ? "file" : "files"}</strong>
            <span>{formatBytes(selectedBytes)} of {formatBytes(MAX_TRANSFER_BYTES)}</span>
          </div>
          <div className="file-list">
            {files.map((file, index) => (
              <div className="file-summary compact" key={`${file.name}-${file.size}-${index}`}>
                <span className="file-icon"><FileIcon /></span>
                <span className="file-details">
                  <strong title={file.name}>{file.name}</strong>
                  <span>{formatBytes(file.size)}{file.type ? ` · ${file.type}` : ""}</span>
                </span>
              </div>
            ))}
          </div>

          {busy && (
            <div className="progress-block" aria-live="polite">
              <div className="progress-label">
                <span>
                  {state === "creating" ? "Preparing secure uploads" : state === "finalizing" ? "Creating your share link" : `Uploading ${activeFile}`}
                </span>
                <strong>{state === "finalizing" ? "Almost done" : `${progress}%`}</strong>
              </div>
              <div className="progress-track"><span style={{ width: `${progress}%` }} /></div>
            </div>
          )}

          {state === "error" && <p className="error-message" role="alert">{message}</p>}

          <div className="upload-actions">
            {busy ? (
              <button className="text-button danger" type="button" onClick={cancelUpload} disabled={!canCancel}>Cancel upload</button>
            ) : (
              <>
                <button className="primary-button" type="button" onClick={startUpload} disabled={state === "error"}>
                  <UploadIcon /> Create 24-hour link
                </button>
                <label className="text-button file-picker-label" htmlFor={`${variant}-files`}>Choose different files</label>
              </>
            )}
          </div>
        </div>
      )}

      <input id={`${variant}-files`} className="visually-hidden" type="file" multiple onChange={onInputChange} />
    </section>
  );
}
