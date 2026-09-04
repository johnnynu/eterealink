"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { AuthMenu } from "@/components/auth-menu";
import { Brand } from "@/components/brand";
import { useAuth } from "@/components/auth-context";

export function SiteHeader() {
  const pathname = usePathname();
  const { loading, user } = useAuth();
  const inWorkspace = pathname.startsWith("/app");
  const signedIn = !loading && Boolean(user);

  return (
    <header className={`site-header ${inWorkspace ? "workspace-header" : ""}`}>
      <Brand href={signedIn ? "/app" : "/"} />
      <nav aria-label="Primary navigation">
        {signedIn ? (
          <>
            <Link className={`workspace-nav-link ${inWorkspace ? "is-active" : ""}`} href="/app">
              Files
            </Link>
            <a className="workspace-nav-link" href="/app#temporary-transfer">
              Send a link
            </a>
          </>
        ) : (
          <>
            <span className="nav-note">No account needed</span>
            <Link className="nav-link" href="/">Send files</Link>
          </>
        )}
        <AuthMenu />
      </nav>
    </header>
  );
}
