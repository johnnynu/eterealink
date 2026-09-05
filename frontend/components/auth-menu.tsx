"use client";

import Image from "next/image";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useAuth } from "@/components/auth-context";
import { APIError } from "@/lib/api";

export function AuthMenu() {
  const { configured, loading, busy, user, error, signIn, signOut, updateProfile } = useAuth();
  const [editing, setEditing] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [saving, setSaving] = useState(false);
  const [profileMessage, setProfileMessage] = useState("");
  const [profileError, setProfileError] = useState("");
  const menuRef = useRef<HTMLDetailsElement>(null);

  useEffect(() => {
    function resetEditor() {
      setEditing(false);
      setProfileMessage("");
      setProfileError("");
    }

    function closeOnOutsideClick(event: PointerEvent) {
      const menu = menuRef.current;
      if (!menu?.open || menu.contains(event.target as Node)) return;
      menu.open = false;
      resetEditor();
    }

    function closeOnEscape(event: KeyboardEvent) {
      const menu = menuRef.current;
      if (event.key !== "Escape" || !menu?.open) return;
      menu.open = false;
      resetEditor();
    }

    document.addEventListener("pointerdown", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, []);
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

  function beginEditing() {
    setDisplayName(user?.customDisplayName ?? "");
    setProfileMessage("");
    setProfileError("");
    setEditing(true);
  }

  function cancelEditing() {
    setEditing(false);
    setProfileMessage("");
    setProfileError("");
  }

  async function saveDisplayName(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setProfileMessage("");
    setProfileError("");
    try {
      const updated = await updateProfile(displayName);
      setDisplayName(updated.customDisplayName ?? "");
      setProfileMessage("Display name saved.");
    } catch (saveError) {
      if (saveError instanceof APIError && saveError.code === "display_name_taken") {
        setProfileError("That display name is already in use.");
      } else if (saveError instanceof APIError && saveError.code === "validation_failed") {
        setProfileError(saveError.message);
      } else {
        setProfileError("We could not save your display name. Please try again.");
      }
    } finally {
      setSaving(false);
    }
  }

  async function removeDisplayName() {
    setSaving(true);
    setProfileMessage("");
    setProfileError("");
    try {
      await updateProfile(null);
      setDisplayName("");
      setProfileMessage("Custom display name removed.");
    } catch {
      setProfileError("We could not remove your display name. Please try again.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <details
      ref={menuRef}
      className={`auth-control account-menu ${editing ? "is-editing" : ""}`}
      onToggle={(event) => {
        if (!event.currentTarget.open) cancelEditing();
      }}
    >
      <summary aria-label={`Open account menu for ${user.displayName || user.email}`}>
        <span className="user-avatar" aria-hidden="true">{initial}</span>
        <span className="user-summary">
          <strong>{user.displayName || user.email}</strong>
          <span>Account</span>
        </span>
        <span className="account-chevron" aria-hidden="true">⌄</span>
      </summary>
      <div className="account-popover">
        {editing ? (
          <form className="profile-form" onSubmit={saveDisplayName}>
            <div className="profile-form-heading">
              <div className="profile-form-heading-row">
                <strong>Edit profile</strong>
                <button className="profile-close-button" type="button" onClick={cancelEditing} disabled={saving} aria-label="Close profile editor">×</button>
              </div>
              <span>Your custom name is optional, unique, and shown to collaborators.</span>
            </div>
            <div className="identity-name-row">
              <span>Google name</span>
              <strong>{user.identityDisplayName || user.email}</strong>
            </div>
            <label htmlFor="custom-display-name">Custom display name</label>
            <input
              id="custom-display-name"
              name="displayName"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              minLength={3}
              maxLength={40}
              autoComplete="nickname"
              disabled={saving}
              aria-describedby="display-name-help"
            />
            <small id="display-name-help">3–40 characters. Spaces are normalized.</small>
            {profileError && <p className="profile-form-error" role="alert">{profileError}</p>}
            {profileMessage && <p className="profile-form-success" role="status">{profileMessage}</p>}
            <div className="profile-form-actions">
              <button className="profile-save-button" type="submit" disabled={saving}>{saving ? "Saving…" : "Save"}</button>
            </div>
            {user.customDisplayName && (
              <button className="remove-display-name" type="button" onClick={removeDisplayName} disabled={saving}>
                Use Google name instead
              </button>
            )}
          </form>
        ) : (
          <>
            <div className="account-identity">
              <strong>{user.displayName || user.email}</strong>
              <span>{user.email}</span>
            </div>
            <button className="account-action-button" type="button" onClick={beginEditing}>Edit profile</button>
            <button className="sign-out-button" type="button" onClick={signOut} disabled={busy}>
              {busy ? "Signing out…" : "Sign out"}
            </button>
          </>
        )}
        {error && <span className="auth-error" role="alert">{error}</span>}
      </div>
    </details>
  );
}
