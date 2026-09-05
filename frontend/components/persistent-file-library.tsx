"use client";

import { ChangeEvent, DragEvent, FormEvent, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useAuth } from "@/components/auth-context";
import { CopyIcon, DownloadIcon, FileIcon, FolderIcon, LinkIcon, UploadIcon, UsersIcon } from "@/components/icons";
import { FilePreview } from "@/components/file-preview";
import {
  acceptFolderInvite,
  addFolderMember,
  APIError,
  completePersistentUpload,
  createFolder,
  createFolderInvite,
  createPersistentFileShare,
  createPersistentUpload,
  deletePersistentFile,
  deleteFolder,
  getPersistentFileDownload,
  listFolderContents,
  listFolderInvites,
  listFolderMembers,
  movePersistentFiles,
  removeContributedFile,
  removeFolderMember,
  revokeFolderInvite,
  revokePersistentFileShare,
	streamFolderEvents,
  updateFolder,
  uploadResumable,
} from "@/lib/api";
import { formatBytes, formatExpiry, formatFileType, formatRelativeDate } from "@/lib/format";
import { PROFILE_UPDATE_FINISHED_EVENT, PROFILE_UPDATE_STARTED_EVENT } from "@/lib/events";
import type {
	FileDownloadResult,
  FileLibrarySummary,
  FileRecord,
  FolderAccess,
  FolderInvite,
  FolderMember,
  FolderRecord,
  OwnedFileRecord,
  PersistentShareExpiration,
} from "@/lib/types";

const MAX_FILE_BYTES = Number(process.env.NEXT_PUBLIC_MAX_PERSISTENT_FILE_BYTES ?? 5 * 1024 ** 3);
const MAX_FILES_PER_SELECTION = 10;
const FILES_PER_PAGE = 10;

type FileSort = "newest" | "oldest" | "name" | "size";
type FileFilter = "all" | "shared";
type LibraryScope = "owned" | "shared";
type LocationHistoryMode = "none" | "push" | "replace";
type OpenLocationOptions = {
	search?: string;
	sort?: FileSort;
	filter?: FileFilter;
	historyMode?: LocationHistoryMode;
};
type UploadQueueItem = {
	id: string;
	file: File;
	progress: number;
	status: "queued" | "uploading" | "complete" | "failed" | "canceled";
	error?: string;
};

function errorMessage(error: unknown, fallback: string) {
  return error instanceof APIError ? error.message : fallback;
}

function readLibraryLocation() {
	const parameters = new URLSearchParams(window.location.search);
	return {
		folderID: parameters.get("folder") ?? parameters.get("openFolder") ?? undefined,
		scope: parameters.get("scope") === "shared" ? "shared" as const : "owned" as const,
	};
}

function writeLibraryLocation(folderID: string | undefined, scope: LibraryScope, mode: Exclude<LocationHistoryMode, "none">) {
	const url = new URL(window.location.href);
	url.searchParams.delete("folderInvite");
	url.searchParams.delete("openFolder");
	url.searchParams.delete("scope");
	if (folderID) url.searchParams.set("folder", folderID);
	else url.searchParams.delete("folder");
	if (scope === "shared") url.searchParams.set("scope", "shared");
	const nextURL = `${url.pathname}${url.search}${url.hash}`;
	const currentURL = `${window.location.pathname}${window.location.search}${window.location.hash}`;
	if (nextURL === currentURL) return;
	window.history[mode === "push" ? "pushState" : "replaceState"](null, "", nextURL);
}

function folderSnapshot(
	current: FolderAccess | null,
	breadcrumbs: FolderRecord[],
	folders: FolderAccess[],
	files: OwnedFileRecord[] | null,
	summary: FileLibrarySummary | null,
	nextCursor: string,
) {
	return JSON.stringify({ current, breadcrumbs, folders, files, summary, nextCursor });
}

function folderAccessSnapshot(members: FolderMember[], invites: FolderInvite[]) {
	return JSON.stringify({ members, invites });
}

export function PersistentFileLibrary() {
  const { getIDToken, user } = useAuth();
  const [files, setFiles] = useState<OwnedFileRecord[] | null>(null);
  const [summary, setSummary] = useState<FileLibrarySummary | null>(null);
  const [error, setError] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [activeFile, setActiveFile] = useState("");
  const [deletingID, setDeletingID] = useState("");
  const [confirmDeleteID, setConfirmDeleteID] = useState("");
  const [openShareID, setOpenShareID] = useState("");
	const [previewingID, setPreviewingID] = useState("");
	const [openPreview, setOpenPreview] = useState<FileDownloadResult | null>(null);
  const [shareExpiration, setShareExpiration] = useState<PersistentShareExpiration>("7d");
  const [shareBusyID, setShareBusyID] = useState("");
  const [copiedShareID, setCopiedShareID] = useState("");
  const [page, setPage] = useState(1);
	const [nextCursor, setNextCursor] = useState("");
	const [cursorHistory, setCursorHistory] = useState<string[]>([""]);
  const [searchQuery, setSearchQuery] = useState("");
  const [sort, setSort] = useState<FileSort>("newest");
  const [filter, setFilter] = useState<FileFilter>("all");
  const [dragging, setDragging] = useState(false);
	const [scope, setScope] = useState<LibraryScope>("owned");
	const [currentFolder, setCurrentFolder] = useState<FolderAccess | null>(null);
	const [breadcrumbs, setBreadcrumbs] = useState<FolderRecord[]>([]);
	const [folders, setFolders] = useState<FolderAccess[]>([]);
	const [newFolderName, setNewFolderName] = useState("");
	const [creatingFolder, setCreatingFolder] = useState(false);
	const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
	const [memberEmail, setMemberEmail] = useState("");
	const [memberRole, setMemberRole] = useState<"VIEWER" | "CONTRIBUTOR">("VIEWER");
	const [members, setMembers] = useState<FolderMember[]>([]);
	const [folderInvites, setFolderInvites] = useState<FolderInvite[]>([]);
	const [inviteRole, setInviteRole] = useState<"VIEWER" | "CONTRIBUTOR">("VIEWER");
	const [inviteExpiration, setInviteExpiration] = useState<PersistentShareExpiration>("7d");
	const [copiedInviteID, setCopiedInviteID] = useState("");
	const [sharingFolder, setSharingFolder] = useState(false);
	const [memberPanelOpen, setMemberPanelOpen] = useState(false);
	const [editFolderName, setEditFolderName] = useState("");
	const [folderBusy, setFolderBusy] = useState(false);
	const [bulkDeleteConfirm, setBulkDeleteConfirm] = useState(false);
	const [uploadQueue, setUploadQueue] = useState<UploadQueueItem[]>([]);
	const [folderUpdateToast, setFolderUpdateToast] = useState(0);
	const activeUpload = useRef<{ id: string; abort: () => void } | null>(null);
	const uploadSequence = useRef(0);
	const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
	const openLocationRef = useRef<((folderID?: string, nextScope?: LibraryScope, cursor?: string, overrides?: OpenLocationOptions, resetCursor?: boolean) => Promise<void>) | null>(null);
	const refreshCurrentFolderRef = useRef<((announceChange?: boolean) => Promise<void>) | null>(null);
	const backgroundRefreshInFlight = useRef(false);
	const backgroundRefreshPending = useRef(false);
	const backgroundRefreshPendingNotice = useRef(false);
	const folderUpdateToastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
	const profileUpdatePending = useRef(false);
	const profileUpdateGraceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
	const displayedFolderSnapshot = useRef("");
	const displayedAccessSnapshot = useRef("");
	const locationVersion = useRef(0);
	const libraryView = useRef({
		folderID: undefined as string | undefined,
		scope: "owned" as LibraryScope,
		search: "",
		sort: "newest" as FileSort,
		filter: "all" as FileFilter,
		cursor: "",
		memberPanelOpen: false,
		role: undefined as FolderAccess["role"] | undefined,
	});
	libraryView.current = {
		folderID: currentFolder?.folder.id,
		scope,
		search: searchQuery,
		sort,
		filter,
		cursor: cursorHistory[page - 1] ?? "",
		memberPanelOpen,
		role: currentFolder?.role,
	};
	displayedFolderSnapshot.current = folderSnapshot(currentFolder, breadcrumbs, folders, files, summary, nextCursor);
	displayedAccessSnapshot.current = folderAccessSnapshot(members, folderInvites);
	const canUpload = scope === "owned" || (scope === "shared" && currentFolder?.role === "CONTRIBUTOR");

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const token = await getIDToken();
		let result;
		const requestedLocation = readLibraryLocation();
		let initialScope: LibraryScope = requestedLocation.scope;
		let inviteError = "";
		let canonicalizeLocation = false;
		const initialParameters = new URLSearchParams(window.location.search);
		const inviteCode = initialParameters.get("folderInvite");
		const openFolderID = requestedLocation.folderID;
		if (inviteCode) {
			try {
				const access = await acceptFolderInvite(inviteCode, token);
				result = await listFolderContents(token, access.folder.id, "shared", { sort: "newest", limit: FILES_PER_PAGE });
				initialScope = "shared";
				canonicalizeLocation = true;
			} catch (inviteFailure) {
				inviteError = errorMessage(inviteFailure, "This folder invite could not be accepted.");
			}
		}
		if (!result && openFolderID) {
			try {
				result = await listFolderContents(token, openFolderID, requestedLocation.scope, { sort: "newest", limit: FILES_PER_PAGE });
				initialScope = requestedLocation.scope;
				canonicalizeLocation = true;
			} catch (folderFailure) {
				inviteError = errorMessage(folderFailure, "This folder could not be opened.");
			}
		}
		if (!result) result = await listFolderContents(token, undefined, initialScope, { sort: "newest", limit: FILES_PER_PAGE });
        if (active) {
          setFiles(result.files);
			setFolders(result.folders);
			setBreadcrumbs(result.breadcrumbs);
			setCurrentFolder(result.current ?? null);
			setEditFolderName(result.current?.folder.name ?? "");
			setScope(initialScope);
			setNextCursor(result.nextCursor ?? "");
			setError(inviteError);
			if (canonicalizeLocation) writeLibraryLocation(result.current?.folder.id, initialScope, "replace");
          setSummary(result.summary ?? {
            fileCount: result.files.length,
            totalBytes: result.files.reduce((total, entry) => total + entry.file.sizeBytes, 0),
          });
        }
      } catch (loadError) {
        if (active) {
          setFiles([]);
          setSummary({ fileCount: 0, totalBytes: 0 });
          setError(errorMessage(loadError, "Your files could not be loaded. Please try again."));
        }
      }
    })();
    return () => {
		active = false;
		if (searchTimer.current) clearTimeout(searchTimer.current);
		if (folderUpdateToastTimer.current) clearTimeout(folderUpdateToastTimer.current);
	};
  }, [getIDToken]);

	useEffect(() => {
		if (!memberPanelOpen) return;
		function closeOnEscape(event: KeyboardEvent) {
			if (event.key === "Escape") setMemberPanelOpen(false);
		}
		window.addEventListener("keydown", closeOnEscape);
		return () => window.removeEventListener("keydown", closeOnEscape);
	}, [memberPanelOpen]);

	async function openLocation(
		folderID?: string,
		nextScope: LibraryScope = scope,
		cursor = "",
		overrides: OpenLocationOptions = {},
		resetCursor = true,
	) {
		const requestVersion = ++locationVersion.current;
		dismissFolderUpdateToast();
		if (searchTimer.current) {
			clearTimeout(searchTimer.current);
			searchTimer.current = null;
		}
		setError("");
		setFiles(null);
		setSelectedIDs([]);
		try {
			const token = await getIDToken();
			const result = await listFolderContents(token, folderID, nextScope, {
				search: overrides.search ?? searchQuery,
				sort: overrides.sort ?? sort,
				filter: overrides.filter ?? filter,
				limit: FILES_PER_PAGE,
				cursor,
			});
			if (requestVersion !== locationVersion.current) return;
			setScope(nextScope);
			setCurrentFolder(result.current ?? null);
			setEditFolderName(result.current?.folder.name ?? "");
			setBreadcrumbs(result.breadcrumbs ?? []);
			setFolders(result.folders ?? []);
			setFiles(result.files ?? []);
			setSummary(result.summary ?? { fileCount: 0, totalBytes: 0 });
			setNextCursor(result.nextCursor ?? "");
			setMembers([]);
			setFolderInvites([]);
			setCopiedInviteID("");
			setMemberPanelOpen(false);
			if (overrides.historyMode && overrides.historyMode !== "none") {
				writeLibraryLocation(result.current?.folder.id, nextScope, overrides.historyMode);
			}
			if (resetCursor) {
				setPage(1);
				setCursorHistory([""]);
			}
		} catch (loadError) {
			if (requestVersion !== locationVersion.current) return;
			setFiles([]);
			setFolders([]);
			setError(errorMessage(loadError, "This folder could not be loaded. Please try again."));
		}
	}

	openLocationRef.current = openLocation;

	async function refreshCurrentFolder(announceChange = false) {
		if (backgroundRefreshInFlight.current) {
			backgroundRefreshPending.current = true;
			backgroundRefreshPendingNotice.current ||= announceChange;
			return;
		}
		const view = libraryView.current;
		if (!view.folderID) return;
		const requestVersion = locationVersion.current;
		backgroundRefreshInFlight.current = true;
		try {
			const token = await getIDToken();
			const result = await listFolderContents(token, view.folderID, view.scope, {
				search: view.search,
				sort: view.sort,
				filter: view.filter,
				limit: FILES_PER_PAGE,
				cursor: view.cursor,
			});
			if (requestVersion !== locationVersion.current || libraryView.current.folderID !== view.folderID) return;
			const nextCurrent = result.current ?? null;
			const nextBreadcrumbs = result.breadcrumbs ?? [];
			const nextFolders = result.folders ?? [];
			const nextFiles = result.files ?? [];
			const nextSummary = result.summary ?? { fileCount: 0, totalBytes: 0 };
			const nextPageCursor = result.nextCursor ?? "";
			let changed = displayedFolderSnapshot.current !== folderSnapshot(
				nextCurrent,
				nextBreadcrumbs,
				nextFolders,
				nextFiles,
				nextSummary,
				nextPageCursor,
			);
			setScope(view.scope);
			setCurrentFolder(nextCurrent);
			setBreadcrumbs(nextBreadcrumbs);
			setFolders(nextFolders);
			setFiles(nextFiles);
			setSummary(nextSummary);
			setNextCursor(nextPageCursor);
			const visibleIDs = new Set(nextFiles.map((entry) => entry.file.id));
			setSelectedIDs((current) => current.filter((id) => visibleIDs.has(id)));

			if (view.memberPanelOpen && view.role === "OWNER") {
				const [nextMembers, nextInvites] = await Promise.all([
					listFolderMembers(view.folderID, token),
					listFolderInvites(view.folderID, token),
				]);
				if (requestVersion === locationVersion.current && libraryView.current.folderID === view.folderID) {
					changed ||= displayedAccessSnapshot.current !== folderAccessSnapshot(nextMembers, nextInvites);
					setMembers(nextMembers);
					setFolderInvites(nextInvites);
				}
			}
			if (announceChange && changed) showFolderUpdateToast();
		} catch (refreshError) {
			if (requestVersion !== locationVersion.current || libraryView.current.folderID !== view.folderID) return;
			if (refreshError instanceof APIError && (refreshError.status === 403 || refreshError.status === 404)) {
				await openLocationRef.current?.(undefined, view.scope, "", {
					search: "", sort: "newest", filter: "all", historyMode: "replace",
				});
				setError("This folder is no longer available to you.");
			}
		} finally {
			backgroundRefreshInFlight.current = false;
			if (backgroundRefreshPending.current) {
				const announcePendingChange = backgroundRefreshPendingNotice.current;
				backgroundRefreshPending.current = false;
				backgroundRefreshPendingNotice.current = false;
				void refreshCurrentFolderRef.current?.(announcePendingChange);
			}
		}
	}

	refreshCurrentFolderRef.current = refreshCurrentFolder;

	function dismissFolderUpdateToast() {
		if (folderUpdateToastTimer.current) {
			clearTimeout(folderUpdateToastTimer.current);
			folderUpdateToastTimer.current = null;
		}
		setFolderUpdateToast(0);
	}

	function showFolderUpdateToast() {
		if (folderUpdateToastTimer.current) clearTimeout(folderUpdateToastTimer.current);
		setFolderUpdateToast((current) => current + 1);
		folderUpdateToastTimer.current = setTimeout(() => {
			setFolderUpdateToast(0);
			folderUpdateToastTimer.current = null;
		}, 4_200);
	}

	useEffect(() => {
		function profileUpdateStarted() {
			if (profileUpdateGraceTimer.current) clearTimeout(profileUpdateGraceTimer.current);
			profileUpdatePending.current = true;
		}

		function profileUpdateFinished() {
			if (profileUpdateGraceTimer.current) clearTimeout(profileUpdateGraceTimer.current);
			profileUpdateGraceTimer.current = setTimeout(() => {
				profileUpdatePending.current = false;
				profileUpdateGraceTimer.current = null;
			}, 1_500);
		}

		window.addEventListener(PROFILE_UPDATE_STARTED_EVENT, profileUpdateStarted);
		window.addEventListener(PROFILE_UPDATE_FINISHED_EVENT, profileUpdateFinished);
		return () => {
			window.removeEventListener(PROFILE_UPDATE_STARTED_EVENT, profileUpdateStarted);
			window.removeEventListener(PROFILE_UPDATE_FINISHED_EVENT, profileUpdateFinished);
			if (profileUpdateGraceTimer.current) clearTimeout(profileUpdateGraceTimer.current);
		};
	}, []);

	useEffect(() => {
		const folderID = currentFolder?.folder.id;
		if (!folderID) return;
		let stopped = false;
		let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
		let activeController: AbortController | null = null;

		const waitToReconnect = (delay: number) => new Promise<void>((resolve) => {
			reconnectTimer = setTimeout(resolve, delay);
		});
		const run = async () => {
			let failedAttempts = 0;
			while (!stopped) {
				activeController = new AbortController();
				let opened = false;
				let changed = false;
				const connectedAt = Date.now();
				try {
					const token = await getIDToken();
					await streamFolderEvents(
						token,
						folderID,
						activeController.signal,
						() => {
							opened = true;
							failedAttempts = 0;
							void refreshCurrentFolderRef.current?.(!profileUpdatePending.current);
						},
						() => {
							changed = true;
							void refreshCurrentFolderRef.current?.(!profileUpdatePending.current);
						},
					);
				} catch (streamError) {
					if (activeController.signal.aborted || stopped) return;
					if (streamError instanceof APIError && (streamError.status === 403 || streamError.status === 404)) {
						void refreshCurrentFolderRef.current?.();
						return;
					}
				}
				if (stopped) return;
				const wasStable = opened && Date.now() - connectedAt > 30_000;
				if (changed || wasStable) continue;
				const delay = Math.min(30_000, 1_000 * 2 ** failedAttempts) + Math.floor(Math.random() * 250);
				failedAttempts++;
				await waitToReconnect(delay);
			}
		};
		void run();

		function refreshWhenVisible() {
			if (document.visibilityState === "visible") void refreshCurrentFolderRef.current?.(true);
		}
		document.addEventListener("visibilitychange", refreshWhenVisible);
		return () => {
			stopped = true;
			activeController?.abort();
			if (reconnectTimer) clearTimeout(reconnectTimer);
			document.removeEventListener("visibilitychange", refreshWhenVisible);
		};
	}, [currentFolder?.folder.id, getIDToken]);

	useEffect(() => {
		function restoreLocation() {
			const location = readLibraryLocation();
			setSearchQuery("");
			setSort("newest");
			setFilter("all");
			resetLibraryView();
			void openLocationRef.current?.(location.folderID, location.scope, "", {
				search: "",
				sort: "newest",
				filter: "all",
				historyMode: "none",
			});
		}
		window.addEventListener("popstate", restoreLocation);
		return () => window.removeEventListener("popstate", restoreLocation);
	}, []);

	async function renameCurrentFolder(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (!currentFolder || !editFolderName.trim()) return;
		setFolderBusy(true);
		try {
			const token = await getIDToken();
			const folder = await updateFolder(currentFolder.folder.id, editFolderName, currentFolder.folder.parentFolderId, token);
			setCurrentFolder((current) => current ? { ...current, folder } : current);
			setBreadcrumbs((current) => current.map((item) => item.id === folder.id ? folder : item));
		} catch (folderError) {
			setError(errorMessage(folderError, "The folder could not be renamed."));
		} finally {
			setFolderBusy(false);
		}
	}

	async function removeCurrentFolder() {
		if (!currentFolder) return;
		setFolderBusy(true);
		try {
			const token = await getIDToken();
			const parentID = currentFolder.folder.parentFolderId;
			await deleteFolder(currentFolder.folder.id, token);
			await openLocation(parentID, "owned", "", { historyMode: "replace" });
		} catch (folderError) {
			setError(errorMessage(folderError, "Only an empty folder can be deleted."));
		} finally {
			setFolderBusy(false);
		}
	}

	async function submitFolder(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (!newFolderName.trim() || creatingFolder) return;
		setCreatingFolder(true);
		setError("");
		try {
			const token = await getIDToken();
			await createFolder(newFolderName, currentFolder?.folder.id, token);
			setNewFolderName("");
			await openLocation(currentFolder?.folder.id, scope);
		} catch (folderError) {
			setError(errorMessage(folderError, "The folder could not be created."));
		} finally {
			setCreatingFolder(false);
		}
	}

	async function moveSelected(folderID?: string) {
		if (selectedIDs.length === 0) return;
		setError("");
		try {
			const token = await getIDToken();
			await movePersistentFiles(selectedIDs, folderID, token);
			await openLocation(currentFolder?.folder.id, "owned");
		} catch (moveError) {
			setError(errorMessage(moveError, "The selected files could not be moved."));
		}
	}

	async function deleteSelected() {
		if (!bulkDeleteConfirm) {
			setBulkDeleteConfirm(true);
			return;
		}
		setError("");
		try {
			const token = await getIDToken();
			const removed = (files ?? []).filter((entry) => selectedIDs.includes(entry.file.id));
			for (const fileID of selectedIDs) await deletePersistentFile(fileID, token);
			setFiles((current) => current?.filter((entry) => !selectedIDs.includes(entry.file.id)) ?? []);
			setSummary((current) => ({
				fileCount: Math.max(0, (current?.fileCount ?? 0) - removed.length),
				totalBytes: Math.max(0, (current?.totalBytes ?? 0) - removed.reduce((total, entry) => total + entry.file.sizeBytes, 0)),
				quotaBytes: current?.quotaBytes,
			}));
			setSelectedIDs([]);
			setBulkDeleteConfirm(false);
		} catch (deleteError) {
			setError(errorMessage(deleteError, "The selected files could not all be deleted."));
		}
	}

	async function toggleMembers() {
		if (!currentFolder) return;
		if (memberPanelOpen) {
			setMemberPanelOpen(false);
			return;
		}
		setMemberPanelOpen(true);
		setSharingFolder(true);
		try {
			const token = await getIDToken();
			const [nextMembers, nextInvites] = await Promise.all([
				listFolderMembers(currentFolder.folder.id, token),
				listFolderInvites(currentFolder.folder.id, token),
			]);
			setMembers(nextMembers);
			setFolderInvites(nextInvites);
		} catch (memberError) {
			setError(errorMessage(memberError, "Folder access could not be loaded."));
		} finally {
			setSharingFolder(false);
		}
	}

	async function inviteMember(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (!currentFolder || !memberEmail.trim()) return;
		setSharingFolder(true);
		setError("");
		try {
			const token = await getIDToken();
			const member = await addFolderMember(currentFolder.folder.id, memberEmail, memberRole, token);
			setMembers((current) => [...current.filter((item) => item.user.id !== member.user.id), member]);
			setMemberEmail("");
		} catch (memberError) {
			setError(errorMessage(memberError, "The member could not be added. They must sign in to Eterealink first."));
		} finally {
			setSharingFolder(false);
		}
	}

	async function revokeMember(userID: string) {
		if (!currentFolder) return;
		setSharingFolder(true);
		try {
			const token = await getIDToken();
			await removeFolderMember(currentFolder.folder.id, userID, token);
			setMembers((current) => current.filter((item) => item.user.id !== userID));
		} catch (memberError) {
			setError(errorMessage(memberError, "The member could not be removed."));
		} finally {
			setSharingFolder(false);
		}
	}

	async function createInviteLink() {
		if (!currentFolder) return;
		setSharingFolder(true);
		setError("");
		try {
			const token = await getIDToken();
			const result = await createFolderInvite(currentFolder.folder.id, inviteRole, inviteExpiration, token);
			setFolderInvites((current) => [result.invite, ...current]);
			await copyInviteLink(result.invite.id, result.invitePath);
		} catch (inviteError) {
			setError(errorMessage(inviteError, "The invite link could not be created."));
		} finally {
			setSharingFolder(false);
		}
	}

	async function copyInviteLink(inviteID: string, invitePath: string) {
		try {
			await navigator.clipboard.writeText(absoluteShareURL(invitePath));
			setCopiedInviteID(inviteID);
			window.setTimeout(() => setCopiedInviteID((current) => current === inviteID ? "" : current), 1800);
		} catch {
			setError("The invite link could not be copied. Select it and copy it manually.");
		}
	}

	async function revokeInvite(inviteID: string) {
		if (!currentFolder) return;
		setSharingFolder(true);
		try {
			const token = await getIDToken();
			await revokeFolderInvite(currentFolder.folder.id, inviteID, token);
			setFolderInvites((current) => current.filter((invite) => invite.id !== inviteID));
		} catch (inviteError) {
			setError(errorMessage(inviteError, "The invite link could not be revoked."));
		} finally {
			setSharingFolder(false);
		}
	}

  async function uploadSelectedFiles(selected: File[]) {
	if (!canUpload || selected.length === 0 || uploading) return;
    if (selected.length > MAX_FILES_PER_SELECTION) {
      setError(`Choose no more than ${MAX_FILES_PER_SELECTION} files at once.`);
      return;
    }
    if (selected.some((file) => file.size <= 0 || file.size > MAX_FILE_BYTES)) {
      setError(`Each file must contain data and be no larger than ${formatBytes(MAX_FILE_BYTES)}.`);
      return;
    }

    setUploading(true);
    setUploadProgress(0);
    setError("");
	const queue = selected.map((file) => ({
		id: `upload-${uploadSequence.current += 1}`,
		file,
		progress: 0,
		status: "queued" as const,
	}));
	setUploadQueue(queue);
    const completed: FileRecord[] = [];
	const failures: string[] = [];
    try {
      const token = await getIDToken();
	  for (let index = 0; index < queue.length; index += 1) {
		const item = queue[index];
		const file = item.file;
		let pendingID = "";
        setActiveFile(file.name);
		setUploadQueue((current) => current.map((queued) => queued.id === item.id ? { ...queued, status: "uploading", error: undefined } : queued));
		try {
			const created = await createPersistentUpload(file, token, currentFolder?.folder.id);
			pendingID = created.file.id;
			const upload = uploadResumable(file, created.uploadTarget, (filePercent) => {
				setUploadProgress(Math.round(((index + filePercent / 100) / queue.length) * 100));
				setUploadQueue((current) => current.map((queued) => queued.id === item.id ? { ...queued, progress: filePercent } : queued));
			});
			activeUpload.current = { id: item.id, abort: upload.abort };
			await upload.promise;
			completed.push(await completePersistentUpload(created.file.id, token));
			pendingID = "";
			setUploadQueue((current) => current.map((queued) => queued.id === item.id ? { ...queued, progress: 100, status: "complete" } : queued));
		} catch (uploadError) {
			const message = errorMessage(uploadError, `${file.name} could not be uploaded.`);
			const canceled = uploadError instanceof APIError && uploadError.code === "canceled";
			if (!canceled) failures.push(message);
			setUploadQueue((current) => current.map((queued) => queued.id === item.id ? { ...queued, status: canceled ? "canceled" : "failed", error: canceled ? undefined : message } : queued));
			if (pendingID) {
				try { await deletePersistentFile(pendingID, token); } catch { /* hidden pending metadata is cleaned up later */ }
			}
		} finally {
			activeUpload.current = null;
		}
      }
	  if (completed.length > 0) await openLocation(currentFolder?.folder.id, scope);
      setUploadProgress(100);
		if (failures.length > 0) setError(`${failures.length} ${failures.length === 1 ? "file" : "files"} could not be uploaded. Retry from the queue below.`);
	} catch (uploadError) {
		setError(errorMessage(uploadError, "Your uploads could not be started. Please try again."));
    } finally {
	  activeUpload.current = null;
      setUploading(false);
      setActiveFile("");
    }
  }

	function cancelActiveUpload(itemID: string) {
		if (activeUpload.current?.id === itemID) activeUpload.current.abort();
	}

  async function uploadFiles(event: ChangeEvent<HTMLInputElement>) {
    const selected = Array.from(event.target.files ?? []);
    event.target.value = "";
    await uploadSelectedFiles(selected);
  }

  async function dropFiles(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setDragging(false);
    await uploadSelectedFiles(Array.from(event.dataTransfer.files));
  }

  async function downloadFile(fileID: string) {
    setError("");
    try {
      const token = await getIDToken();
      const result = await getPersistentFileDownload(fileID, token);
      window.location.assign(result.downloadTarget.url);
    } catch (downloadError) {
      setError(errorMessage(downloadError, "The download could not be prepared. Please try again."));
    }
  }

	async function previewFile(fileID: string) {
		setError("");
		setPreviewingID(fileID);
		try {
			const token = await getIDToken();
			setOpenPreview(await getPersistentFileDownload(fileID, token));
		} catch (previewError) {
			setError(errorMessage(previewError, "The preview could not be prepared. Please try again."));
		} finally {
			setPreviewingID("");
		}
	}

	useEffect(() => {
		if (!openPreview) return;
		function closeOnEscape(event: KeyboardEvent) {
			if (event.key === "Escape") setOpenPreview(null);
		}
		window.addEventListener("keydown", closeOnEscape);
		return () => window.removeEventListener("keydown", closeOnEscape);
	}, [openPreview]);

  async function removeFile(fileID: string) {
    const removedFile = files?.find((entry) => entry.file.id === fileID)?.file;
    setDeletingID(fileID);
    setError("");
    try {
      const token = await getIDToken();
      await deletePersistentFile(fileID, token);
      setFiles((current) => current?.filter((entry) => entry.file.id !== fileID) ?? []);
      if (removedFile) {
        setSummary((current) => ({
          fileCount: Math.max(0, (current?.fileCount ?? 0) - 1),
          totalBytes: Math.max(0, (current?.totalBytes ?? 0) - removedFile.sizeBytes),
		  quotaBytes: current?.quotaBytes,
        }));
      }
      setConfirmDeleteID("");
      setOpenShareID((current) => current === fileID ? "" : current);
    } catch (deleteError) {
      setError(errorMessage(deleteError, "The file could not be deleted. Please try again."));
    } finally {
      setDeletingID("");
    }
  }

	async function removeFileFromFolder(fileID: string) {
		if (!currentFolder) return;
		setDeletingID(fileID);
		setError("");
		try {
			const token = await getIDToken();
			await removeContributedFile(currentFolder.folder.id, fileID, token);
			setFiles((current) => current?.filter((entry) => entry.file.id !== fileID) ?? []);
			setConfirmDeleteID("");
		} catch (removeError) {
			setError(errorMessage(removeError, "The file could not be removed from this folder."));
		} finally {
			setDeletingID("");
		}
	}

  function absoluteShareURL(path: string) {
    if (typeof window === "undefined") return path;
    return new URL(path, window.location.origin).toString();
  }

  async function createShare(fileID: string) {
    setShareBusyID(fileID);
    setError("");
    try {
      const token = await getIDToken();
      const result = await createPersistentFileShare(fileID, shareExpiration, token);
      setFiles((current) => current?.map((entry) => entry.file.id === fileID
        ? { ...entry, share: result.share, sharePath: result.sharePath }
        : entry) ?? []);
    } catch (shareError) {
      setError(errorMessage(shareError, "A share link could not be created. Please try again."));
    } finally {
      setShareBusyID("");
    }
  }

  async function copyShareLink(shareID: string, sharePath: string) {
    setError("");
    try {
      await navigator.clipboard.writeText(absoluteShareURL(sharePath));
      setCopiedShareID(shareID);
      window.setTimeout(() => setCopiedShareID((current) => current === shareID ? "" : current), 1800);
    } catch {
      setError("The link could not be copied. Select it and copy it manually.");
    }
  }

  async function revokeShare(fileID: string, shareID: string) {
    setShareBusyID(fileID);
    setError("");
    try {
      const token = await getIDToken();
      await revokePersistentFileShare(fileID, shareID, token);
      setFiles((current) => current?.map((entry) => entry.file.id === fileID
        ? { file: entry.file }
        : entry) ?? []);
      setCopiedShareID("");
    } catch (shareError) {
      setError(errorMessage(shareError, "The share link could not be revoked. Please try again."));
    } finally {
      setShareBusyID("");
    }
  }

  const visibleFiles = files ?? [];
  const showUploaderAttribution = Boolean(currentFolder && (
	scope === "shared" || visibleFiles.some((entry) => entry.file.ownerId && entry.file.ownerId !== user?.id)
  ));
  const pageStart = (page - 1) * FILES_PER_PAGE;

  function resetLibraryView() {
    setOpenShareID("");
    setConfirmDeleteID("");
    setCopiedShareID("");
  }

	async function goToNextPage() {
		if (!nextCursor) return;
		const cursor = nextCursor;
		setCursorHistory((current) => [...current.slice(0, page), cursor]);
		setPage((current) => current + 1);
		resetLibraryView();
		await openLocation(currentFolder?.folder.id, scope, cursor, {}, false);
	}

	async function goToPreviousPage() {
		if (page <= 1) return;
		const cursor = cursorHistory[page - 2] ?? "";
		setPage((current) => current - 1);
		resetLibraryView();
		await openLocation(currentFolder?.folder.id, scope, cursor, {}, false);
	}

	function updateSearch(value: string) {
		setSearchQuery(value);
		resetLibraryView();
		if (searchTimer.current) clearTimeout(searchTimer.current);
		searchTimer.current = setTimeout(() => { void openLocation(currentFolder?.folder.id, scope, "", { search: value }); }, 250);
	}

  return (
    <div
      className={`library-content ${dragging ? "is-dragging" : ""}`}
			onDragEnter={(event) => {
        event.preventDefault();
		if (canUpload && !uploading && event.dataTransfer.types.includes("Files")) setDragging(true);
      }}
      onDragOver={(event) => event.preventDefault()}
      onDragLeave={(event) => {
        const nextTarget = event.relatedTarget;
        if (!(nextTarget instanceof Node) || !event.currentTarget.contains(nextTarget)) setDragging(false);
      }}
      onDrop={dropFiles}
    >
      {dragging && <div className="library-drop-overlay" aria-hidden="true"><UploadIcon /> Drop files to add them</div>}
	  {folderUpdateToast > 0 && createPortal(
		<div key={folderUpdateToast} className="folder-update-toast" role="status" aria-live="polite" aria-atomic="true">
			<FolderIcon />
			<span><strong>Folder updated</strong><small>Latest changes are now visible.</small></span>
		</div>,
		document.body,
	  )}
		<nav className="library-scope" aria-label="Library location">
			<button type="button" aria-pressed={scope === "owned"} onClick={() => openLocation(undefined, "owned", "", { historyMode: "push" })}>My files</button>
			<button type="button" aria-pressed={scope === "shared"} onClick={() => openLocation(undefined, "shared", "", { historyMode: "push" })}>Shared with me</button>
		</nav>
		{scope === "owned" && breadcrumbs.length > 0 && (
			<nav className="folder-breadcrumbs" aria-label="Folder breadcrumbs">
				<button type="button" onClick={() => openLocation(undefined, "owned", "", { historyMode: "push" })}>My files</button>
				{breadcrumbs.map((folder) => <span key={folder.id}>/ <button type="button" onClick={() => openLocation(folder.id, "owned", "", { historyMode: "push" })}>{folder.name}</button></span>)}
			</nav>
		)}
		{scope === "shared" && currentFolder && (
			<nav className="folder-breadcrumbs" aria-label="Folder breadcrumbs">
				<button type="button" onClick={() => openLocation(undefined, "shared", "", { historyMode: "push" })}>Shared with me</button>
				{breadcrumbs.map((folder) => <span key={folder.id}>/ <button type="button" onClick={() => openLocation(folder.id, "shared", "", { historyMode: "push" })}>{folder.name}</button></span>)}
			</nav>
		)}
      <div className="panel-heading">
        <div>
          <p className="eyebrow">Library</p>
			<h2 id="library-title">{currentFolder?.folder.name ?? (scope === "shared" ? "Shared with you" : "Your files")}</h2>
			{currentFolder?.role === "VIEWER" && <p className="viewer-note">Read-only · Shared by {currentFolder.owner.displayName || currentFolder.owner.email}</p>}
			{currentFolder?.role === "CONTRIBUTOR" && <p className="viewer-note contributor-note">Contributor · You can upload and manage your own files</p>}
          <p className="library-summary" aria-live="polite">
            {summary === null
              ? "Loading storage usage…"
              : `${summary.fileCount} ${summary.fileCount === 1 ? "file" : "files"} · ${formatBytes(summary.totalBytes)} stored`}
		  </p>
		  {summary?.quotaBytes ? <div className="storage-capacity" title={`${formatBytes(summary.totalBytes)} of ${formatBytes(summary.quotaBytes)}`}><span style={{ width: `${Math.min(100, (summary.totalBytes / summary.quotaBytes) * 100)}%` }} /></div> : null}
        </div>
		{canUpload && <label className={`primary-button library-upload-button ${uploading ? "is-disabled" : ""}`} htmlFor="owned-files-input">
          <UploadIcon /> {uploading ? "Uploading…" : "Upload files"}
		</label>}
        <input
          id="owned-files-input"
          className="visually-hidden"
          type="file"
          multiple
          disabled={uploading}
          onChange={uploadFiles}
        />
      </div>

	  {uploading && uploadQueue.length > 1 && (
        <div className="library-progress" role="status">
		  <span>Uploading {uploadQueue.filter((item) => item.status === "complete").length + 1} of {uploadQueue.length} · {activeFile}</span>
          <strong>{uploadProgress}%</strong>
          <div className="progress-track"><span style={{ width: `${uploadProgress}%` }} /></div>
        </div>
      )}
	  {uploadQueue.length > 0 && (
		<div className="persistent-upload-queue" aria-label="Persistent upload queue">
			{uploadQueue.map((item) => (
				<div className={`upload-queue-item ${item.status}`} key={item.id}>
					<span><strong>{item.file.name}</strong><small>{item.status === "failed" ? item.error : item.status}</small></span>
					<div className="queue-progress"><span style={{ width: `${item.progress}%` }} /></div>
					{item.status === "uploading" && <button type="button" onClick={() => cancelActiveUpload(item.id)}>Cancel</button>}
					{(item.status === "failed" || item.status === "canceled") && <button type="button" disabled={uploading} onClick={() => uploadSelectedFiles([item.file])}>Retry</button>}
				</div>
			))}
			{!uploading && uploadQueue.every((item) => item.status === "complete") && <button type="button" className="clear-upload-queue" onClick={() => setUploadQueue([])}>Clear completed</button>}
		</div>
	  )}
      {error && <p className="error-message library-error" role="alert">{error}</p>}

		{scope === "owned" && (
			<div className="folder-tools">
				<form onSubmit={submitFolder}>
					<label htmlFor="new-folder-name" className="visually-hidden">New folder name</label>
					<input id="new-folder-name" value={newFolderName} onChange={(event) => setNewFolderName(event.target.value)} placeholder="New folder name" maxLength={255} />
					<button type="submit" disabled={creatingFolder || !newFolderName.trim()}><FolderIcon /> {creatingFolder ? "Creating…" : "New folder"}</button>
				</form>
				{currentFolder?.role === "OWNER" && <button type="button" className="manage-viewers" aria-expanded={memberPanelOpen} onClick={toggleMembers}><UsersIcon /> {memberPanelOpen ? "Close access" : "Manage access"}</button>}
			</div>
		)}

		{currentFolder?.role === "OWNER" && memberPanelOpen && (
			<div className="folder-sharing-panel" aria-label="Folder access">
				<div className="folder-sharing-heading">
					<div><strong>Folder access</strong><p>Viewers can download. Contributors can also upload and manage only their own files.</p></div>
					<button type="button" className="close-sharing-panel" onClick={() => setMemberPanelOpen(false)} aria-label="Close folder access">Close</button>
				</div>
				<section className="folder-access-section">
					<div><strong>Add an existing user</strong><p>Use this when you know their Eterealink email.</p></div>
					<form onSubmit={inviteMember}>
						<input type="email" required placeholder="person@example.com" value={memberEmail} onChange={(event) => setMemberEmail(event.target.value)} />
						<select aria-label="Member role" value={memberRole} onChange={(event) => setMemberRole(event.target.value as "VIEWER" | "CONTRIBUTOR")}>
							<option value="VIEWER">Viewer</option><option value="CONTRIBUTOR">Contributor</option>
						</select>
						<button disabled={sharingFolder}>Add</button>
					</form>
				</section>
				<section className="folder-access-section invite-link-creator">
					<div><strong>Create an invite link</strong><p>Expiration is the deadline to join. Accepted access remains until you remove the member.</p></div>
					<div className="invite-link-controls">
						<select aria-label="Invite role" value={inviteRole} onChange={(event) => setInviteRole(event.target.value as "VIEWER" | "CONTRIBUTOR")}>
							<option value="VIEWER">Viewer</option><option value="CONTRIBUTOR">Contributor</option>
						</select>
						<select aria-label="Invite expiration" value={inviteExpiration} onChange={(event) => setInviteExpiration(event.target.value as PersistentShareExpiration)}>
							<option value="24h">24 hours</option><option value="7d">7 days</option><option value="30d">30 days</option><option value="never">Never</option>
						</select>
						<button type="button" disabled={sharingFolder} onClick={createInviteLink}><LinkIcon /> Create link</button>
					</div>
				</section>
				<details className="folder-invite-list">
					<summary>Active invite links <span>{folderInvites.length}</span></summary>
					<div>
						{folderInvites.length === 0 && <p className="folder-access-empty">There are no active invite links.</p>}
						{folderInvites.map((invite) => (
							<div className="folder-invite" key={invite.id}>
								<span><strong>{invite.role === "CONTRIBUTOR" ? "Contributor" : "Viewer"} invite</strong><small>{invite.expiresAt ? `Join by ${formatExpiry(invite.expiresAt)}` : "No joining deadline"}</small></span>
								<input aria-label={`${invite.role.toLowerCase()} invite link`} readOnly value={absoluteShareURL(`/join/${invite.shortCode}`)} />
								<button type="button" onClick={() => copyInviteLink(invite.id, `/join/${invite.shortCode}`)}><CopyIcon /> {copiedInviteID === invite.id ? "Copied" : "Copy"}</button>
								<button type="button" className="revoke-folder-access" disabled={sharingFolder} onClick={() => revokeInvite(invite.id)}>Revoke</button>
							</div>
						))}
					</div>
				</details>
				<section className="folder-member-section">
					<div><strong>Members ({members.length})</strong><p>People with direct or inherited access to this folder.</p></div>
					{members.length === 0 && !sharingFolder && <p className="folder-access-empty">No members have joined this folder yet.</p>}
					{members.length > 0 && <div className="folder-member-list">
						{members.map((member) => (
							<div className="folder-member" key={member.user.id}>
								<span className="folder-member-identity">
									<strong>{member.user.displayName || member.user.email}</strong>
									<small className="folder-member-email">{member.user.email}</small>
									{member.inherited && member.sourceFolderName && <small className="folder-member-source">Inherited from {member.sourceFolderName}</small>}
								</span>
								<div className="folder-member-controls">
									<em>{member.role === "CONTRIBUTOR" ? "Contributor" : "Viewer"}</em>
									{member.inherited && member.sourceFolderId ? (
										<button type="button" className="manage-inherited-access" aria-label={`Manage ${member.user.displayName || member.user.email}'s access in ${member.sourceFolderName || "parent folder"}`} onClick={() => openLocation(member.sourceFolderId, "owned", "", { historyMode: "push" })}>Manage source</button>
									) : (
										<button type="button" disabled={sharingFolder} onClick={() => revokeMember(member.user.id)}>Remove</button>
									)}
								</div>
							</div>
						))}
					</div>}
				</section>
			</div>
		)}

		{currentFolder?.role === "OWNER" && (
			<div className="folder-owner-actions">
				<form onSubmit={renameCurrentFolder}><label htmlFor="rename-folder" className="visually-hidden">Rename folder</label><input id="rename-folder" value={editFolderName} maxLength={255} onChange={(event) => setEditFolderName(event.target.value)} /><button disabled={folderBusy || editFolderName.trim() === currentFolder.folder.name}>Rename</button></form>
				{files?.length === 0 && folders.length === 0 ? (
					<button type="button" className="delete-folder" disabled={folderBusy} onClick={removeCurrentFolder}>Delete folder</button>
				) : (
					<span className="folder-delete-note">Move or delete everything to remove this folder.</span>
				)}
			</div>
		)}

		{folders.length > 0 && (
			<div className="folder-grid" aria-label="Folders">
				{folders.map((access) => (
					<button type="button" className="folder-card" key={access.folder.id} onClick={() => openLocation(access.folder.id, scope, "", { historyMode: "push" })}>
						<span><FolderIcon /></span><strong>{access.folder.name}</strong>
						<small>{access.role === "OWNER" ? "Folder" : `${access.role === "CONTRIBUTOR" ? "Contributor" : "Viewer"} · ${access.owner.displayName || access.owner.email}`}</small>
					</button>
				))}
			</div>
		)}

		{selectedIDs.length > 0 && (
			<div className="bulk-actions" role="toolbar" aria-label="Selected file actions">
				<strong>{selectedIDs.length} selected</strong>
				<label>Move to <select defaultValue="" onChange={(event) => { if (event.target.value === "__root") moveSelected(undefined); else if (event.target.value) moveSelected(event.target.value); event.target.value = ""; }}><option value="">Choose folder</option><option value="__root">My files</option>{folders.map((access) => <option key={access.folder.id} value={access.folder.id}>{access.folder.name}</option>)}</select></label>
				<button type="button" className={bulkDeleteConfirm ? "danger" : ""} onClick={deleteSelected}>{bulkDeleteConfirm ? "Confirm delete" : "Delete"}</button>
				<button type="button" onClick={() => setSelectedIDs([])}>Clear</button>
			</div>
		)}

	  {files !== null && ((summary?.fileCount ?? 0) > 0 || searchQuery !== "" || filter !== "all") && (
        <div className="library-toolbar">
          <label className="library-search">
            <span className="visually-hidden">Search your files</span>
            <input
              type="search"
              placeholder="Search files"
              value={searchQuery}
			  onChange={(event) => updateSearch(event.target.value)}
            />
          </label>
          <div className="library-filter" aria-label="Filter files">
            <button
              type="button"
              aria-pressed={filter === "all"}
			  onClick={() => { setFilter("all"); resetLibraryView(); void openLocation(currentFolder?.folder.id, scope, "", { filter: "all" }); }}
            >
              All
            </button>
            <button
              type="button"
              aria-pressed={filter === "shared"}
			  onClick={() => { setFilter("shared"); resetLibraryView(); void openLocation(currentFolder?.folder.id, scope, "", { filter: "shared" }); }}
            >
              Shared
            </button>
          </div>
          <label className="library-sort">
            <span>Sort</span>
            <select
              value={sort}
              onChange={(event) => {
				const nextSort = event.target.value as FileSort;
				setSort(nextSort);
                resetLibraryView();
				void openLocation(currentFolder?.folder.id, scope, "", { sort: nextSort });
              }}
            >
              <option value="newest">Newest</option>
              <option value="oldest">Oldest</option>
              <option value="name">Name</option>
              <option value="size">Largest</option>
            </select>
          </label>
        </div>
      )}

      {files === null ? (
        <div className="library-loading" role="status">
          <span />
          <span />
          <span />
        </div>
		) : files.length === 0 && folders.length === 0 && searchQuery === "" && filter === "all" ? (
		<div className="library-empty">
          <span className="empty-icon"><FileIcon /></span>
          <div>
			<h3>{currentFolder ? "This folder is empty." : scope === "shared" ? "Nothing has been shared with you." : "Your library is empty."}</h3>
			<p>{currentFolder
				? canUpload ? "Upload files here to share them with everyone who can access this folder." : "There are no files in this shared folder yet."
				: scope === "shared" ? "Folders shared by other Eterealink users will appear here." : "Upload files you want to keep. They stay private and do not expire."}</p>
          </div>
		  {canUpload && <label className="secondary-button file-picker-label" htmlFor="owned-files-input">Choose files</label>}
        </div>
		) : visibleFiles.length === 0 ? (
        <div className="library-no-results">
          <FileIcon />
          <h3>No files found.</h3>
          <p>Try another filename or show all files.</p>
          <button
            type="button"
            onClick={() => {
              setSearchQuery("");
              setFilter("all");
              resetLibraryView();
			  void openLocation(currentFolder?.folder.id, scope, "", { search: "", filter: "all" });
            }}
          >
            Clear filters
          </button>
		</div>
		) : (
        <div className="owned-file-list" aria-label="Your files">
          {visibleFiles.map((entry) => {
            const file = entry.file;
            const shareIsOpen = openShareID === file.id;
			const ownsFile = file.ownerId ? file.ownerId === user?.id : scope === "owned";
			const canRemoveContribution = currentFolder?.role === "OWNER" && !ownsFile;
            return (
            <article className={`owned-file-row ${shareIsOpen ? "share-is-open" : ""}`} key={file.id}>
			  {ownsFile && <input className="file-select" type="checkbox" aria-label={`Select ${file.originalName}`} checked={selectedIDs.includes(file.id)} onChange={(event) => setSelectedIDs((current) => event.target.checked ? [...current, file.id] : current.filter((id) => id !== file.id))} />}
              <span className="owned-file-icon"><FileIcon /></span>
              <span className="owned-file-name">
                <strong title={file.originalName}>{file.originalName}</strong>
                <span className="owned-file-kind">
                  {formatFileType(file.mimeType, file.originalName)}
                  {entry.share && <em>Link active</em>}
                </span>
				{showUploaderAttribution && <span className="file-uploader">Uploaded by {ownsFile ? "you" : entry.uploaderName || "another member"}</span>}
              </span>
              <span className="owned-file-meta">{formatBytes(file.sizeBytes)}</span>
              <span className="owned-file-meta" title={formatExpiry(file.completedAt ?? file.createdAt)}>
                {formatRelativeDate(file.completedAt ?? file.createdAt)}
              </span>
              <span className="owned-file-actions">
				{(ownsFile || canRemoveContribution) && confirmDeleteID === file.id ? (
                  <>
                    <button type="button" className="row-action" onClick={() => setConfirmDeleteID("")}>Cancel</button>
					<button type="button" className="row-action danger" disabled={deletingID === file.id} onClick={() => ownsFile ? removeFile(file.id) : removeFileFromFolder(file.id)}>
					  {deletingID === file.id ? "Removing…" : ownsFile ? "Delete permanently" : "Remove from folder"}
                    </button>
                  </>
				) : (
                  <>
					<button type="button" className="row-action preview" disabled={previewingID === file.id} onClick={() => previewFile(file.id)}>
					  {previewingID === file.id ? "Opening…" : "Preview"}
					</button>
                    <button type="button" className="row-action download" onClick={() => downloadFile(file.id)}>
                      <DownloadIcon /> Download
                    </button>
					{ownsFile && <button
                      type="button"
                      className={`row-action share ${entry.share ? "is-active" : ""}`}
                      aria-expanded={shareIsOpen}
                      onClick={() => {
                        setOpenShareID((current) => current === file.id ? "" : file.id);
                        setShareExpiration("7d");
                        setCopiedShareID("");
                      }}
                    >
                      <LinkIcon /> Share
					</button>}
					{ownsFile && <button type="button" className="row-action" onClick={() => setConfirmDeleteID(file.id)}>Delete</button>}
					{canRemoveContribution && <button type="button" className="row-action" onClick={() => setConfirmDeleteID(file.id)}>Remove</button>}
                  </>
				)}
              </span>
              {shareIsOpen && (
                <div className="file-share-panel">
                  {entry.share && entry.sharePath ? (
                    <>
                      <div className="file-share-copy">
                        <label htmlFor={`share-link-${file.id}`}>Anyone with this link can download</label>
                        <div>
                          <input id={`share-link-${file.id}`} readOnly value={absoluteShareURL(entry.sharePath)} />
                          <button type="button" onClick={() => copyShareLink(entry.share!.id, entry.sharePath!)}>
                            <CopyIcon /> {copiedShareID === entry.share.id ? "Copied" : "Copy"}
                          </button>
                        </div>
                        <small>{entry.share.expiresAt ? `Expires ${formatExpiry(entry.share.expiresAt)}` : "This link does not expire."}</small>
                      </div>
                      <button
                        type="button"
                        className="revoke-share-button"
                        disabled={shareBusyID === file.id}
                        onClick={() => revokeShare(file.id, entry.share!.id)}
                      >
                        {shareBusyID === file.id ? "Revoking…" : "Revoke link"}
                      </button>
                    </>
                  ) : (
                    <>
                      <div>
                        <strong>Create a download link</strong>
                        <p>The file stays in your library. You can revoke the link at any time.</p>
                      </div>
                      <label className="share-expiration-control">
                        <span>Expires</span>
                        <select value={shareExpiration} onChange={(event) => setShareExpiration(event.target.value as PersistentShareExpiration)}>
                          <option value="24h">In 24 hours</option>
                          <option value="7d">In 7 days</option>
                          <option value="30d">In 30 days</option>
                          <option value="never">Never</option>
                        </select>
                      </label>
                      <button
                        type="button"
                        className="create-share-button"
                        disabled={shareBusyID === file.id}
                        onClick={() => createShare(file.id)}
                      >
                        <LinkIcon /> {shareBusyID === file.id ? "Creating…" : "Create link"}
                      </button>
                    </>
                  )}
                </div>
              )}
            </article>
          )})}
		  {(page > 1 || nextCursor) && (
            <nav className="library-pagination" aria-label="File library pages">
              <span>
				{pageStart + 1}–{pageStart + visibleFiles.length}
              </span>
              <div>
				<button type="button" disabled={page === 1} onClick={goToPreviousPage}>
                  Previous
                </button>
				<span>Page {page}</span>
				<button type="button" disabled={!nextCursor} onClick={goToNextPage}>
                  Next
                </button>
              </div>
            </nav>
          )}
        </div>
      )}
	  {openPreview && (
		<div className="preview-dialog-backdrop" role="presentation" onMouseDown={(event) => {
			if (event.target === event.currentTarget) setOpenPreview(null);
		}}>
			<section className="preview-dialog" role="dialog" aria-modal="true" aria-labelledby="preview-dialog-title">
				<header>
					<div>
						<p className="eyebrow">File preview</p>
						<h2 id="preview-dialog-title">{openPreview.file.originalName}</h2>
					</div>
					<button type="button" aria-label="Close preview" onClick={() => setOpenPreview(null)}>×</button>
				</header>
				<FilePreview key={openPreview.file.id} name={openPreview.file.originalName} preview={openPreview.preview} />
				<a className="primary-button download-button" href={openPreview.downloadTarget.url}><DownloadIcon /> Download file</a>
			</section>
		</div>
	  )}
    </div>
  );
}
