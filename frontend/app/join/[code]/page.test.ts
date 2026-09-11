import { afterEach, describe, expect, it, vi } from "vitest";
import { generateMetadata } from "@/app/join/[code]/page";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("folder invitation metadata", () => {
  it("uses the invitation details and a link-specific branded image", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      invite: { folderName: "Launch assets", ownerName: "Melo", role: "CONTRIBUTOR" },
    }), { status: 200, headers: { "content-type": "application/json" } })));

    const metadata = await generateMetadata({ params: Promise.resolve({ code: "invite-1" }) });

    expect(metadata.title).toBe("Melo invited you to “Launch assets”");
    expect(metadata.description).toBe("Join as a contributor through Aurea Link.");
    expect(metadata.openGraph?.images).toEqual([
      expect.objectContaining({ url: "/join/invite-1/opengraph-image", width: 1200, height: 630 }),
    ]);
  });

  it("uses branded fallback metadata when the invitation is unavailable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 404 })));

    const metadata = await generateMetadata({ params: Promise.resolve({ code: "missing" }) });

    expect(metadata.title).toBe("Folder invitation");
    expect(metadata.openGraph?.images).toBeDefined();
  });
});
