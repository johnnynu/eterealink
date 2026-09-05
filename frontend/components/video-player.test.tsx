import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatMediaTime, VideoPlayer } from "./video-player";

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

beforeEach(() => {
	const values = new Map<string, string>();
	Object.defineProperty(window, "localStorage", {
		configurable: true,
		value: {
			get length() { return values.size; },
			clear: () => values.clear(),
			getItem: (key: string) => values.get(key) ?? null,
			key: (index: number) => [...values.keys()][index] ?? null,
			removeItem: (key: string) => values.delete(key),
			setItem: (key: string, value: string) => values.set(key, value),
		} satisfies Storage,
	});
});

afterEach(() => {
	for (const view of mounted.splice(0)) {
		act(() => view.unmount());
		view.container.remove();
	}
	vi.useRealTimers();
});

async function renderPlayer() {
	const container = document.createElement("div");
	document.body.append(container);
	const root = createRoot(container);
	mounted.push({ container, unmount: () => root.unmount() });
	await act(async () => root.render(<VideoPlayer name="launch.mp4" url="https://preview.invalid/launch.mp4" />));
	return container;
}

describe("VideoPlayer", () => {
	it("reports the original source resolution and duration from video metadata", async () => {
		const container = await renderPlayer();
		const video = container.querySelector("video")!;
		Object.defineProperties(video, {
			duration: { configurable: true, value: 119 },
			videoWidth: { configurable: true, value: 1920 },
			videoHeight: { configurable: true, value: 1080 },
		});

		await act(async () => video.dispatchEvent(new Event("loadedmetadata", { bubbles: true })));

		expect(container.querySelector(".video-source-badge")?.textContent).toContain("Original");
		expect(container.querySelector(".video-source-badge")?.textContent).toContain("1080p");
		expect(container.querySelector(".video-time")?.textContent).toContain("1:59");
		expect(video.controls).toBe(false);
	});

	it("skips ten seconds in either direction", async () => {
		const container = await renderPlayer();
		const video = container.querySelector("video")!;
		Object.defineProperties(video, {
			currentTime: { configurable: true, value: 50, writable: true },
			duration: { configurable: true, value: 100 },
		});

		await act(async () => {
			video.dispatchEvent(new Event("loadedmetadata", { bubbles: true }));
			video.dispatchEvent(new Event("timeupdate", { bubbles: true }));
		});
		act(() => (container.querySelector('[aria-label="Skip back 10 seconds"]') as HTMLButtonElement).click());
		expect(video.currentTime).toBe(40);
		act(() => (container.querySelector('[aria-label="Skip forward 10 seconds"]') as HTMLButtonElement).click());
		expect(video.currentTime).toBe(50);
	});

	it("shows buffering and actionable codec errors", async () => {
		const container = await renderPlayer();
		const video = container.querySelector("video")!;
		Object.defineProperty(video, "error", { configurable: true, value: { code: 4 } });
		const load = vi.fn();
		Object.defineProperty(video, "load", { configurable: true, value: load });

		await act(async () => video.dispatchEvent(new Event("loadedmetadata", { bubbles: true })));
		act(() => video.dispatchEvent(new Event("waiting", { bubbles: true })));
		expect(container.textContent).toContain("Buffering…");

		act(() => video.dispatchEvent(new Event("error", { bubbles: true })));
		expect(container.textContent).toContain("Video preview unavailable");
		expect(container.textContent).toContain("does not support the video's codec or format");
		expect(container.textContent).toContain("download the original file below");

		act(() => (container.querySelector(".video-error-state button") as HTMLButtonElement).click());
		expect(load).toHaveBeenCalledOnce();
		expect(container.textContent).toContain("Loading video…");
	});

	it("auto-hides controls while playing and restores them on pointer movement", async () => {
		vi.useFakeTimers();
		const container = await renderPlayer();
		const player = container.querySelector(".eterea-video-player")!;
		const video = container.querySelector("video")!;

		act(() => video.dispatchEvent(new Event("playing", { bubbles: true })));
		act(() => vi.advanceTimersByTime(2_500));
		expect(player.classList.contains("controls-hidden")).toBe(true);

		act(() => player.dispatchEvent(new MouseEvent("mousemove", { bubbles: true })));
		expect(player.classList.contains("controls-hidden")).toBe(false);
	});

	it("restores and updates volume and playback-speed preferences", async () => {
		window.localStorage.setItem("eterealink-video-preferences", JSON.stringify({ volume: 0.35, playbackRate: 1.5 }));
		const container = await renderPlayer();
		const video = container.querySelector("video")!;

		await act(async () => video.dispatchEvent(new Event("loadedmetadata", { bubbles: true })));
		expect(video.volume).toBe(0.35);
		expect(video.playbackRate).toBe(1.5);

		const speed = container.querySelector('[aria-label="Playback speed"]') as HTMLSelectElement;
		act(() => {
			speed.value = "2";
			speed.dispatchEvent(new Event("change", { bubbles: true }));
		});
		expect(JSON.parse(window.localStorage.getItem("eterealink-video-preferences")!)).toEqual({ volume: 0.35, playbackRate: 2 });
	});

	it("enters fullscreen on a video double click", async () => {
		const container = await renderPlayer();
		const player = container.querySelector(".eterea-video-player") as HTMLDivElement;
		const video = container.querySelector("video")!;
		const requestFullscreen = vi.fn().mockResolvedValue(undefined);
		Object.defineProperty(player, "requestFullscreen", { configurable: true, value: requestFullscreen });

		act(() => video.dispatchEvent(new MouseEvent("dblclick", { bubbles: true })));
		expect(requestFullscreen).toHaveBeenCalledOnce();
	});

	it("formats short and long media durations", () => {
		expect(formatMediaTime(8.9)).toBe("0:08");
		expect(formatMediaTime(65)).toBe("1:05");
		expect(formatMediaTime(3661)).toBe("1:01:01");
	});
});
