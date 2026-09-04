"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-context";
import { FileIcon, FolderIcon, LinkIcon, UsersIcon } from "@/components/icons";
import { UploadWorkspace } from "@/components/upload-workspace";

function firstName(displayName: string, email: string) {
  const value = displayName.trim() || email;
  return value.split(/\s+/)[0];
}

export function AppDashboard() {
  const router = useRouter();
  const { configured, loading, user } = useAuth();

  useEffect(() => {
    if (!loading && !user) router.replace("/");
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
        <a className="primary-button compact-button" href="#temporary-transfer">
          <LinkIcon /> Send a 24-hour link
        </a>
      </section>

      <section className="library-panel" aria-labelledby="library-title">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Library</p>
            <h2 id="library-title">Your files</h2>
          </div>
          <span className="status-pill">Private to you</span>
        </div>

        <div className="library-empty">
          <span className="empty-icon"><FileIcon /></span>
          <div>
            <h3>Your library is ready.</h3>
            <p>Persistent uploads will appear here when file storage is connected to your account.</p>
          </div>
        </div>

        <div className="workspace-columns">
          <article className="workspace-summary-card">
            <span className="summary-icon"><FolderIcon /></span>
            <div>
              <h3>Folders</h3>
              <p>Your folders will keep files grouped without changing how they are stored.</p>
            </div>
            <span className="coming-label">Coming next</span>
          </article>
          <article className="workspace-summary-card">
            <span className="summary-icon apricot"><UsersIcon /></span>
            <div>
              <h3>Shared with you</h3>
              <p>Read-only folders shared by other Eterealink users will show up here.</p>
            </div>
            <span className="coming-label">Coming next</span>
          </article>
        </div>
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
