"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { APIError, resolveShare } from "@/lib/api";
import { formatBytes, formatExpiry, timeRemaining } from "@/lib/format";
import type { ShareResult } from "@/lib/types";
import { isTransferResult } from "@/lib/types";
import { DownloadIcon, FileIcon } from "@/components/icons";
import { FilePreview } from "@/components/file-preview";

type LoadState = "loading" | "ready" | "expired" | "missing" | "unavailable";

function expiresAt(result: ShareResult) {
  return result.share.expiresAt ?? (isTransferResult(result) ? result.transfer.expiresAt : result.file.expiresAt);
}

export function ShareView({ code }: { code: string }) {
  const [state, setState] = useState<LoadState>("loading");
  const [transfer, setTransfer] = useState<ShareResult | null>(null);
  const [remaining, setRemaining] = useState("");
	const [activePreviewID, setActivePreviewID] = useState("");

	const acceptResult = useCallback((result: ShareResult) => {
		setTransfer(result);
		setRemaining(timeRemaining(expiresAt(result)));
		if (isTransferResult(result)) {
			setActivePreviewID((current) => result.files.some((item) => item.file.id === current && item.preview)
				? current
				: result.files.find((item) => item.preview)?.file.id ?? "");
		}
		setState("ready");
	}, []);

  const load = useCallback(async (silent = false) => {
    if (!silent) setState("loading");
    try {
      const result = await resolveShare(code);
		acceptResult(result);
    } catch (error) {
      if (silent) return;
      if (error instanceof APIError && (error.code === "expired" || error.code === "revoked" || error.status === 410)) {
        setState("expired");
      } else if (error instanceof APIError && error.status === 404) {
        setState("missing");
      } else {
        setState("unavailable");
      }
    }
	}, [acceptResult, code]);

  useEffect(() => {
    let active = true;
    resolveShare(code)
      .then((result) => {
        if (!active) return;
		acceptResult(result);
      })
      .catch((error: unknown) => {
        if (!active) return;
        if (error instanceof APIError && (error.code === "expired" || error.code === "revoked" || error.status === 410)) {
          setState("expired");
        } else if (error instanceof APIError && error.status === 404) {
          setState("missing");
        } else {
          setState("unavailable");
        }
      });
    return () => { active = false; };
	}, [acceptResult, code]);

  useEffect(() => {
    if (state !== "ready" || !transfer || !isTransferResult(transfer)) return;
    if (!["WAITING", "PENDING", "BUILDING"].includes(transfer.archive.status)) return;
    const timer = window.setInterval(() => void load(true), 2_000);
    return () => window.clearInterval(timer);
  }, [load, state, transfer]);

  useEffect(() => {
    if (state !== "ready" || !transfer) return;
    const expiry = expiresAt(transfer);
    const timer = window.setInterval(() => {
      const next = timeRemaining(expiry);
      setRemaining(next);
      if (next === "Expired") setState("expired");
    }, 30_000);
    return () => window.clearInterval(timer);
  }, [state, transfer]);

  if (state === "loading") {
    return (
      <section className="share-card loading-card" aria-live="polite">
        <div className="loading-orbit"><span /></div>
        <p>Opening secure link…</p>
      </section>
    );
  }

  if (state !== "ready" || !transfer) {
    const content = state === "expired"
      ? ["This link has expired", "Anonymous transfers are available for 24 hours. Ask the sender to create a new link."]
      : state === "missing"
        ? ["We can’t find that link", "Check the address for a typo, or ask the sender for the complete share link."]
        : ["This link can’t be opened right now", "The service may be temporarily unavailable. Try again in a moment."];

    return (
      <section className="share-card unavailable-card">
        <span className="unavailable-mark" aria-hidden="true">{state === "expired" ? "24" : "?"}</span>
        <p className="eyebrow">Link unavailable</p>
        <h1>{content[0]}</h1>
        <p>{content[1]}</p>
        {state === "unavailable" && <button className="secondary-button" type="button" onClick={() => void load()}>Try again</button>}
        <Link className="primary-button" href="/">Share files</Link>
      </section>
    );
  }

  const expiry = expiresAt(transfer);
  if (isTransferResult(transfer)) {
    const total = transfer.files.reduce((sum, item) => sum + item.file.sizeBytes, 0);
    const archiveReady = transfer.archive.status === "READY" && transfer.archive.downloadTarget;
    const archiveFailed = transfer.archive.status === "FAILED";
		const activePreview = transfer.files.find((item) => item.file.id === activePreviewID);
    return (
      <section className="share-card ready-share bundle-share">
        <div className="share-file-icon"><FileIcon /></div>
        <p className="eyebrow">Files were shared with you</p>
        <h1>{transfer.files.length} {transfer.files.length === 1 ? "file" : "files"}</h1>
        <div className="metadata-row">
          <span>{formatBytes(total)}</span>
          <span aria-hidden="true">•</span>
          <span>Available for 24 hours</span>
        </div>

        {archiveReady ? (
          <a className="primary-button download-button" href={transfer.archive.downloadTarget?.url}>
            <DownloadIcon /> Download all as ZIP
          </a>
        ) : (
          <button className="primary-button download-button" type="button" disabled>
            <DownloadIcon /> {archiveFailed ? "ZIP unavailable" : "Preparing ZIP…"}
          </button>
        )}

        <div className="bundle-file-list" aria-label="Files in this transfer">
			{transfer.files.map(({ file, downloadTarget, preview }) => (
            <div className="bundle-file" key={file.id}>
              <span className="file-details">
                <strong title={file.originalName}>{file.originalName}</strong>
                <span>{formatBytes(file.sizeBytes)}</span>
              </span>
				{preview && <button type="button" className={activePreviewID === file.id ? "is-active" : ""} onClick={() => setActivePreviewID(file.id)}>Preview</button>}
				<a href={downloadTarget.url} aria-label={`Download ${file.originalName}`}><DownloadIcon /></a>
            </div>
          ))}
        </div>
		{activePreview && <FilePreview key={activePreview.file.id} name={activePreview.file.originalName} preview={activePreview.preview} />}

        {archiveFailed && <p className="safety-copy">The ZIP could not be prepared, but each file can still be downloaded separately.</p>}
        <div className="share-expiry">
          <span className="status-dot" />
          <span><strong>{remaining}</strong><small>Expires {formatExpiry(expiry)}</small></span>
        </div>
      </section>
    );
  }

  return (
	<section className={`share-card ready-share${transfer.preview ? ` preview-share preview-share-${transfer.preview.kind}` : ""}`}>
      <div className="share-file-icon"><FileIcon /></div>
      <p className="eyebrow">A file was shared with you</p>
      <h1>{transfer.file.originalName}</h1>
      <div className="metadata-row">
        <span>{formatBytes(transfer.file.sizeBytes)}</span>
        <span aria-hidden="true">•</span>
        <span>{transfer.file.mimeType || "File"}</span>
      </div>
		<FilePreview name={transfer.file.originalName} preview={transfer.preview} />
      <a className="primary-button download-button" href={transfer.downloadTarget.url}>
        <DownloadIcon /> Download file
      </a>
      <div className="share-expiry">
        <span className="status-dot" />
        <span><strong>{remaining}</strong><small>Expires {formatExpiry(expiry)}</small></span>
      </div>
      <p className="safety-copy">The download address is temporary and is generated only when this page opens.</p>
    </section>
  );
}
