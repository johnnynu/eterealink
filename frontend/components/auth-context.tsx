"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  GoogleAuthProvider,
  onIdTokenChanged,
  signInWithPopup,
  signOut as firebaseSignOut,
} from "firebase/auth";
import { getCurrentUser } from "@/lib/api";
import { firebaseConfigured, getFirebaseAuth } from "@/lib/firebase";
import type { UserRecord } from "@/lib/types";

type AuthContextValue = {
  configured: boolean;
  loading: boolean;
  busy: boolean;
  user: UserRecord | null;
  error: string;
  getIDToken: () => Promise<string>;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserRecord | null>(null);
  const [loading, setLoading] = useState(firebaseConfigured);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const auth = getFirebaseAuth();
    if (!auth) {
      return;
    }

    let currentUpdate = 0;
    return onIdTokenChanged(auth, async (firebaseUser) => {
      const update = ++currentUpdate;
      setError("");
      if (!firebaseUser) {
        setUser(null);
        setLoading(false);
        return;
      }
      try {
        const idToken = await firebaseUser.getIdToken();
        const currentUser = await getCurrentUser(idToken);
        if (update === currentUpdate) setUser(currentUser);
      } catch {
        if (update === currentUpdate) {
          setUser(null);
          setError("We could not verify your session. Please sign in again.");
        }
      } finally {
        if (update === currentUpdate) setLoading(false);
      }
    });
  }, []);

  async function signIn() {
    const auth = getFirebaseAuth();
    if (!auth) return;
    setBusy(true);
    setError("");
    try {
      const provider = new GoogleAuthProvider();
      provider.setCustomParameters({ prompt: "select_account" });
      await signInWithPopup(auth, provider);
    } catch (signInError) {
      const code = typeof signInError === "object" && signInError && "code" in signInError
        ? String(signInError.code)
        : "";
      if (code !== "auth/popup-closed-by-user" && code !== "auth/cancelled-popup-request") {
        setError("Google Sign-In could not be completed. Please try again.");
      }
    } finally {
      setBusy(false);
    }
  }

  const getIDToken = useCallback(async () => {
    const firebaseUser = getFirebaseAuth()?.currentUser;
    if (!firebaseUser) throw new Error("A signed-in session is required.");
    return firebaseUser.getIdToken();
  }, []);

  async function signOut() {
    const auth = getFirebaseAuth();
    if (!auth) return;
    setBusy(true);
    setError("");
    try {
      await firebaseSignOut(auth);
      setUser(null);
    } catch {
      setError("Sign-out could not be completed. Please try again.");
    } finally {
      setBusy(false);
    }
  }

  const value = useMemo<AuthContextValue>(() => ({
    configured: firebaseConfigured,
    loading,
    busy,
    user,
    error,
    getIDToken,
    signIn,
    signOut,
  }), [loading, busy, user, error, getIDToken]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used within AuthProvider");
  return value;
}
