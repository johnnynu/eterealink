import { afterEach, describe, expect, it, vi } from "vitest";
import { APIError, createAnonymousTransfer, createAnonymousUpload, resolveShare, uploadResumable } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("API client", () => {
  it("sends the browser file metadata expected by the Go API", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ sharePath: "/s/abc" }), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);
    const file = new File(["hello"], "hello.txt", { type: "text/plain" });

    await createAnonymousUpload(file);

    expect(fetchMock).toHaveBeenCalledWith("/api/v1/uploads", expect.objectContaining({ method: "POST" }));
    const request = fetchMock.mock.calls[0][1];
    expect(JSON.parse(request.body)).toEqual({ originalName: "hello.txt", mimeType: "text/plain", sizeBytes: 5 });
  });

  it("preserves structured API errors for share-page states", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: { code: "expired", message: "share has expired" },
    }), { status: 410, headers: { "Content-Type": "application/json" } })));

    await expect(resolveShare("old-link")).rejects.toEqual(new APIError("share has expired", 410, "expired"));
  });

  it("creates one transfer request containing every selected file", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ uploads: [] }), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);

    await createAnonymousTransfer([
      new File(["alpha"], "alpha.txt", { type: "text/plain" }),
      new File(["beta"], "beta.txt", { type: "text/plain" }),
    ]);

    const request = fetchMock.mock.calls[0][1];
    expect(JSON.parse(request.body)).toEqual({ files: [
      { originalName: "alpha.txt", mimeType: "text/plain", sizeBytes: 5 },
      { originalName: "beta.txt", mimeType: "text/plain", sizeBytes: 4 },
    ] });
  });

  it("initiates a resumable session and uploads the file to its session URL", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 201, headers: { Location: "https://upload.invalid/session" } }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const progress = vi.fn();

    const upload = uploadResumable(
      new File(["hello"], "hello.txt", { type: "text/plain" }),
      { url: "https://signed.invalid/start", method: "POST", headers: { "X-Goog-Resumable": "start" }, expiresAt: "soon" },
      progress,
    );
    await upload.promise;

    expect(fetchMock).toHaveBeenNthCalledWith(1, "https://signed.invalid/start", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, "https://upload.invalid/session", expect.objectContaining({
      method: "PUT",
      headers: expect.objectContaining({ "Content-Range": "bytes 0-4/5" }),
    }));
    expect(progress).toHaveBeenLastCalledWith(100);
  });
});
