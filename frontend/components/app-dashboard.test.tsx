import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppDashboard } from "./app-dashboard";

const replace = vi.hoisted(() => vi.fn());
const authState = vi.hoisted(() => ({
  configured: true,
  loading: false,
  user: null as null | { id: string; email: string; displayName: string; createdAt: string },
  getIDToken: vi.fn(async () => "token"),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
vi.mock("@/components/auth-context", () => ({ useAuth: () => authState }));
vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
	listFolderContents: vi.fn(async () => ({ files: [], folders: [], breadcrumbs: [], summary: { fileCount: 0, totalBytes: 0 } })),
}));

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
  authState.user = null;
	window.history.replaceState({}, "", "/");
  vi.clearAllMocks();
});

function renderDashboard() {
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  mounted.push({ container, unmount: () => root.unmount() });
  act(() => root.render(<AppDashboard />));
  return container;
}

describe("AppDashboard", () => {
	it("shows a useful workspace for an authenticated user", async () => {
    authState.user = {
      id: "user-1",
      email: "person@example.com",
      displayName: "Person Example",
      createdAt: "2026-09-03T00:00:00Z",
    };
	const container = renderDashboard();
	await act(async () => { await Promise.resolve(); });
    expect(container.textContent).toContain("Welcome back, Person.");
    expect(container.textContent).toContain("Your files");
	expect(container.textContent).toContain("Shared with me");
    expect(container.textContent).toContain("Upload files");
    expect(container.textContent).toContain("Create a link");
    expect(container.querySelector("#workspace-files")).not.toBeNull();
  });

  it("returns a signed-out visitor to the public page", () => {
    renderDashboard();
    expect(replace).toHaveBeenCalledWith("/");
  });

	it("preserves a legacy folder invite while sending a signed-out visitor to sign in", () => {
		window.history.replaceState({}, "", "/app?folderInvite=join-code");
		renderDashboard();
		expect(replace).toHaveBeenCalledWith("/join/join-code");
	});
});
