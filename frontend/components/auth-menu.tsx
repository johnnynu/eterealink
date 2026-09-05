"use client";

import Image from "next/image";
import { useAuth } from "@/components/auth-context";

export function AuthMenu() {
  const { configured, loading, busy, user, error, signIn, signOut } = useAuth();
  if (!configured) return null;

  if (loading) {
    return <span className="auth-loading" aria-label="Checking sign-in status">Checking session…</span>;
  }

  if (!user) {
    return (
      <span className="auth-control">
        <button className="auth-button google-auth-button" type="button" onClick={signIn} disabled={busy} aria-label={busy ? "Signing in with Google" : undefined}>
          {busy ? (
            <span className="google-auth-busy">Signing in…</span>
          ) : (
            <>
              <Image className="google-sign-in-full" src="/google-sign-in-pill.svg" alt="" width={180} height={40} priority />
              <Image className="google-sign-in-compact" src="/google-sign-in-icon.svg" alt="" width={40} height={40} priority />
              <span className="visually-hidden">Sign in with Google</span>
            </>
          )}
        </button>
        {error && <span className="auth-error" role="alert">{error}</span>}
      </span>
    );
  }

  const initial = (user.displayName || user.email).charAt(0).toLocaleUpperCase();
  return (
    <details className="auth-control account-menu">
      <summary aria-label={`Open account menu for ${user.displayName || user.email}`}>
        <span className="user-avatar" aria-hidden="true">{initial}</span>
        <span className="user-summary">
          <strong>{user.displayName || user.email}</strong>
          <span>Account</span>
        </span>
        <span className="account-chevron" aria-hidden="true">⌄</span>
      </summary>
      <div className="account-popover">
        <div className="account-identity">
          <strong>{user.displayName || user.email}</strong>
          <span>{user.email}</span>
        </div>
        <button className="sign-out-button" type="button" onClick={signOut} disabled={busy}>
          {busy ? "Signing out…" : "Sign out"}
        </button>
        {error && <span className="auth-error" role="alert">{error}</span>}
      </div>
    </details>
  );
}
