"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-context";
import { LinkIcon, UploadIcon } from "@/components/icons";
import { PersistentFileLibrary } from "@/components/persistent-file-library";
import { UploadWorkspace } from "@/components/upload-workspace";

function firstName(displayName: string, email: string) {
  const value = displayName.trim() || email;
  return value.split(/\s+/)[0];
}

export function AppDashboard() {
  const router = useRouter();
  const { configured, loading, user } = useAuth();

  useEffect(() => {
    if (!loading && !user) {
      const inviteCode = new URLSearchParams(window.location.search).get("folderInvite");
      router.replace(inviteCode ? `/join/${encodeURIComponent(inviteCode)}` : "/");
    }
  }, [loading, router, user]);

  if (!configured || loading || !user) {
    return (
      <div className="session-screen" role="status" aria-label="Checking your session">
        <div className="loading-orbit"><span /></div>
        <p>Opening your workspace…</p>
      </div>
    );
  }

  return (
    <div className="app-page">
      <div className="app-ambient" />
      <section className="workspace-intro">
        <div>
          <p className="eyebrow">Your workspace</p>
          <h1>Welcome back, {firstName(user.displayName, user.email)}.</h1>
          <p>Keep what matters, organize it simply, and share only when you choose.</p>
        </div>
        <div className="workspace-primary-actions">
          <label className="primary-button compact-button" htmlFor="owned-files-input">
            <UploadIcon /> Upload files
          </label>
          <a className="secondary-button compact-button" href="#temporary-transfer">
            <LinkIcon /> Send a 24-hour link
          </a>
        </div>
      </section>

      <section className="library-panel" aria-labelledby="library-title">
        <PersistentFileLibrary />
      </section>

      <section id="temporary-transfer" className="temporary-section" aria-labelledby="temporary-title">
        <div className="temporary-copy">
          <p className="eyebrow">Quick transfer</p>
          <h2 id="temporary-title">Need to send something now?</h2>
          <p>Create the same private, direct link you already use. It remains separate from your library and expires after 24 hours.</p>
          <ul>
            <li><span /> Up to 10 files in one link</li>
            <li><span /> Direct browser-to-storage upload</li>
            <li><span /> Automatic 24-hour expiration</li>
          </ul>
        </div>
        <UploadWorkspace variant="workspace" />
      </section>
    </div>
  );
}
