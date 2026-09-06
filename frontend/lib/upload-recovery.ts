export type UploadRecoveryRecord = {
  fileId: string;
  userId: string;
  sessionUrl: string;
  fileName: string;
  fileSize: number;
  mimeType: string;
  lastModified: number;
  confirmedOffset: number;
  completionPending: boolean;
  folderId?: string;
  createdAt: number;
  updatedAt: number;
};

const DATABASE_NAME = "eterealink-upload-recovery";
const DATABASE_VERSION = 1;
const STORE_NAME = "persistent-uploads";

function openDatabase(): Promise<IDBDatabase | null> {
  if (typeof window === "undefined" || typeof indexedDB === "undefined") return Promise.resolve(null);
  return new Promise((resolve) => {
    try {
      const request = indexedDB.open(DATABASE_NAME, DATABASE_VERSION);
      request.onupgradeneeded = () => {
        const database = request.result;
        if (!database.objectStoreNames.contains(STORE_NAME)) {
          const store = database.createObjectStore(STORE_NAME, { keyPath: "fileId" });
          store.createIndex("userId", "userId", { unique: false });
        }
      };
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => resolve(null);
      request.onblocked = () => resolve(null);
    } catch {
      resolve(null);
    }
  });
}

async function withStore<T>(mode: IDBTransactionMode, operation: (store: IDBObjectStore) => IDBRequest<T>): Promise<T | null> {
  const database = await openDatabase();
  if (!database) return null;
  return new Promise((resolve) => {
    try {
      const transaction = database.transaction(STORE_NAME, mode);
      const request = operation(transaction.objectStore(STORE_NAME));
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => resolve(null);
      transaction.onabort = () => resolve(null);
      transaction.oncomplete = () => database.close();
      transaction.onerror = () => resolve(null);
    } catch {
      database.close();
      resolve(null);
    }
  });
}

export async function saveUploadRecovery(record: UploadRecoveryRecord): Promise<boolean> {
  return (await withStore("readwrite", (store) => store.put(record))) !== null;
}

export async function updateUploadRecovery(
  record: UploadRecoveryRecord,
  update: Partial<Pick<UploadRecoveryRecord, "confirmedOffset" | "completionPending">>,
): Promise<UploadRecoveryRecord> {
  const updated = { ...record, ...update, updatedAt: Date.now() };
  await saveUploadRecovery(updated);
  return updated;
}

export async function listUploadRecoveries(userId: string): Promise<UploadRecoveryRecord[]> {
  const records = await withStore("readonly", (store) => store.index("userId").getAll(userId));
  return Array.isArray(records)
    ? records.filter((record): record is UploadRecoveryRecord => record?.userId === userId)
      .sort((left, right) => right.updatedAt - left.updatedAt)
    : [];
}

export async function deleteUploadRecovery(fileId: string): Promise<boolean> {
  const result = await withStore("readwrite", (store) => store.delete(fileId));
  return result !== null;
}

export function matchesRecoveryFile(record: UploadRecoveryRecord, file: File): boolean {
  return record.fileName === file.name
    && record.fileSize === file.size
    && record.lastModified === file.lastModified
    && (!record.mimeType || !file.type || record.mimeType === file.type);
}
