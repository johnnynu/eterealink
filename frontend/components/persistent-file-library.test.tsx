import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PersistentFileLibrary } from "./persistent-file-library";
import type { FileRecord } from "@/lib/types";

const api = vi.hoisted(() => ({
  completePersistentUpload: vi.fn(),
  createPersistentFileShare: vi.fn(),
  createPersistentUpload: vi.fn(),
  deletePersistentFile: vi.fn(),
  getPersistentFileDownload: vi.fn(),
  listPersistentFiles: vi.fn(),
  revokePersistentFileShare: vi.fn(),
  uploadResumable: vi.fn(),
}));
const getIDToken = vi.hoisted(() => vi.fn(async () => "verified-token"));

vi.mock("@/components/auth-context", () => ({ useAuth: () => ({ getIDToken }) }));
vi.mock("@/lib/api", () => ({
  APIError: class APIError extends Error {},
  ...api,
}));

const savedFile: FileRecord = {
  id: "file-1",
  originalName: "project-notes.txt",
  mimeType: "text/plain",
  sizeBytes: 5,
  status: "READY",
  createdAt: "2026-09-03T12:00:00Z",
  completedAt: "2026-09-03T12:01:00Z",
};
const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

function library(files: Array<{ file: FileRecord; share?: object; sharePath?: string }>) {
  return {
    files,
    summary: {
      fileCount: files.length,
      totalBytes: files.reduce((total, entry) => total + entry.file.sizeBytes, 0),
    },
  };
}

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
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
    api.listPersistentFiles.mockResolvedValue(library([{ file: savedFile }]));
    api.deletePersistentFile.mockResolvedValue(undefined);
    const container = await renderLibrary();

    expect(container.textContent).toContain("project-notes.txt");
    const deleteButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Delete");
    act(() => deleteButton?.click());
    expect(api.deletePersistentFile).not.toHaveBeenCalled();

    const confirmButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Delete");
    await act(async () => { confirmButton?.click(); });
    expect(api.deletePersistentFile).toHaveBeenCalledWith("file-1", "verified-token");
    expect(container.textContent).not.toContain("project-notes.txt");
  });

  it("uploads a selected file into the persistent library", async () => {
    api.listPersistentFiles.mockResolvedValue(library([]));
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

    expect(api.createPersistentUpload).toHaveBeenCalledWith(expect.any(File), "verified-token");
    expect(api.completePersistentUpload).toHaveBeenCalledWith("file-1", "verified-token");
    expect(container.textContent).toContain("project-notes.txt");
  });

  it("accepts files dropped onto the persistent library", async () => {
    api.listPersistentFiles.mockResolvedValue(library([]));
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

    expect(api.createPersistentUpload).toHaveBeenCalledWith(droppedFile, "verified-token");
    expect(container.textContent).toContain("project-notes.txt");
  });

  it("keeps the library bounded to ten rows and pages through additional files", async () => {
    api.listPersistentFiles.mockResolvedValue(library(Array.from({ length: 21 }, (_, index) => ({
      file: { ...savedFile, id: `file-${index + 1}`, originalName: `file-${index + 1}.txt` },
    }))));
    const container = await renderLibrary();

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(10);
    expect(container.textContent).toContain("1–10 of 21");
    expect(container.textContent).toContain("file-1.txt");
    expect(container.textContent).not.toContain("file-11.txt");

    const nextButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Next");
    act(() => nextButton?.click());

    expect(container.querySelectorAll(".owned-file-row")).toHaveLength(10);
    expect(container.textContent).toContain("11–20 of 21");
    expect(container.textContent).toContain("file-11.txt");
    expect(container.textContent).not.toContain("file-1.txt");

    const updatedNextButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "Next");
    act(() => updatedNextButton?.click());

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
    api.listPersistentFiles.mockResolvedValue(library([{ file: savedFile }]));
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
    api.listPersistentFiles.mockResolvedValue(library([
      { file: { ...savedFile, id: "file-1", originalName: "zebra.txt", sizeBytes: 5 } },
      { file: { ...savedFile, id: "file-2", originalName: "alpha.pdf", mimeType: "application/pdf", sizeBytes: 2048 }, share: shared, sharePath: "/s/sharedcode" },
    ]));
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
    act(() => sharedButton?.click());
    expect(container.textContent).toContain("alpha.pdf");
    expect(container.textContent).not.toContain("zebra.txt");

    const search = container.querySelector<HTMLInputElement>(".library-search input");
    await act(async () => {
      if (search) setInputValue(search, "missing");
    });
    expect(container.textContent).toContain("No files found.");
  });
});
