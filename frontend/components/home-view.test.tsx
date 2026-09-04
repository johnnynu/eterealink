import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { HomeView } from "./home-view";

const replace = vi.hoisted(() => vi.fn());
const authState = vi.hoisted(() => ({
  configured: true,
  loading: false,
  user: null as null | { id: string; email: string; displayName: string; createdAt: string },
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("@/components/auth-context", () => ({ useAuth: () => authState }));

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
  authState.loading = false;
  authState.user = null;
  vi.clearAllMocks();
});

function renderHome() {
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  mounted.push({ container, unmount: () => root.unmount() });
  act(() => root.render(<HomeView />));
  return container;
}

describe("HomeView", () => {
  it("keeps the anonymous transfer landing page for signed-out visitors", () => {
    const container = renderHome();
    expect(container.textContent).toContain("Share your files.");
    expect(replace).not.toHaveBeenCalled();
  });

  it("opens the workspace for a verified signed-in user", () => {
    authState.user = {
      id: "user-1",
      email: "person@example.com",
      displayName: "Person",
      createdAt: "2026-09-03T00:00:00Z",
    };
    const container = renderHome();
    expect(container.textContent).toContain("Opening your workspace");
    expect(container.textContent).not.toContain("Share your files.");
    expect(replace).toHaveBeenCalledWith("/app");
  });
});
