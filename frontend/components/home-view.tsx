"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-context";
import { AuthMenu } from "@/components/auth-menu";
import { FileIcon, FolderIcon, LinkIcon, UsersIcon } from "@/components/icons";
import { UploadWorkspace } from "@/components/upload-workspace";

function SessionTransition() {
  return (
    <div className="session-screen" role="status" aria-label="Opening your workspace">
      <div className="loading-orbit"><span /></div>
      <p>Opening your workspace…</p>
    </div>
  );
}

export function HomeView() {
  const router = useRouter();
  const { configured, loading, user } = useAuth();

  useEffect(() => {
    if (user) router.replace("/app");
  }, [router, user]);

  if (configured && (loading || user)) return <SessionTransition />;

  return (
    <div className="home-page">
      <div className="ambient ambient-one" />
      <div className="ambient ambient-two" />
      <section className="hero">
        <div className="hero-copy">
          <p className="hero-kicker"><span /> Files travel light</p>
          <h1>Share your files.<br /><em>Leave no clutter.</em></h1>
          <p className="hero-description">A direct, private handoff with no account and no inbox. Your link quietly disappears after 24 hours.</p>
          <a className="account-discovery-link" href="#account-benefits">Need a home for your files? Explore account features <span aria-hidden="true">↗</span></a>
          <div className="promise-row" aria-label="Transfer features">
            <div><strong>Direct</strong><span>Browser to storage</span></div>
            <div><strong>Private</strong><span>Short-lived access</span></div>
            <div><strong>Temporary</strong><span>24-hour links</span></div>
          </div>
        </div>
        <UploadWorkspace />
      </section>
      <section className="account-benefits" id="account-benefits" aria-labelledby="account-title">
        <div className="account-benefits-intro">
          <p className="eyebrow">A little more room to stay</p>
          <h2 id="account-title">Quick handoff.<br />Or a place to keep it.</h2>
          <p>Upload, share, and download with no account. Sign in when you want to keep your files, stay organized, and share a space with others.</p>
          {configured && (
            <div className="account-benefits-sign-in">
              <AuthMenu />
              <p>New here? Your first Google sign-in creates your account.</p>
            </div>
          )}
        </div>
        <ul className="account-feature-list" aria-label="Features included with an account">
          <li>
            <span className="account-feature-icon"><FileIcon /></span>
            <h3>Keep your files</h3>
            <p>Build a private file library. Your saved files stay beyond the 24-hour window for anonymous transfers.</p>
          </li>
          <li>
            <span className="account-feature-icon"><FolderIcon /></span>
            <h3>Give everything a place</h3>
            <p>Organize files into folders and subfolders. Search, sort, and find what you need when you come back.</p>
          </li>
          <li>
            <span className="account-feature-icon"><UsersIcon /></span>
            <h3>Share a folder, work together</h3>
            <p>Invite viewers to browse and download, or contributors to add their own files to a shared folder.</p>
          </li>
          <li>
            <span className="account-feature-icon"><LinkIcon /></span>
            <h3>Stay in control of links</h3>
            <p>Choose when your file links expire and revoke them when you’re done sharing. Your original files stay in your library.</p>
          </li>
        </ul>
      </section>
      <section className="how-it-works" aria-labelledby="how-title">
        <div>
          <p className="eyebrow">Just need to send something?</p>
          <h2 id="how-title">One link. Three steps.</h2>
        </div>
        <ol>
          <li><span>01</span><strong>Choose</strong><p>Pick up to 10 files, with a combined limit of 1 GB. No sign-up required.</p></li>
          <li><span>02</span><strong>Send</strong><p>Your browser transfers them directly to private object storage.</p></li>
          <li><span>03</span><strong>Share</strong><p>Copy one link. Recipients can download a ZIP or individual files for 24 hours.</p></li>
        </ol>
      </section>
    </div>
  );
}
