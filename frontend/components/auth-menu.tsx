"use client";

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
        <button className="auth-button" type="button" onClick={signIn} disabled={busy}>
          {busy ? "Signing in…" : "Sign in with Google"}
        </button>
        {error && <span className="auth-error" role="alert">{error}</span>}
      </span>
    );
  }

  const initial = (user.displayName || user.email).charAt(0).toLocaleUpperCase();
  return (
    <span className="auth-control signed-in">
      <span className="user-avatar" aria-hidden="true">{initial}</span>
      <span className="user-summary">
        <strong>{user.displayName}</strong>
        <span>{user.email}</span>
      </span>
      <button className="sign-out-button" type="button" onClick={signOut} disabled={busy}>Sign out</button>
      {error && <span className="auth-error" role="alert">{error}</span>}
    </span>
  );
}
