import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FilePreview } from "./file-preview";

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

afterEach(() => {
	for (const view of mounted.splice(0)) {
		act(() => view.unmount());
		view.container.remove();
	}
	vi.unstubAllGlobals();
});

async function renderPreview(element: React.ReactNode) {
	const container = document.createElement("div");
	document.body.append(container);
	const root = createRoot(container);
	mounted.push({ container, unmount: () => root.unmount() });
	await act(async () => {
		root.render(element);
		await Promise.resolve();
	});
	return container;
}

describe("FilePreview", () => {
	it("renders fetched text as escaped content", async () => {
		const maliciousText = `<img src=x onerror="window.hacked=true">`;
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(maliciousText, { status: 200 })));

		const container = await renderPreview(
			<FilePreview name="example.html" preview={{ kind: "text", url: "https://preview.invalid/text", expiresAt: "soon" }} />,
		);

		expect(container.querySelector("pre")?.textContent).toBe(maliciousText);
		expect(container.querySelector("img")).toBeNull();
	});

	it("shows a generic fallback without requesting unsupported files", async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal("fetch", fetchMock);

		const container = await renderPreview(<FilePreview name="archive.zip" />);

		expect(container.textContent).toContain("Preview unavailable");
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("allows the cross-origin browser PDF viewer to run", async () => {
		const container = await renderPreview(
			<FilePreview name="resume.pdf" preview={{ kind: "pdf", url: "https://storage.invalid/resume.pdf", expiresAt: "soon" }} />,
		);
		const frame = container.querySelector("iframe")!;

		expect(frame.getAttribute("src")).toBe("https://storage.invalid/resume.pdf");
		expect(frame.hasAttribute("sandbox")).toBe(false);
		expect(frame.getAttribute("referrerpolicy")).toBe("no-referrer");
	});
});
