import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PersistentFileLibrary } from "./persistent-file-library";
import type { FileRecord } from "@/lib/types";

const api = vi.hoisted(() => ({
	acceptFolderInvite: vi.fn(),
	addFolderMember: vi.fn(),
  completePersistentUpload: vi.fn(),
	createFolder: vi.fn(),
	createFolderInvite: vi.fn(),
  createPersistentFileShare: vi.fn(),
  createPersistentUpload: vi.fn(),
  deletePersistentFile: vi.fn(),
	deleteFolder: vi.fn(),
  getPersistentFileDownload: vi.fn(),
	listFolderContents: vi.fn(),
	listFolderInvites: vi.fn(),
	listFolderMembers: vi.fn(),
	movePersistentFiles: vi.fn(),
	removeContributedFile: vi.fn(),
	removeFolderMember: vi.fn(),
	revokeFolderInvite: vi.fn(),
  revokePersistentFileShare: vi.fn(),
	streamFolderEvents: vi.fn(),
  uploadResumable: vi.fn(),
	updateFolder: vi.fn(),
}));
const getIDToken = vi.hoisted(() => vi.fn(async () => "verified-token"));
const signedInUser = vi.hoisted(() => ({ id: "user-1", email: "me@example.com", displayName: "Me", createdAt: "2026-09-03T12:00:00Z" }));

vi.mock("@/components/auth-context", () => ({ useAuth: () => ({ getIDToken, user: signedInUser }) }));
vi.mock("@/lib/api", () => ({
  APIError: class APIError extends Error {},
  ...api,
}));

const savedFile: FileRecord = {
  id: "file-1",
	ownerId: "user-1",
  originalName: "project-notes.txt",
  mimeType: "text/plain",
  sizeBytes: 5,
  status: "READY",
  createdAt: "2026-09-03T12:00:00Z",
  completedAt: "2026-09-03T12:01:00Z",
};
const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

function library(files: Array<{ file: FileRecord; uploaderName?: string; share?: object; sharePath?: string }>) {
  return {
    files,
	folders: [],
	breadcrumbs: [],
    summary: {
      fileCount: files.length,
      totalBytes: files.reduce((total, entry) => total + entry.file.sizeBytes, 0),
    },
  };
}

beforeEach(() => {
	window.history.replaceState(null, "", "/app");
	api.streamFolderEvents.mockImplementation((_token, _folderID, signal: AbortSignal) => new Promise<void>((resolve) => {
		signal.addEventListener("abort", () => resolve(), { once: true });
	}));
});

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
	window.history.replaceState(null, "", "/");
  vi.clearAllMocks();
});

async function renderLibrary() {
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  mounted.push({ container, unmount: () => root.unmount() });
  await act(async () => {
    root.render(<PersistentFileLibrary />);
    await Promise.resolve();
  });
  return container;
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

describe("PersistentFileLibrary", () => {
  it("lists ready files and requires confirmation before deletion", async () => {
	api.listFolderContents.mockResolvedValue(library([{ file: savedFile }]));
    api.deletePersistentFile.mockResolvedValue(undefined);
    const container = await renderLibrary();

    expect(container.textContent).toContain("project-notes.txt");
    const deleteButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Delete");
    act(() => deleteButton?.click());
    expect(api.deletePersistentFile).not.toHaveBeenCalled();

    const confirmButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Delete permanently");
    await act(async () => { confirmButton?.click(); });
    expect(api.deletePersistentFile).toHaveBeenCalledWith("file-1", "verified-token");
    expect(container.textContent).not.toContain("project-notes.txt");
  });

	it("opens an authorized preview from the file library", async () => {
		api.listFolderContents.mockResolvedValue(library([{ file: savedFile }]));
		api.getPersistentFileDownload.mockResolvedValue({
			file: savedFile,
			downloadTarget: { url: "https://download.invalid/file-1", expiresAt: "soon" },
			preview: { kind: "text", url: "https://preview.invalid/file-1", expiresAt: "soon" },
		});
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("Preview contents", { status: 200 })));
		const container = await renderLibrary();

		const previewButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Preview");
		await act(async () => { previewButton?.click(); await Promise.resolve(); });

		expect(api.getPersistentFileDownload).toHaveBeenCalledWith("file-1", "verified-token");
		expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Preview contents");
	});

  it("uploads a selected file into the persistent library", async () => {
	api.listFolderContents.mockResolvedValueOnce(library([])).mockResolvedValueOnce(library([{ file: savedFile }]));
    api.createPersistentUpload.mockResolvedValue({
      file: { ...savedFile, status: "PENDING" },
      uploadTarget: { url: "https://upload.invalid", method: "PUT", headers: {}, expiresAt: "2026-09-03T12:15:00Z" },
    });
    api.uploadResumable.mockImplementation((_file, _target, onProgress) => {
      onProgress(100);
      return { promise: Promise.resolve(), abort: vi.fn() };
    });
    api.completePersistentUpload.mockResolvedValue(savedFile);
    const container = await renderLibrary();
    const input = container.querySelector<HTMLInputElement>("#owned-files-input");
    Object.defineProperty(input, "files", {
      configurable: true,
      value: [new File(["hello"], "project-notes.txt", { type: "text/plain" })],
    });

    await act(async () => { input?.dispatchEvent(new Event("change", { bubbles: true })); });

	expect(api.createPersistentUpload).toHaveBeenCalledWith(expect.any(File), "verified-token", undefined);
    expect(api.completePersistentUpload).toHaveBeenCalledWith("file-1", "verified-token");
    expect(container.textContent).toContain("project-notes.txt");
  });

  it("accepts files dropped onto the persistent library", async () => {
	api.listFolderContents.mockResolvedValueOnce(library([])).mockResolvedValueOnce(library([{ file: savedFile }]));
    api.createPersistentUpload.mockResolvedValue({
      file: { ...savedFile, status: "PENDING" },
      uploadTarget: { url: "https://upload.invalid", method: "POST", headers: {}, expiresAt: "2026-09-03T12:15:00Z" },
    });
    api.uploadResumable.mockReturnValue({ promise: Promise.resolve(), abort: vi.fn() });
    api.completePersistentUpload.mockResolvedValue(savedFile);
    const container = await renderLibrary();
    const libraryContent = container.querySelector(".library-content");
    const droppedFile = new File(["hello"], "project-notes.txt", { type: "text/plain" });
    const dragEnter = new Event("dragenter", { bubbles: true });
    Object.defineProperty(dragEnter, "dataTransfer", { value: { types: ["Files"], files: [droppedFile] } });
    act(() => libraryContent?.dispatchEvent(dragEnter));
    expect(container.textContent).toContain("Drop files to add them");

    const drop = new Event("drop", { bubbles: true, cancelable: true });
    Object.defineProperty(drop, "dataTransfer", { value: { types: ["Files"], files: [droppedFile] } });
    await act(async () => { libraryContent?.dispatchEvent(drop); });

	expect(api.createPersistentUpload).toHaveBeenCalledWith(droppedFile, "verified-token", undefined);
    expect(container.textContent).toContain("project-notes.txt");
  });

  it("keeps the library bounded to ten rows and pages through additional files", async () => {
	const entries = Array.from({ length: 21 }, (_, index) => ({
		file: { ...savedFile, id: `file-${index + 1}`, originalName: `file-${index + 1}.txt` },
	}));
	api.listFolderContents
		.mockResolvedValueOnce({ ...library(entries.slice(0, 10)), nextCursor: "cursor-2" })
		.mockResolvedValueOnce({ ...library(entries.slice(10, 20)), nextCursor: "cursor-3" })
		.mockResolvedValueOnce(library(entries.slice(20)));
    const container = await renderLibrary();

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(10);
	expect(container.textContent).toContain("1–10");
    expect(container.textContent).toContain("file-1.txt");
    expect(container.textContent).not.toContain("file-11.txt");

    const nextButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Next");
	await act(async () => { nextButton?.click(); });

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(10);
	expect(container.textContent).toContain("11–20");
    expect(container.textContent).toContain("file-11.txt");
    expect(container.textContent).not.toContain("file-1.txt");

    const updatedNextButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Next");
	await act(async () => { updatedNextButton?.click(); });

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(1);
	expect(container.textContent).toContain("21–21");
	expect(container.textContent).toContain("Page 3");
  });

  it("creates, copies, and revokes a persistent file share link", async () => {
    const share = {
      id: "share-1",
      shortCode: "sharecode",
      fileId: "file-1",
      createdAt: "2026-09-03T12:02:00Z",
      expiresAt: "2026-09-10T12:02:00Z",
    };
	api.listFolderContents.mockResolvedValue(library([{ file: savedFile }]));
    api.createPersistentFileShare.mockResolvedValue({ share, sharePath: "/s/sharecode" });
    api.revokePersistentFileShare.mockResolvedValue(undefined);
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    const container = await renderLibrary();

    const shareButton = container.querySelector<HTMLButtonElement>(".row-action.share");
    act(() => shareButton?.click());
    const createButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent?.includes("Create link"));
    await act(async () => { createButton?.click(); });

    expect(api.createPersistentFileShare).toHaveBeenCalledWith("file-1", "7d", "verified-token");
    expect(container.textContent).toContain("Link active");
    const copyButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent?.includes("Copy"));
    await act(async () => { copyButton?.click(); });
    expect(writeText).toHaveBeenCalledWith("http://localhost:3000/s/sharecode");

    const revokeButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Revoke link");
    await act(async () => { revokeButton?.click(); });
    expect(api.revokePersistentFileShare).toHaveBeenCalledWith("file-1", "share-1", "verified-token");
    expect(container.textContent).not.toContain("Link active");
  });

  it("shows storage usage and supports search, shared filtering, and sorting", async () => {
    const shared = {
      id: "share-1", shortCode: "sharedcode", fileId: "file-2", createdAt: "2026-09-03T12:00:00Z",
    };
	const initial = library([
      { file: { ...savedFile, id: "file-1", originalName: "zebra.txt", sizeBytes: 5 } },
      { file: { ...savedFile, id: "file-2", originalName: "alpha.pdf", mimeType: "application/pdf", sizeBytes: 2048 }, share: shared, sharePath: "/s/sharedcode" },
	]);
	api.listFolderContents
		.mockResolvedValueOnce(initial)
		.mockResolvedValueOnce({ ...initial, files: [initial.files[1], initial.files[0]] })
		.mockResolvedValueOnce({ ...initial, files: [initial.files[1]] })
		.mockResolvedValueOnce({ ...initial, files: [] });
    const container = await renderLibrary();

    expect(container.textContent).toContain("2 files · 2.00 KB stored");
    const sort = container.querySelector<HTMLSelectElement>(".library-sort select");
    await act(async () => {
      if (sort) {
        sort.value = "name";
        sort.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    const names = Array.from(container.querySelectorAll(".owned-file-name strong")).map((node) => node.textContent);
    expect(names).toEqual(["alpha.pdf", "zebra.txt"]);

	const sharedButton = Array.from(container.querySelectorAll<HTMLButtonElement>(".library-filter button")).find((button) => button.textContent === "Shared");
	await act(async () => { sharedButton?.click(); });
    expect(container.textContent).toContain("alpha.pdf");
    expect(container.textContent).not.toContain("zebra.txt");

    const search = container.querySelector<HTMLInputElement>(".library-search input");
	await act(async () => {
	  if (search) setInputValue(search, "missing");
	  await new Promise((resolve) => setTimeout(resolve, 275));
	});
    expect(container.textContent).toContain("No files found.");
  });

	it("creates a folder and refreshes the active location", async () => {
		api.listFolderContents.mockResolvedValue(library([]));
		api.createFolder.mockResolvedValue({ id: "folder-1", ownerId: "user-1", name: "Designs", createdAt: "2026-09-04T12:00:00Z" });
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#new-folder-name")!;
		setInputValue(input, "Designs");
		const button = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("New folder"));
		await act(async () => { button?.click(); });
		expect(api.createFolder).toHaveBeenCalledWith("Designs", undefined, "verified-token");
		expect(api.listFolderContents).toHaveBeenCalledTimes(2);
	});

	it("opens a shared folder as a read-only viewer", async () => {
		const sharedFolder = {
			folder: { id: "folder-1", ownerId: "owner-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "VIEWER" as const,
			owner: { id: "owner-1", email: "owner@example.com", displayName: "Owner", createdAt: "2026-09-04T12:00:00Z" },
		};
		api.listFolderContents
			.mockResolvedValueOnce(library([]))
			.mockResolvedValueOnce({ ...library([]), folders: [sharedFolder] })
			.mockResolvedValueOnce({ ...library([{ file: { ...savedFile, ownerId: "owner-1" } }]), current: sharedFolder, breadcrumbs: [sharedFolder.folder] });
		const container = await renderLibrary();
		const sharedButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent === "Shared with me");
		await act(async () => { sharedButton?.click(); });
		const folderButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Launch"));
		await act(async () => { folderButton?.click(); });
		expect(container.textContent).toContain("Read-only · Shared by Owner");
		expect(container.textContent).toContain("project-notes.txt");
		expect(container.textContent).not.toContain("Delete");
		expect(window.location.search).toBe("?folder=folder-1&scope=shared");
	});

	it("restores a deeply nested owned folder from the workspace URL", async () => {
		const projectFolder = { id: "project-folder", ownerId: "user-1", name: "Project", createdAt: "2026-09-04T12:00:00Z" };
		const designsFolder = { id: "designs-folder", ownerId: "user-1", parentFolderId: "project-folder", name: "Designs", createdAt: "2026-09-04T12:01:00Z" };
		const finalFolder = {
			folder: { id: "final-folder", ownerId: "user-1", parentFolderId: "designs-folder", name: "Final", createdAt: "2026-09-04T12:02:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		window.history.replaceState(null, "", "/app?folder=final-folder");
		api.listFolderContents.mockResolvedValue({ ...library([]), current: finalFolder, breadcrumbs: [projectFolder, designsFolder, finalFolder.folder] });

		const container = await renderLibrary();

		expect(api.listFolderContents).toHaveBeenCalledWith("verified-token", "final-folder", "owned", { sort: "newest", limit: 10 });
		expect(container.querySelector("#library-title")?.textContent).toBe("Final");
		expect(container.querySelector(".folder-breadcrumbs")?.textContent).toContain("My files/ Project/ Designs/ Final");
		expect(window.location.search).toBe("?folder=final-folder");
	});

	it("refreshes an open folder after a realtime invalidation without clearing selection", async () => {
		const folder = {
			folder: { id: "folder-1", ownerId: "user-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		const updatedFile = { ...savedFile, id: "file-2", originalName: "new-notes.txt" };
		window.history.replaceState(null, "", "/app?folder=folder-1");
		api.listFolderContents
			.mockResolvedValueOnce({ ...library([{ file: savedFile }]), current: folder, breadcrumbs: [folder.folder] })
			.mockResolvedValueOnce({ ...library([{ file: savedFile }]), current: folder, breadcrumbs: [folder.folder] })
			.mockResolvedValueOnce({ ...library([{ file: savedFile }, { file: updatedFile }]), current: folder, breadcrumbs: [folder.folder] });
		let sendChange: (() => void) | undefined;
		api.streamFolderEvents.mockImplementation((_token, _folderID, signal: AbortSignal, _onOpen, onChange) => {
			sendChange = () => onChange({ folderId: "folder-1" });
			return new Promise<void>((resolve) => signal.addEventListener("abort", () => resolve(), { once: true }));
		});

		const container = await renderLibrary();
		await act(async () => {
			await vi.waitFor(() => expect(api.streamFolderEvents).toHaveBeenCalled());
		});
		expect(document.querySelector(".folder-update-toast")).toBeNull();
		const selection = container.querySelector<HTMLInputElement>('input[aria-label="Select project-notes.txt"]')!;
		act(() => selection.click());

		await act(async () => {
			sendChange?.();
			await vi.waitFor(() => expect(api.listFolderContents).toHaveBeenCalledTimes(2));
		});
		expect(document.querySelector(".folder-update-toast")).toBeNull();

		await act(async () => {
			sendChange?.();
			await vi.waitFor(() => expect(api.listFolderContents).toHaveBeenCalledTimes(3));
		});
		expect(container.textContent).toContain("new-notes.txt");
		expect(document.querySelector(".folder-update-toast strong")?.textContent).toBe("Folder updated");
		expect(document.querySelector(".folder-update-toast small")?.textContent).toBe("Latest changes are now visible.");

		expect(api.listFolderContents).toHaveBeenLastCalledWith("verified-token", "folder-1", "owned", {
			search: "", sort: "newest", filter: "all", limit: 10, cursor: "",
		});
		expect(container.querySelector<HTMLInputElement>('input[aria-label="Select project-notes.txt"]')?.checked).toBe(true);
		expect(container.textContent).toContain("1 selected");
	});

	it("updates the URL during folder navigation and responds to browser history", async () => {
		const folder = {
			folder: { id: "folder-1", ownerId: "user-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		api.listFolderContents
			.mockResolvedValueOnce({ ...library([]), folders: [folder] })
			.mockResolvedValueOnce({ ...library([]), current: folder, breadcrumbs: [folder.folder] })
			.mockResolvedValueOnce(library([]))
			.mockResolvedValueOnce({ ...library([]), current: folder, breadcrumbs: [folder.folder] });
		const container = await renderLibrary();

		const folderButton = container.querySelector<HTMLButtonElement>(".folder-card")!;
		await act(async () => { folderButton.click(); });
		expect(window.location.pathname).toBe("/app");
		expect(window.location.search).toBe("?folder=folder-1");

		const rootBreadcrumb = container.querySelector<HTMLButtonElement>(".folder-breadcrumbs button")!;
		await act(async () => { rootBreadcrumb.click(); });
		expect(window.location.search).toBe("");

		window.history.replaceState(null, "", "/app?folder=folder-1");
		await act(async () => {
			window.dispatchEvent(new PopStateEvent("popstate"));
			await Promise.resolve();
		});
		expect(api.listFolderContents).toHaveBeenLastCalledWith("verified-token", "folder-1", "owned", {
			search: "",
			sort: "newest",
			filter: "all",
			limit: 10,
			cursor: "",
		});
		expect(container.querySelector("#library-title")?.textContent).toBe("Launch");
	});

	it("opens and closes folder access management", async () => {
		const ownedFolder = {
			folder: { id: "folder-1", ownerId: "user-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		api.listFolderContents
			.mockResolvedValueOnce({ ...library([]), folders: [ownedFolder] })
			.mockResolvedValueOnce({ ...library([]), current: ownedFolder, breadcrumbs: [ownedFolder.folder] });
		api.listFolderMembers.mockResolvedValue(Array.from({ length: 12 }, (_, index) => ({
			user: { id: `member-${index}`, email: `member-${index}@example.com`, displayName: `Member ${index + 1}`, createdAt: "2026-09-04T12:00:00Z" },
			role: index % 2 === 0 ? "CONTRIBUTOR" : "VIEWER",
			createdAt: "2026-09-04T12:00:00Z",
		})));
		api.listFolderInvites.mockResolvedValue([]);
		const container = await renderLibrary();
		const folderButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Launch"));
		await act(async () => { folderButton?.click(); });
		const manageButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Manage access"));
		await act(async () => { manageButton?.click(); });
		expect(container.querySelector(".folder-sharing-panel")).not.toBeNull();
		expect(api.listFolderMembers).toHaveBeenCalledWith("folder-1", "verified-token");
		expect(container.textContent).toContain("Members (12)");
		expect(container.querySelectorAll(".folder-member-list .folder-member")).toHaveLength(12);
		const closeButton = container.querySelector<HTMLButtonElement>(".close-sharing-panel");
		act(() => closeButton?.click());
		expect(container.querySelector(".folder-sharing-panel")).toBeNull();
	});

	it("shows inherited members and manages their access from the source folder", async () => {
		const rootFolder = {
			folder: { id: "root-folder", ownerId: "user-1", name: "Project", createdAt: "2026-09-04T12:00:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		const childFolder = {
			folder: { id: "child-folder", ownerId: "user-1", parentFolderId: "root-folder", name: "Plans", createdAt: "2026-09-04T12:01:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		api.listFolderContents
			.mockResolvedValueOnce({ ...library([]), folders: [childFolder] })
			.mockResolvedValueOnce({ ...library([]), current: childFolder, breadcrumbs: [rootFolder.folder, childFolder.folder] })
			.mockResolvedValueOnce({ ...library([]), current: rootFolder, breadcrumbs: [rootFolder.folder], folders: [childFolder] });
		api.listFolderMembers.mockResolvedValue([{
			user: { id: "member-1", email: "member@example.com", displayName: "Collaborator", createdAt: "2026-09-04T12:00:00Z" },
			role: "CONTRIBUTOR",
			createdAt: "2026-09-04T12:00:00Z",
			inherited: true,
			sourceFolderId: "root-folder",
			sourceFolderName: "Project",
		}]);
		api.listFolderInvites.mockResolvedValue([]);

		const container = await renderLibrary();
		const childButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Plans"));
		await act(async () => { childButton?.click(); });
		const manageButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Manage access"));
		await act(async () => { manageButton?.click(); });

		expect(container.textContent).toContain("Members (1)");
		expect(container.textContent).toContain("Inherited from Project");
		expect(container.textContent).not.toContain("Remove");
		const sourceButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent === "Manage source");
		expect(sourceButton?.getAttribute("aria-label")).toContain("access in Project");
		await act(async () => { sourceButton?.click(); });

		expect(api.removeFolderMember).not.toHaveBeenCalled();
		expect(api.listFolderContents).toHaveBeenLastCalledWith("verified-token", "root-folder", "owned", {
			search: "",
			sort: "newest",
			filter: "all",
			limit: 10,
			cursor: "",
		});
		expect(container.textContent).toContain("Project");
		expect(container.querySelector(".folder-sharing-panel")).toBeNull();
	});

	it("creates a role-aware folder invite link and copies it", async () => {
		const ownedFolder = {
			folder: { id: "folder-1", ownerId: "user-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		const invite = { id: "invite-1", folderId: "folder-1", shortCode: "joinme", role: "CONTRIBUTOR" as const, createdAt: "2026-09-04T12:00:00Z", expiresAt: "2026-09-11T12:00:00Z" };
		api.listFolderContents
			.mockResolvedValueOnce({ ...library([]), folders: [ownedFolder] })
			.mockResolvedValueOnce({ ...library([]), current: ownedFolder, breadcrumbs: [ownedFolder.folder] });
		api.listFolderMembers.mockResolvedValue([]);
		api.listFolderInvites.mockResolvedValue([]);
		api.createFolderInvite.mockResolvedValue({ invite, invitePath: "/join/joinme" });
		const writeText = vi.fn().mockResolvedValue(undefined);
		Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
		const container = await renderLibrary();
		const folderButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Launch"));
		await act(async () => { folderButton?.click(); });
		const manageButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Manage access"));
		await act(async () => { manageButton?.click(); });
		const role = container.querySelector<HTMLSelectElement>('select[aria-label="Invite role"]')!;
		act(() => { role.value = "CONTRIBUTOR"; role.dispatchEvent(new Event("change", { bubbles: true })); });
		const createLink = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Create link"));
		await act(async () => { createLink?.click(); });
		expect(api.createFolderInvite).toHaveBeenCalledWith("folder-1", "CONTRIBUTOR", "7d", "verified-token");
		expect(writeText).toHaveBeenCalledWith("http://localhost:3000/join/joinme");
		expect(container.textContent).toContain("Contributor invite");
	});

	it("accepts a folder invite from the workspace URL and opens the shared folder", async () => {
		const sharedFolder = {
			folder: { id: "folder-1", ownerId: "owner-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "VIEWER" as const,
			owner: { id: "owner-1", email: "owner@example.com", displayName: "Owner", createdAt: "2026-09-04T12:00:00Z" },
		};
		window.history.replaceState({}, "", "/app?folderInvite=joinme");
		api.acceptFolderInvite.mockResolvedValue(sharedFolder);
		api.listFolderContents.mockResolvedValue({ ...library([]), current: sharedFolder, breadcrumbs: [sharedFolder.folder] });
		const container = await renderLibrary();
		expect(api.acceptFolderInvite).toHaveBeenCalledWith("joinme", "verified-token");
		expect(container.textContent).toContain("Read-only · Shared by Owner");
		expect(window.location.search).toBe("?folder=folder-1&scope=shared");
	});

	it("opens the folder handed off by the public invitation page", async () => {
		const sharedFolder = {
			folder: { id: "folder-1", ownerId: "owner-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "VIEWER" as const,
			owner: { id: "owner-1", email: "owner@example.com", displayName: "Owner", createdAt: "2026-09-04T12:00:00Z" },
		};
		window.history.replaceState({}, "", "/app?openFolder=folder-1&scope=shared");
		api.listFolderContents.mockResolvedValue({ ...library([]), current: sharedFolder, breadcrumbs: [sharedFolder.folder] });
		const container = await renderLibrary();
		expect(api.listFolderContents).toHaveBeenCalledWith("verified-token", "folder-1", "shared", { sort: "newest", limit: 10 });
		expect(container.textContent).toContain("Read-only · Shared by Owner");
		expect(window.location.search).toBe("?folder=folder-1&scope=shared");
	});

	it("lets contributors upload and manage only their own shared-folder files", async () => {
		const sharedFolder = {
			folder: { id: "folder-1", ownerId: "owner-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "CONTRIBUTOR" as const,
			owner: { id: "owner-1", email: "owner@example.com", displayName: "Owner", createdAt: "2026-09-04T12:00:00Z" },
		};
		const ownFile = { ...savedFile, id: "mine", originalName: "mine.txt" };
		const ownerFile = { ...savedFile, id: "theirs", ownerId: "owner-1", originalName: "theirs.txt" };
		api.listFolderContents
			.mockResolvedValueOnce(library([]))
			.mockResolvedValueOnce({ ...library([]), folders: [sharedFolder] })
			.mockResolvedValueOnce({ ...library([{ file: ownFile, uploaderName: "Me" }, { file: ownerFile, uploaderName: "Owner" }]), current: sharedFolder, breadcrumbs: [sharedFolder.folder] })
			.mockResolvedValueOnce({ ...library([{ file: ownFile, uploaderName: "Me" }, { file: ownerFile, uploaderName: "Owner" }]), current: sharedFolder, breadcrumbs: [sharedFolder.folder] });
		api.createPersistentUpload.mockResolvedValue({ file: { ...ownFile, id: "pending", status: "PENDING" }, uploadTarget: { url: "https://upload.invalid", method: "PUT", headers: {}, expiresAt: "2026-09-04T12:15:00Z" } });
		api.uploadResumable.mockReturnValue({ promise: Promise.resolve(), abort: vi.fn() });
		api.completePersistentUpload.mockResolvedValue(ownFile);
		const container = await renderLibrary();
		const sharedButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent === "Shared with me");
		await act(async () => { sharedButton?.click(); });
		const folderButton = Array.from(container.querySelectorAll("button")).find((item) => item.textContent?.includes("Launch"));
		await act(async () => { folderButton?.click(); });
		expect(container.textContent).toContain("Contributor · You can upload and manage your own files");
		expect(container.textContent).toContain("Uploaded by you");
		expect(container.textContent).toContain("Uploaded by Owner");
		const rows = Array.from(container.querySelectorAll<HTMLElement>(".owned-file-row"));
		expect(rows.find((row) => row.textContent?.includes("mine.txt"))?.textContent).toContain("Delete");
		expect(rows.find((row) => row.textContent?.includes("theirs.txt"))?.textContent).not.toContain("Delete");
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["mine"], "mine.txt")] });
		await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
		expect(api.createPersistentUpload).toHaveBeenCalledWith(expect.any(File), "verified-token", "folder-1");
	});

	it("continues the upload queue when one file fails", async () => {
		api.listFolderContents.mockResolvedValueOnce(library([])).mockResolvedValueOnce(library([{ file: { ...savedFile, id: "file-2", originalName: "second.txt" } }]));
		api.createPersistentUpload
			.mockResolvedValueOnce({ file: { ...savedFile, id: "pending-1", status: "PENDING" }, uploadTarget: { url: "https://upload.invalid/1", method: "POST", headers: {}, expiresAt: "2026-09-04T12:15:00Z" } })
			.mockResolvedValueOnce({ file: { ...savedFile, id: "pending-2", status: "PENDING" }, uploadTarget: { url: "https://upload.invalid/2", method: "POST", headers: {}, expiresAt: "2026-09-04T12:15:00Z" } });
		api.uploadResumable
			.mockImplementationOnce(() => { throw new Error("network down"); })
			.mockReturnValueOnce({ promise: Promise.resolve(), abort: vi.fn() });
		api.completePersistentUpload.mockResolvedValue({ ...savedFile, id: "file-2", originalName: "second.txt" });
		api.deletePersistentFile.mockResolvedValue(undefined);
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input");
		Object.defineProperty(input, "files", {
			configurable: true,
			value: [new File(["one"], "first.txt"), new File(["two"], "second.txt")],
		});

		await act(async () => { input?.dispatchEvent(new Event("change", { bubbles: true })); });

		expect(api.completePersistentUpload).toHaveBeenCalledWith("pending-2", "verified-token");
		expect(api.deletePersistentFile).toHaveBeenCalledWith("pending-1", "verified-token");
		expect(container.textContent).toContain("Retry");
		expect(container.textContent).toContain("second.txt");
	});
});
