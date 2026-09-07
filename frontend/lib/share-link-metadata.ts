import { formatBytes } from "@/lib/format";
import { isTransferResult, type ShareResult } from "@/lib/types";

export function shareLinkTitle(result: ShareResult) {
	if (!isTransferResult(result)) return result.file.originalName;
	if (result.files.length === 1) return result.files[0].file.originalName;
	return `${result.files.length} files shared with you`;
}

export function shareLinkDescription(result: ShareResult) {
	const totalBytes = isTransferResult(result)
		? result.files.reduce((total, item) => total + item.file.sizeBytes, 0)
		: result.file.sizeBytes;
	return `${formatBytes(totalBytes)} shared securely through Eterealink. No account required.`;
}
