import { describe, expect, it } from "vitest";
import { shareLinkDescription, shareLinkTitle } from "@/lib/share-link-metadata";
import type { ShareResult } from "@/lib/types";

const sharedFile = {
	id: "file-1",
	originalName: "summer-film.mov",
	mimeType: "video/quicktime",
	sizeBytes: 2048,
	status: "READY" as const,
	createdAt: "2026-09-06T12:00:00Z",
};

function directShare(): ShareResult {
	return {
		file: sharedFile,
		share: { id: "share-1", shortCode: "abc", fileId: "file-1", createdAt: "2026-09-06T12:00:00Z" },
		downloadTarget: { url: "https://download.invalid/file", expiresAt: "2026-09-06T12:15:00Z" },
	};
}

function transferShare(fileNames: string[]): ShareResult {
	return {
		transfer: { id: "transfer-1", status: "READY", archiveStatus: "READY", createdAt: "2026-09-06T12:00:00Z", expiresAt: "2026-09-07T12:00:00Z" },
		share: { id: "share-1", shortCode: "abc", transferId: "transfer-1", createdAt: "2026-09-06T12:00:00Z" },
		files: fileNames.map((name, index) => ({
			file: { ...sharedFile, id: `file-${index}`, originalName: name },
			downloadTarget: { url: `https://download.invalid/${index}`, expiresAt: "2026-09-06T12:15:00Z" },
		})),
		archive: { status: "READY", downloadTarget: { url: "https://download.invalid/archive", expiresAt: "2026-09-06T12:15:00Z" } },
	};
}

describe("share link metadata", () => {
	it("uses the file name for direct and single-file transfer links", () => {
		expect(shareLinkTitle(directShare())).toBe("summer-film.mov");
		expect(shareLinkTitle(transferShare(["concert.mp4"]))).toBe("concert.mp4");
	});

	it("uses a clear title for multi-file links", () => {
		expect(shareLinkTitle(transferShare(["one.txt", "two.txt"]))).toBe("2 files shared with you");
		expect(shareLinkDescription(directShare())).toBe("2.00 KB shared securely through Aurea Link. No account required.");
	});
});
