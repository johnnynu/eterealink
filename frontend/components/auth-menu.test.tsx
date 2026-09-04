import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthMenu } from "./auth-menu";

const authState = vi.hoisted(() => ({
  configured: true,
  loading: false,
  busy: false,
  user: null as null | { id: string; email: string; displayName: string; createdAt: string },
  error: "",
  signIn: vi.fn(async () => {}),
  signOut: vi.fn(async () => {}),
}));

vi.mock("@/components/auth-context", () => ({ useAuth: () => authState }));

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
  authState.user = null;
  authState.error = "";
  vi.clearAllMocks();
});

function renderMenu() {
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  mounted.push({ container, unmount: () => root.unmount() });
  act(() => root.render(<AuthMenu />));
  return container;
}

describe("AuthMenu", () => {
  it("offers Google Sign-In to an anonymous visitor", () => {
    const container = renderMenu();
    const button = container.querySelector("button");
    expect(button?.textContent).toBe("Sign in with Google");
    act(() => button?.click());
    expect(authState.signIn).toHaveBeenCalledOnce();
  });

  it("shows the provisioned account and sign-out action", () => {
    authState.user = {
      id: "user-1",
      email: "person@example.com",
      displayName: "Person",
      createdAt: "2026-09-03T00:00:00Z",
    };
    const container = renderMenu();
    expect(container.textContent).toContain("Person");
    expect(container.textContent).toContain("person@example.com");
    const button = container.querySelector("button");
    act(() => button?.click());
    expect(authState.signOut).toHaveBeenCalledOnce();
  });
});
