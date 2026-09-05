"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { MouseEvent } from "react";
import { AuthMenu } from "@/components/auth-menu";
import { Brand } from "@/components/brand";
import { useAuth } from "@/components/auth-context";

export function SiteHeader() {
  const pathname = usePathname();
  const { loading, user } = useAuth();
  const inWorkspace = pathname.startsWith("/app");
  const signedIn = !loading && Boolean(user);

	function openWorkspaceRoot(event: MouseEvent<HTMLAnchorElement>) {
		if (pathname !== "/app" || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
		event.preventDefault();
		if (window.location.search || window.location.hash) {
			window.history.pushState(null, "", "/app");
			window.dispatchEvent(new PopStateEvent("popstate"));
		}
	}

  return (
    <header className={`site-header ${inWorkspace ? "workspace-header" : ""}`}>
      <Brand href={signedIn ? "/app" : "/"} onClick={signedIn ? openWorkspaceRoot : undefined} />
      <nav aria-label="Primary navigation">
        {signedIn ? (
          <>
            <Link className={`workspace-nav-link ${inWorkspace ? "is-active" : ""}`} href="/app" onClick={openWorkspaceRoot}>
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
