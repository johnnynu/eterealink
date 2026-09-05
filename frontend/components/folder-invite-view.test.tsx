import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FolderInviteView } from "./folder-invite-view";

const router = vi.hoisted(() => ({ replace: vi.fn() }));
const api = vi.hoisted(() => ({ acceptFolderInvite: vi.fn(), getFolderInvitePreview: vi.fn() }));
const MockAPIError = vi.hoisted(() => class APIError extends Error {
  constructor(message: string, readonly status: number, readonly code: string) {
    super(message);
  }
});
const auth = vi.hoisted(() => ({
  configured: true,
  loading: false,
  busy: false,
  user: null as null | { id: string; email: string; displayName: string; createdAt: string },
  error: "",
  getIDToken: vi.fn(async () => "verified-token"),
  signIn: vi.fn(async () => {}),
}));

vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/components/auth-context", () => ({ useAuth: () => auth }));
vi.mock("@/lib/api", () => ({ APIError: MockAPIError, ...api }));

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

beforeEach(() => {
  auth.configured = true;
  auth.loading = false;
  auth.busy = false;
  auth.user = null;
  auth.error = "";
	api.getFolderInvitePreview.mockResolvedValue({
		folderName: "Launch plans",
		ownerName: "Morgan Lee",
		role: "CONTRIBUTOR",
		expiresAt: "2026-09-11T12:00:00Z",
	});
});

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
  vi.clearAllMocks();
});

async function renderInvite() {
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  mounted.push({ container, unmount: () => root.unmount() });
  await act(async () => {
    root.render(<FolderInviteView code="join-code" />);
    await Promise.resolve();
  });
  return container;
}

describe("FolderInviteView", () => {
  it("keeps a first-time recipient on the invitation while they sign in", async () => {
    const container = await renderInvite();
    expect(container.textContent).toContain("Morgan Lee invited you.");
		expect(container.textContent).toContain("“Launch plans” as a contributor");
		expect(container.textContent).toContain("browse, download, and upload your own files");
    const button = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Sign in with Google"));
    await act(async () => { button?.click(); });
    expect(auth.signIn).toHaveBeenCalledOnce();
    expect(api.acceptFolderInvite).not.toHaveBeenCalled();
  });

  it("accepts the invitation after authentication and opens the shared folder", async () => {
    auth.user = { id: "user-1", email: "person@example.com", displayName: "Person", createdAt: "2026-09-04T12:00:00Z" };
    api.acceptFolderInvite.mockResolvedValue({
      folder: { id: "folder-1", ownerId: "owner-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
      role: "VIEWER",
      owner: { id: "owner-1", email: "owner@example.com", displayName: "Owner", createdAt: "2026-09-04T12:00:00Z" },
    });
    await renderInvite();
    expect(api.acceptFolderInvite).toHaveBeenCalledWith("join-code", "verified-token");
		expect(router.replace).toHaveBeenCalledWith("/app?folder=folder-1&scope=shared");
  });

  it("shows a specific state for an expired invitation", async () => {
    auth.user = { id: "user-1", email: "person@example.com", displayName: "Person", createdAt: "2026-09-04T12:00:00Z" };
    api.acceptFolderInvite.mockRejectedValue(new MockAPIError("expired", 410, "expired"));
    const container = await renderInvite();
    expect(container.textContent).toContain("This invitation has expired");
    expect(container.textContent).toContain("Ask the folder owner to create a new invitation link.");
  });

	it("shows an expired link before asking a signed-out recipient to authenticate", async () => {
		api.getFolderInvitePreview.mockRejectedValue(new MockAPIError("expired", 410, "expired"));
		const container = await renderInvite();
		expect(container.textContent).toContain("This invitation has expired");
		expect(container.textContent).not.toContain("Sign in with Google to join");
		expect(api.acceptFolderInvite).not.toHaveBeenCalled();
	});
});
