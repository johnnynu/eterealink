"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-context";
import { FolderIcon } from "@/components/icons";
import { acceptFolderInvite, APIError, getFolderInvitePreview } from "@/lib/api";
import { formatExpiry } from "@/lib/format";
import type { FolderInvitePreview } from "@/lib/types";

type JoinState = "idle" | "accepting" | "expired" | "revoked" | "missing" | "unavailable";
type PreviewState = "loading" | "ready" | "expired" | "revoked" | "missing" | "unavailable";

export function FolderInviteView({ code }: { code: string }) {
  const router = useRouter();
  const { configured, loading, busy, user, error: authError, getIDToken, signIn } = useAuth();
  const [state, setState] = useState<JoinState>("idle");
  const [previewState, setPreviewState] = useState<PreviewState>("loading");
  const [preview, setPreview] = useState<FolderInvitePreview | null>(null);
  const attemptedKey = useRef("");

  const loadPreview = useCallback(async () => {
    setPreviewState("loading");
    try {
      setPreview(await getFolderInvitePreview(code));
      setPreviewState("ready");
    } catch (inviteError) {
      if (inviteError instanceof APIError && inviteError.code === "expired") setPreviewState("expired");
      else if (inviteError instanceof APIError && inviteError.code === "revoked") setPreviewState("revoked");
      else if (inviteError instanceof APIError && inviteError.status === 410) setPreviewState("expired");
      else if (inviteError instanceof APIError && inviteError.status === 404) setPreviewState("missing");
      else setPreviewState("unavailable");
    }
  }, [code]);

  useEffect(() => {
    let active = true;
    queueMicrotask(() => {
      if (active) void loadPreview();
    });
    return () => { active = false; };
  }, [loadPreview]);

  const acceptInvite = useCallback(async (retry = false) => {
    if (!user || !code) return;
    const key = `${user.id}:${code}`;
    if (!retry && attemptedKey.current === key) return;
    attemptedKey.current = key;
    try {
      const token = await getIDToken();
      setState("accepting");
      const access = await acceptFolderInvite(code, token);
			router.replace(`/app?folder=${encodeURIComponent(access.folder.id)}&scope=shared`);
    } catch (inviteError) {
      if (inviteError instanceof APIError && inviteError.code === "expired") setState("expired");
      else if (inviteError instanceof APIError && inviteError.code === "revoked") setState("revoked");
      else if (inviteError instanceof APIError && inviteError.status === 410) setState("expired");
      else if (inviteError instanceof APIError && inviteError.status === 404) setState("missing");
      else setState("unavailable");
    }
  }, [code, getIDToken, router, user]);

  useEffect(() => {
    if (loading || !user || previewState !== "ready") return;
    let active = true;
    queueMicrotask(() => {
      if (active) void acceptInvite();
    });
    return () => { active = false; };
  }, [acceptInvite, loading, previewState, user]);

  if (loading || previewState === "loading") {
    return (
      <div className="invite-page">
        <section className="share-card invite-card loading-card" aria-live="polite">
          <div className="loading-orbit"><span /></div>
          <p>Checking your invitation…</p>
        </section>
      </div>
    );
  }

  if (previewState !== "ready" || !preview) {
    const previewContent = inviteErrorContent(previewState);
    return (
      <InviteUnavailable
        title={previewContent[0]}
        message={previewContent[1]}
        onRetry={previewState === "unavailable" ? loadPreview : undefined}
      />
    );
  }

  if (!configured) {
    return <InviteUnavailable title="Sign-in is unavailable" message="Folder invitations require Google Sign-In, which is not configured for this environment." />;
  }

  if (!user) {
    return (
      <div className="invite-page">
        <div className="ambient ambient-one" />
        <section className="share-card invite-card">
          <span className="invite-folder-icon"><FolderIcon /></span>
          <p className="eyebrow">Folder invitation</p>
          <h1>{preview.ownerName} invited you.</h1>
          <p className="invite-description">
            Collaborate in <strong>“{preview.folderName}”</strong> as a {preview.role === "CONTRIBUTOR" ? "contributor" : "viewer"}.
            {preview.role === "CONTRIBUTOR" ? " You can browse, download, and upload your own files." : " You can browse and download its files."}
          </p>
          <button className="primary-button invite-sign-in" type="button" onClick={() => void signIn()} disabled={busy}>
            {busy ? "Signing in…" : "Sign in with Google to join"}
          </button>
          {authError && <p className="error-message" role="alert">{authError}</p>}
          <p className="safety-copy">{preview.expiresAt ? `Join by ${formatExpiry(preview.expiresAt)}. ` : ""}Signing in confirms your identity. The folder owner can remove your access later.</p>
        </section>
      </div>
    );
  }

  if (state === "idle" || state === "accepting") {
    return (
      <div className="invite-page">
        <section className="share-card invite-card loading-card" aria-live="polite">
          <div className="loading-orbit"><span /></div>
          <p>Joining “{preview.folderName}”…</p>
        </section>
      </div>
    );
  }

  const content = inviteErrorContent(state);

  return (
    <div className="invite-page">
      <section className="share-card invite-card unavailable-card">
        <span className="unavailable-mark" aria-hidden="true">?</span>
        <p className="eyebrow">Invitation unavailable</p>
        <h1>{content[0]}</h1>
        <p>{content[1]}</p>
        {state === "unavailable" && <button className="secondary-button" type="button" onClick={() => void acceptInvite(true)}>Try again</button>}
        <Link className="primary-button" href="/app">Go to your files</Link>
      </section>
    </div>
  );
}

function inviteErrorContent(state: PreviewState | JoinState) {
  return state === "expired"
    ? ["This invitation has expired", "Ask the folder owner to create a new invitation link."]
    : state === "revoked"
      ? ["This invitation was revoked", "The folder owner has disabled this invitation link."]
      : state === "missing"
        ? ["We can’t find this invitation", "Check the address for a typo, or ask the folder owner for the complete link."]
        : ["This invitation can’t be opened right now", "The service may be temporarily unavailable. Try again in a moment."];
}

function InviteUnavailable({ title, message, onRetry }: { title: string; message: string; onRetry?: () => Promise<void> }) {
  return (
    <div className="invite-page">
      <section className="share-card invite-card unavailable-card">
        <span className="unavailable-mark" aria-hidden="true">?</span>
        <p className="eyebrow">Invitation unavailable</p>
        <h1>{title}</h1>
        <p>{message}</p>
        {onRetry && <button className="secondary-button" type="button" onClick={() => void onRetry()}>Try again</button>}
        <Link className="primary-button" href="/">Return home</Link>
      </section>
    </div>
  );
}
