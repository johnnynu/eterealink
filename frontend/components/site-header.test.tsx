import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SiteHeader } from "./site-header";

vi.mock("next/navigation", () => ({ usePathname: () => "/app" }));
vi.mock("@/components/auth-context", () => ({
	useAuth: () => ({ loading: false, user: { id: "user-1", email: "me@example.com", displayName: "Me" } }),
}));
vi.mock("@/components/auth-menu", () => ({ AuthMenu: () => <span>Account</span> }));

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

afterEach(() => {
	for (const view of mounted.splice(0)) {
		act(() => view.unmount());
		view.container.remove();
	}
	window.history.replaceState(null, "", "/");
});

function renderHeader() {
	const container = document.createElement("div");
	document.body.append(container);
	const root = createRoot(container);
	mounted.push({ container, unmount: () => root.unmount() });
	act(() => root.render(<SiteHeader />));
	return container;
}

describe("SiteHeader", () => {
	it("returns a nested workspace to its library root when the logo is clicked", () => {
		window.history.replaceState(null, "", "/app?folder=deep-folder");
		const onPopState = vi.fn();
		window.addEventListener("popstate", onPopState, { once: true });
		const container = renderHeader();

		act(() => container.querySelector<HTMLAnchorElement>(".brand")?.click());

		expect(window.location.pathname).toBe("/app");
		expect(window.location.search).toBe("");
		expect(onPopState).toHaveBeenCalledOnce();
	});

	it("does the same from the Files navigation link", () => {
		window.history.replaceState(null, "", "/app?folder=deep-folder&scope=shared");
		const onPopState = vi.fn();
		window.addEventListener("popstate", onPopState, { once: true });
		const container = renderHeader();
		const filesLink = Array.from(container.querySelectorAll<HTMLAnchorElement>("a")).find((link) => link.textContent === "Files");

		act(() => filesLink?.click());

		expect(window.location.search).toBe("");
		expect(onPopState).toHaveBeenCalledOnce();
	});
});
