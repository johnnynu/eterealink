import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PersistentFileLibrary } from "./persistent-file-library";
import type { FileRecord } from "@/lib/types";
import type { UploadRecoveryRecord } from "@/lib/upload-recovery";
import { PROFILE_UPDATE_FINISHED_EVENT, PROFILE_UPDATE_STARTED_EVENT } from "@/lib/events";

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
	resumeResumableUpload: vi.fn(),
	streamFolderEvents: vi.fn(),
  uploadResumable: vi.fn(),
	updateFolder: vi.fn(),
}));
const getIDToken = vi.hoisted(() => vi.fn(async () => "verified-token"));
const signedInUser = vi.hoisted(() => ({ id: "user-1", email: "me@example.com", displayName: "Me", createdAt: "2026-09-03T12:00:00Z" }));
const recovery = vi.hoisted(() => ({
	deleteUploadRecovery: vi.fn(async () => true),
	listUploadRecoveries: vi.fn(async (): Promise<UploadRecoveryRecord[]> => []),
	matchesRecoveryFile: vi.fn(() => true),
	saveUploadRecovery: vi.fn(async () => true),
	updateUploadRecovery: vi.fn(async (record, update) => ({ ...record, ...update, updatedAt: Date.now() })),
}));

vi.mock("@/components/auth-context", () => ({ useAuth: () => ({ getIDToken, user: signedInUser }) }));
vi.mock("@/lib/api", () => ({
  APIError: class APIError extends Error {},
  ...api,
}));
vi.mock("@/lib/upload-recovery", () => recovery);

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

function library(files: Array<{ file: FileRecord; uploaderName?: string; share?: object; sharePath?: string }>, quotaBytes?: number) {
  return {
    files,
	folders: [],
	breadcrumbs: [],
	totalCount: files.length,
    summary: {
      fileCount: files.length,
      totalBytes: files.reduce((total, entry) => total + entry.file.sizeBytes, 0),
	  quotaBytes,
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

function deferred<T>() {
	let resolve!: (value: T | PromiseLike<T>) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((nextResolve, nextReject) => {
		resolve = nextResolve;
		reject = nextReject;
	});
	return { promise, resolve, reject };
}

function recoveryRecord(overrides: Partial<UploadRecoveryRecord> = {}): UploadRecoveryRecord {
	return {
		fileId: "pending-1",
		userId: "user-1",
		sessionUrl: "https://upload.invalid/private",
		fileName: "same.txt",
		fileSize: 5,
		mimeType: "text/plain",
		lastModified: 100,
		confirmedOffset: 2,
		completionPending: false,
		createdAt: 1,
		updatedAt: 1,
		...overrides,
	};
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

	it("does not apply the former 5 GiB authenticated per-file cap", async () => {
		api.listFolderContents.mockResolvedValue(library([], 10 * 1024 ** 3));
		const large = new File(["x"], "archive.bin", { type: "application/octet-stream", lastModified: 100 });
		Object.defineProperty(large, "size", { configurable: true, value: 6 * 1024 ** 3 });
		api.createPersistentUpload.mockResolvedValue({
			file: { ...savedFile, id: "large-file", originalName: large.name, sizeBytes: large.size, status: "PENDING" },
			uploadTarget: { url: "https://upload.invalid", method: "POST", headers: {}, expiresAt: "soon" },
		});
		api.uploadResumable.mockReturnValue({ promise: Promise.resolve(), abort: vi.fn() });
		api.completePersistentUpload.mockResolvedValue({ ...savedFile, id: "large-file" });
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [large] });
		await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
		expect(api.createPersistentUpload).toHaveBeenCalledWith(large, "verified-token", undefined);
	});

	it("rejects a selected batch that exceeds remaining account capacity", async () => {
		const state = library([{ file: { ...savedFile, sizeBytes: 8 } }], 10);
		api.listFolderContents.mockResolvedValue(state);
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["abc"], "more.txt")] });
		await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
		expect(api.createPersistentUpload).not.toHaveBeenCalled();
		expect(container.textContent).toContain("2 B remaining");
	});

	it("stores resumable recovery after initiation and removes it only after completion", async () => {
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.createPersistentUpload.mockResolvedValue({
			file: { ...savedFile, status: "PENDING" },
			uploadTarget: { url: "https://upload.invalid/start", method: "POST", headers: {}, expiresAt: "soon" },
		});
		api.uploadResumable.mockImplementation((_file, _target, _progress, callbacks) => ({
			promise: (async () => {
				await callbacks.onSession("https://upload.invalid/private-session");
				await callbacks.onConfirmedOffset(5);
			})(),
			abort: vi.fn(),
		}));
		api.completePersistentUpload.mockResolvedValue(savedFile);
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["hello"], "project-notes.txt", { type: "text/plain", lastModified: 100 })] });
		await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
		expect(recovery.saveUploadRecovery).toHaveBeenCalledWith(expect.objectContaining({
			fileId: "file-1", userId: "user-1", sessionUrl: "https://upload.invalid/private-session",
		}));
		expect(recovery.updateUploadRecovery).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({ confirmedOffset: 5 }));
		expect(recovery.deleteUploadRecovery).toHaveBeenCalledWith("file-1");
		expect(container.textContent).not.toContain("private-session");
	});

	it("shows one row while an active upload also has browser recovery data", async () => {
		const uploadGate = deferred<void>();
		recovery.listUploadRecoveries.mockResolvedValueOnce([recoveryRecord({ fileId: "other-pending", fileName: "other.txt" })]);
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.createPersistentUpload.mockResolvedValue({
			file: { ...savedFile, status: "PENDING" },
			uploadTarget: { url: "https://upload.invalid/start", method: "POST", headers: {}, expiresAt: "soon" },
		});
		api.uploadResumable.mockImplementation((_file, _target, _progress, callbacks) => ({
			promise: (async () => {
				await callbacks.onSession?.("https://upload.invalid/private-session");
				await callbacks.onConfirmedOffset?.(2);
				await uploadGate.promise;
			})(),
			abort: vi.fn(),
		}));
		api.completePersistentUpload.mockResolvedValue(savedFile);
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["hello"], "project-notes.txt", { type: "text/plain", lastModified: 100 })] });

		act(() => input.dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(recovery.saveUploadRecovery).toHaveBeenCalled());

		expect(container.querySelectorAll(".upload-queue-item")).toHaveLength(1);
		expect(container.querySelectorAll(".upload-recovery-item")).toHaveLength(1);
		expect(Array.from(container.querySelectorAll(".upload-queue-item strong, .upload-recovery-item strong"))
			.filter((element) => element.textContent === "project-notes.txt")).toHaveLength(1);
		expect(container.querySelector<HTMLInputElement>(".upload-recovery-item input")?.disabled).toBe(true);

		uploadGate.resolve();
		await vi.waitFor(() => expect(container.textContent).toContain("Complete"));
	});

	it("updates confirmed bytes and percentage live during an initial upload", async () => {
		const uploadGate = deferred<void>();
		let callbacks: Parameters<typeof api.uploadResumable>[3] | undefined;
		const file = new File(["0123456789"], "ten.txt", { type: "text/plain", lastModified: 100 });
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.createPersistentUpload.mockResolvedValue({
			file: { ...savedFile, id: "pending-10", originalName: file.name, sizeBytes: file.size, status: "PENDING" },
			uploadTarget: { url: "https://upload.invalid/start", method: "POST", headers: {}, expiresAt: "soon" },
		});
		api.uploadResumable.mockImplementation((_file, _target, _progress, nextCallbacks) => {
			callbacks = nextCallbacks;
			return { promise: uploadGate.promise, abort: vi.fn() };
		});
		api.completePersistentUpload.mockResolvedValue(savedFile);
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [file] });
		act(() => input.dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(callbacks).toBeDefined());

		await act(async () => {
			await callbacks?.onSession?.("https://upload.invalid/private-session");
			await callbacks?.onConfirmedOffset?.(4);
		});

		expect(container.querySelector(".upload-queue-item small")?.textContent).toContain("4 B of 10 B · 40%");
		expect(recovery.updateUploadRecovery).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({ confirmedOffset: 4 }));
		uploadGate.resolve();
		await vi.waitFor(() => expect(container.textContent).toContain("Complete"));
	});

	it("rejects a mismatched recovery reselection", async () => {
		recovery.listUploadRecoveries.mockResolvedValueOnce([{
			fileId: "pending-1", userId: "user-1", sessionUrl: "https://upload.invalid/private",
			fileName: "same.txt", fileSize: 5, mimeType: "text/plain", lastModified: 100,
			confirmedOffset: 2, completionPending: false, createdAt: 1, updatedAt: 1,
		}]);
		recovery.matchesRecoveryFile.mockReturnValueOnce(false);
		api.listFolderContents.mockResolvedValue(library([], 100));
		const container = await renderLibrary();
		await act(async () => { await Promise.resolve(); });
		const input = container.querySelector<HTMLInputElement>(".upload-recovery-item input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["other"], "same.txt")] });
		await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
		expect(api.resumeResumableUpload).not.toHaveBeenCalled();
		expect(container.textContent).toContain("name, size, modified date, and type must match");
	});

	it("moves a reselected recovery through checking, uploading, finalizing, and complete", async () => {
		const uploadGate = deferred<void>();
		const completionGate = deferred<FileRecord>();
		let confirmed: ((offset: number) => void | Promise<void>) | undefined;
		recovery.listUploadRecoveries.mockResolvedValueOnce([recoveryRecord()]);
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.resumeResumableUpload.mockImplementation((_file, _session, _progress, onConfirmedOffset) => {
			confirmed = onConfirmedOffset;
			return { promise: uploadGate.promise, abort: vi.fn() };
		});
		api.completePersistentUpload.mockReturnValue(completionGate.promise);
		const container = await renderLibrary();
		await act(async () => { await Promise.resolve(); });

		expect(container.textContent).toContain("Upload paused at 2 B of 5 B — select the original file to continue.");
		const input = container.querySelector<HTMLInputElement>(".upload-recovery-item input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["hello"], "same.txt", { type: "text/plain", lastModified: 100 })] });
		act(() => input.dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(api.resumeResumableUpload).toHaveBeenCalledTimes(1));

		expect(container.querySelectorAll(".upload-recovery-item")).toHaveLength(0);
		expect(container.querySelectorAll(".upload-queue-item")).toHaveLength(1);
		expect(container.textContent).toContain("Checking server progress…");

		await act(async () => { await confirmed?.(3); });
		expect(container.textContent).toContain("Uploading · 3 B of 5 B · 60%");
		expect(Array.from(container.querySelectorAll("button")).some((button) => button.textContent === "Pause")).toBe(true);
		expect(recovery.updateUploadRecovery).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({ confirmedOffset: 3 }));

		uploadGate.resolve();
		await vi.waitFor(() => expect(container.textContent).toContain("Finalizing upload…"));
		completionGate.resolve(savedFile);
		await vi.waitFor(() => expect(container.textContent).toContain("Complete"));
		expect(recovery.deleteUploadRecovery).toHaveBeenCalledWith("pending-1");
	});

	it("prevents concurrent recovery and new-upload requests", async () => {
		const uploadGate = deferred<void>();
		const first = recoveryRecord();
		const second = recoveryRecord({ fileId: "pending-2", fileName: "other.txt", sessionUrl: "https://upload.invalid/other" });
		recovery.listUploadRecoveries.mockResolvedValueOnce([first, second]);
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.resumeResumableUpload.mockImplementation((_file, _session, _progress, onConfirmedOffset) => ({
			promise: (async () => {
				await onConfirmedOffset?.(3);
				await uploadGate.promise;
			})(),
			abort: vi.fn(),
		}));
		api.completePersistentUpload.mockResolvedValue(savedFile);
		const container = await renderLibrary();
		await act(async () => { await Promise.resolve(); });
		const inputs = Array.from(container.querySelectorAll<HTMLInputElement>(".upload-recovery-item input"));
		Object.defineProperty(inputs[0], "files", { configurable: true, value: [new File(["hello"], "same.txt", { type: "text/plain", lastModified: 100 })] });
		act(() => inputs[0].dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(api.resumeResumableUpload).toHaveBeenCalledTimes(1));

		const remainingInput = container.querySelector<HTMLInputElement>(".upload-recovery-item input")!;
		expect(remainingInput.disabled).toBe(true);
		const mainInput = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		expect(mainInput.disabled).toBe(true);
		Object.defineProperty(remainingInput, "files", { configurable: true, value: [new File(["hello"], "other.txt", { type: "text/plain", lastModified: 100 })] });
		remainingInput.dispatchEvent(new Event("change", { bubbles: true }));
		Object.defineProperty(mainInput, "files", { configurable: true, value: [new File(["new"], "new.txt")] });
		mainInput.dispatchEvent(new Event("change", { bubbles: true }));
		expect(api.resumeResumableUpload).toHaveBeenCalledTimes(1);
		expect(api.createPersistentUpload).not.toHaveBeenCalled();

		uploadGate.resolve();
		await vi.waitFor(() => expect(container.textContent).toContain("Complete"));
	});

	it("pauses a resumed upload and keeps its confirmed progress", async () => {
		const uploadGate = deferred<void>();
		const abort = vi.fn(() => uploadGate.reject(new DOMException("Paused", "AbortError")));
		recovery.listUploadRecoveries.mockResolvedValueOnce([recoveryRecord()]);
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.resumeResumableUpload.mockImplementation((_file, _session, _progress, onConfirmedOffset) => ({
			promise: (async () => {
				await onConfirmedOffset?.(3);
				await uploadGate.promise;
			})(),
			abort,
		}));
		const container = await renderLibrary();
		await act(async () => { await Promise.resolve(); });
		const input = container.querySelector<HTMLInputElement>(".upload-recovery-item input")!;
		Object.defineProperty(input, "files", { configurable: true, value: [new File(["hello"], "same.txt", { type: "text/plain", lastModified: 100 })] });
		act(() => input.dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(container.textContent).toContain("Uploading · 3 B of 5 B · 60%"));

		const pause = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Pause")!;
		act(() => pause.click());

		await vi.waitFor(() => expect(container.textContent).toContain("Paused · 3 B of 5 B · 60%"));
		expect(abort).toHaveBeenCalledTimes(1);
		expect(container.textContent).toContain("Resume");
	});

	it("shows preparing, uploading, finalizing, complete, and failed states", async () => {
		const createGate = deferred<Awaited<ReturnType<typeof api.createPersistentUpload>>>();
		const uploadGate = deferred<void>();
		const completionGate = deferred<FileRecord>();
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.createPersistentUpload.mockReturnValue(createGate.promise);
		api.uploadResumable.mockReturnValue({ promise: uploadGate.promise, abort: vi.fn() });
		api.completePersistentUpload.mockReturnValue(completionGate.promise);
		const container = await renderLibrary();
		const input = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		const file = new File(["hello"], "states.txt", { type: "text/plain" });
		Object.defineProperty(input, "files", { configurable: true, value: [file] });
		act(() => input.dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(container.textContent).toContain("Preparing…"));

		createGate.resolve({
			file: { ...savedFile, id: "states", originalName: file.name, status: "PENDING" },
			uploadTarget: { url: "https://upload.invalid/start", method: "POST", headers: {}, expiresAt: "soon" },
		});
		await vi.waitFor(() => expect(container.textContent).toContain("Uploading · 0 B of 5 B · 0%"));
		uploadGate.resolve();
		await vi.waitFor(() => expect(container.textContent).toContain("Finalizing upload…"));
		completionGate.resolve(savedFile);
		await vi.waitFor(() => expect(container.textContent).toContain("Complete"));

		api.createPersistentUpload.mockResolvedValue({
			file: { ...savedFile, id: "failed", originalName: "failed.txt", status: "PENDING" },
			uploadTarget: { url: "https://upload.invalid/fail", method: "POST", headers: {}, expiresAt: "soon" },
		});
		api.uploadResumable.mockReturnValue({ promise: Promise.reject(new Error("network down")), abort: vi.fn() });
		const refreshedInput = container.querySelector<HTMLInputElement>("#owned-files-input")!;
		Object.defineProperty(refreshedInput, "files", { configurable: true, value: [new File(["bad"], "failed.txt")] });
		act(() => refreshedInput.dispatchEvent(new Event("change", { bubbles: true })));
		await vi.waitFor(() => expect(container.textContent).toContain("Failed · failed.txt could not be uploaded."));
	});

	it("preserves completion-only recovery after failure and allows explicit discard", async () => {
		recovery.listUploadRecoveries.mockResolvedValueOnce([{
			fileId: "pending-1", userId: "user-1", sessionUrl: "https://upload.invalid/private",
			fileName: "same.txt", fileSize: 5, mimeType: "text/plain", lastModified: 100,
			confirmedOffset: 5, completionPending: true, createdAt: 1, updatedAt: 1,
		}]);
		api.listFolderContents.mockResolvedValue(library([], 100));
		api.completePersistentUpload.mockRejectedValueOnce(new Error("temporarily unavailable"));
		const container = await renderLibrary();
		await act(async () => { await Promise.resolve(); });
		const retry = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Retry completion")!;
		await act(async () => { retry.click(); });
		expect(recovery.deleteUploadRecovery).not.toHaveBeenCalled();
		expect(container.textContent).toContain("Failed · The upload is stored, but Eterealink could not finish it yet. Retry completion.");
		const discard = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Forget recovery on this browser")!;
		await act(async () => { discard.click(); });
		expect(recovery.deleteUploadRecovery).toHaveBeenCalledWith("pending-1");
		expect(Array.from(container.querySelectorAll("button")).some((button) => button.textContent === "Retry completion")).toBe(false);
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
		.mockResolvedValueOnce({ ...library(entries), files: entries.slice(0, 10), nextCursor: "cursor-2" })
		.mockResolvedValueOnce({ ...library(entries), files: entries.slice(10, 20), nextCursor: "cursor-3" })
		.mockResolvedValueOnce({ ...library(entries), files: entries.slice(20) });
    const container = await renderLibrary();

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(10);
	expect(container.textContent).toContain("1–10 of 21");
	expect(container.textContent).toContain("Page 1 of 3");
    expect(container.textContent).toContain("file-1.txt");
    expect(container.textContent).not.toContain("file-11.txt");

    const nextButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Next");
	await act(async () => { nextButton?.click(); });

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(10);
	expect(container.textContent).toContain("11–20 of 21");
	expect(container.textContent).toContain("Page 2 of 3");
    expect(container.textContent).toContain("file-11.txt");
    expect(container.textContent).not.toContain("file-1.txt");

    const updatedNextButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Next");
	await act(async () => { updatedNextButton?.click(); });

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(1);
	expect(container.textContent).toContain("21–21 of 21");
	expect(container.textContent).toContain("Page 3 of 3");
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

	it("shows remaining and total account capacity", async () => {
		api.listFolderContents.mockResolvedValue(library([{ file: { ...savedFile, sizeBytes: 25 } }], 100));
		const container = await renderLibrary();

		expect(container.textContent).toContain("75 B remaining of 100 B total");
		expect(container.querySelector(".storage-capacity")?.getAttribute("title")).toBe("25 B of 100 B used");
	});

	it("shows folder usage separately from account capacity", async () => {
		const folder = {
			folder: { id: "folder-1", ownerId: "user-1", name: "Team assets", createdAt: "2026-09-04T12:00:00Z" },
			role: "OWNER" as const,
			owner: signedInUser,
		};
		window.history.replaceState(null, "", "/app?folder=folder-1");
		api.listFolderContents.mockResolvedValue({
			...library([]),
			current: folder,
			breadcrumbs: [folder.folder],
			summary: { fileCount: 4, totalBytes: 384 * 1024 ** 2, accountTotalBytes: 25, quotaBytes: 100 },
		});

		const container = await renderLibrary();

		expect(container.textContent).toContain("4 files · 384.0 MB in this folder");
		expect(container.textContent).toContain("Your account: 75 B remaining of 100 B total");
		expect(container.querySelector(".storage-capacity")?.getAttribute("title")).toBe("25 B of 100 B used");
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

	it("silently refreshes an open folder after the current user updates their profile", async () => {
		const folder = {
			folder: { id: "folder-1", ownerId: "owner-1", name: "Launch", createdAt: "2026-09-04T12:00:00Z" },
			role: "VIEWER" as const,
			owner: { id: "owner-1", email: "owner@example.com", displayName: "Owner", createdAt: "2026-09-04T12:00:00Z" },
		};
		const renamedFolder = { ...folder, owner: { ...folder.owner, displayName: "Owner Alias" } };
		window.history.replaceState(null, "", "/app?folder=folder-1&scope=shared");
		api.listFolderContents
			.mockResolvedValueOnce({ ...library([]), current: folder, breadcrumbs: [folder.folder] })
			.mockResolvedValueOnce({ ...library([]), current: renamedFolder, breadcrumbs: [folder.folder] });
		let sendChange: (() => void) | undefined;
		api.streamFolderEvents.mockImplementation((_token, _folderID, signal: AbortSignal, _onOpen, onChange) => {
			sendChange = () => onChange({ folderId: "folder-1" });
			return new Promise<void>((resolve) => signal.addEventListener("abort", () => resolve(), { once: true }));
		});

		const container = await renderLibrary();
		await act(async () => {
			await vi.waitFor(() => expect(api.streamFolderEvents).toHaveBeenCalled());
			window.dispatchEvent(new Event(PROFILE_UPDATE_STARTED_EVENT));
			sendChange?.();
			await vi.waitFor(() => expect(api.listFolderContents).toHaveBeenCalledTimes(2));
		});

		expect(container.textContent).toContain("Shared by Owner Alias");
		expect(document.querySelector(".folder-update-toast")).toBeNull();
		window.dispatchEvent(new Event(PROFILE_UPDATE_FINISHED_EVENT));
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
