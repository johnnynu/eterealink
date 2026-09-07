import { afterEach, describe, expect, it, vi } from "vitest";
import { generateMetadata } from "@/app/s/[code]/page";

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("shared file page metadata", () => {
	it("uses the resolved file name and branded social image", async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
			file: {
				id: "file-1",
				originalName: "concert-film.mp4",
				mimeType: "video/mp4",
				sizeBytes: 4096,
				status: "READY",
				createdAt: "2026-09-06T12:00:00Z",
			},
			share: { id: "share-1", shortCode: "abc", fileId: "file-1", createdAt: "2026-09-06T12:00:00Z" },
			downloadTarget: { url: "https://download.invalid/file", expiresAt: "2026-09-06T12:15:00Z" },
		}), { status: 200, headers: { "content-type": "application/json" } }));
		vi.stubGlobal("fetch", fetchMock);

		const metadata = await generateMetadata({ params: Promise.resolve({ code: "abc" }) });

		expect(fetchMock).toHaveBeenCalledWith("http://localhost:8080/v1/shares/abc", { cache: "no-store" });
		expect(metadata.title).toBe("concert-film.mp4");
		expect(metadata.description).toBe("4.00 KB shared securely through Eterealink. No account required.");
		expect(metadata.openGraph?.images).toEqual([expect.objectContaining({ url: "/opengraph-image", width: 1200, height: 630 })]);
	});

	it("keeps generic metadata when the link cannot be resolved", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 404 })));

		const metadata = await generateMetadata({ params: Promise.resolve({ code: "missing" }) });

		expect(metadata.title).toBe("Shared file");
		expect(metadata.openGraph?.images).toBeDefined();
	});
});
