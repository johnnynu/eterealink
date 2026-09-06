import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  deleteUploadRecovery,
  listUploadRecoveries,
  matchesRecoveryFile,
  saveUploadRecovery,
  updateUploadRecovery,
  type UploadRecoveryRecord,
} from "./upload-recovery";

function request<T>(operation: () => T) {
  const result = {} as IDBRequest<T>;
  queueMicrotask(() => {
    try {
      Object.defineProperty(result, "result", { configurable: true, value: operation() });
      result.onsuccess?.({} as Event);
    } catch {
      result.onerror?.({} as Event);
    }
  });
  return result;
}

function fakeIndexedDB() {
  const records = new Map<string, UploadRecoveryRecord>();
  let created = false;
  return {
    open: vi.fn(() => {
      const openRequest = {} as IDBOpenDBRequest;
      queueMicrotask(() => {
        const objectStore = () => ({
          put: (value: UploadRecoveryRecord) => request(() => { records.set(value.fileId, value); return value.fileId; }),
          delete: (fileId: string) => request(() => { records.delete(fileId); return undefined; }),
          index: () => ({ getAll: (userId: string) => request(() => [...records.values()].filter((item) => item.userId === userId)) }),
          createIndex: vi.fn(),
        });
        const database = {
          objectStoreNames: { contains: () => created },
          createObjectStore: () => { created = true; return objectStore(); },
          transaction: () => ({ objectStore, onabort: null, oncomplete: null, onerror: null }),
          close: vi.fn(),
        } as unknown as IDBDatabase;
        Object.defineProperty(openRequest, "result", { configurable: true, value: database });
        if (!created) openRequest.onupgradeneeded?.({} as IDBVersionChangeEvent);
        openRequest.onsuccess?.({} as Event);
      });
      return openRequest;
    }),
  } as unknown as IDBFactory;
}

function recovery(overrides: Partial<UploadRecoveryRecord> = {}): UploadRecoveryRecord {
  return {
    fileId: "file-1", userId: "user-1", sessionUrl: "https://upload.invalid/private",
    fileName: "notes.txt", fileSize: 5, mimeType: "text/plain", lastModified: 100,
    confirmedOffset: 0, completionPending: false, createdAt: 1000, updatedAt: 1000,
    ...overrides,
  };
}

beforeEach(() => vi.stubGlobal("indexedDB", fakeIndexedDB()));
afterEach(() => vi.unstubAllGlobals());

describe("persistent upload recovery", () => {
  it("creates, updates, isolates, and deletes records by authenticated user", async () => {
    const first = recovery();
    const second = recovery({ fileId: "file-2", userId: "user-2" });
    expect(await saveUploadRecovery(first)).toBe(true);
    await saveUploadRecovery(second);
    const updated = await updateUploadRecovery(first, { confirmedOffset: 4 });
    expect((await listUploadRecoveries("user-1"))).toEqual([expect.objectContaining({ fileId: "file-1", confirmedOffset: 4 })]);
    expect((await listUploadRecoveries("user-2"))).toHaveLength(1);
    expect(updated.updatedAt).toBeGreaterThanOrEqual(first.updatedAt);
    await deleteUploadRecovery("file-1");
    expect(await listUploadRecoveries("user-1")).toEqual([]);
  });

  it("requires matching file metadata before resume", () => {
    const record = recovery();
    const matching = new File(["hello"], "notes.txt", { type: "text/plain", lastModified: 100 });
    expect(matchesRecoveryFile(record, matching)).toBe(true);
    expect(matchesRecoveryFile(record, new File(["world!"], "notes.txt", { type: "text/plain", lastModified: 100 }))).toBe(false);
    expect(matchesRecoveryFile(record, new File(["hello"], "other.txt", { type: "text/plain", lastModified: 100 }))).toBe(false);
    expect(matchesRecoveryFile(record, new File(["hello"], "notes.txt", { type: "application/json", lastModified: 100 }))).toBe(false);
  });

  it("degrades gracefully when IndexedDB is unavailable or fails", async () => {
    vi.stubGlobal("indexedDB", { open: () => { throw new Error("disabled"); } });
    expect(await saveUploadRecovery(recovery())).toBe(false);
    expect(await listUploadRecoveries("user-1")).toEqual([]);
    expect(await deleteUploadRecovery("file-1")).toBe(false);
  });
});
