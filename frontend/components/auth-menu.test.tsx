import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthMenu } from "./auth-menu";

const authState = vi.hoisted(() => ({
  configured: true,
  loading: false,
  busy: false,
  user: null as null | {
    id: string;
    email: string;
    displayName: string;
    identityDisplayName?: string;
    customDisplayName?: string | null;
    createdAt: string;
  },
  error: "",
  signIn: vi.fn(async () => {}),
  signOut: vi.fn(async () => {}),
  updateProfile: vi.fn(async (displayName: string | null) => ({
    id: "user-1",
    email: "person@example.com",
    displayName: displayName ?? "Google Person",
    identityDisplayName: "Google Person",
    customDisplayName: displayName,
    createdAt: "2026-09-03T00:00:00Z",
  })),
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
    const button = Array.from(container.querySelectorAll("button")).find((candidate) => candidate.textContent === "Sign out");
    act(() => button?.click());
    expect(authState.signOut).toHaveBeenCalledOnce();
  });

  it("edits and removes an optional custom display name", async () => {
    authState.user = {
      id: "user-1",
      email: "person@example.com",
      displayName: "Johnny",
      identityDisplayName: "Google Person",
      customDisplayName: "Johnny",
      createdAt: "2026-09-03T00:00:00Z",
    };
    const container = renderMenu();
    const edit = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Edit profile");
    act(() => edit?.click());

    expect(container.textContent).toContain("Google name");
    expect(container.textContent).toContain("optional, unique");
    expect(container.querySelector('[aria-label="Close profile editor"]')).not.toBeNull();
    expect(container.textContent).not.toContain("Cancel");
    const input = container.querySelector<HTMLInputElement>("#custom-display-name");
    expect(input?.value).toBe("Johnny");
    act(() => {
      if (input) {
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
        setter?.call(input, "Johnny Cloud");
        input.dispatchEvent(new Event("input", { bubbles: true }));
      }
    });
    const form = container.querySelector("form");
    await act(async () => form?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
    expect(authState.updateProfile).toHaveBeenCalledWith("Johnny Cloud");

    const remove = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Use Google name instead");
    await act(async () => remove?.click());
    expect(authState.updateProfile).toHaveBeenCalledWith(null);
  });

  it("closes the profile editor without changing the saved name", () => {
    authState.user = {
      id: "user-1",
      email: "person@example.com",
      displayName: "Johnny",
      identityDisplayName: "Google Person",
      customDisplayName: "Johnny",
      createdAt: "2026-09-03T00:00:00Z",
    };
    const container = renderMenu();
    const details = container.querySelector("details")!;
    act(() => {
      details.open = true;
      details.dispatchEvent(new Event("toggle"));
    });
    const edit = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Edit profile");
    act(() => edit?.click());
    const close = container.querySelector<HTMLButtonElement>('[aria-label="Close profile editor"]');
    act(() => close?.click());

    expect(container.textContent).toContain("Edit profile");
    expect(container.querySelector("#custom-display-name")).toBeNull();
    expect(authState.updateProfile).not.toHaveBeenCalled();

    act(() => {
      Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Edit profile")?.click();
      document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    });
    expect(container.querySelector("#custom-display-name")).toBeNull();

    act(() => {
      Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Edit profile")?.click();
      document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    });
    expect(details.open).toBe(false);
    expect(authState.updateProfile).not.toHaveBeenCalled();
  });
});
