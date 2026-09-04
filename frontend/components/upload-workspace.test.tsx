import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { UploadWorkspace } from "./upload-workspace";

const mounted: Array<{ container: HTMLDivElement; unmount: () => void }> = [];

afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.unmount());
    view.container.remove();
  }
});

describe("UploadWorkspace", () => {
  it("shows the selected file and the upload action after input selection", () => {
    const container = document.createElement("div");
    document.body.append(container);
    const root = createRoot(container);
    mounted.push({ container, unmount: () => root.unmount() });

    act(() => root.render(<UploadWorkspace />));
    const input = container.querySelector<HTMLInputElement>("#anonymous-files");
    expect(input).not.toBeNull();

    Object.defineProperty(input, "files", {
      configurable: true,
      value: [new File(["hello"], "hello.txt", { type: "text/plain" })],
    });
    act(() => input?.dispatchEvent(new Event("change", { bubbles: true })));

    expect(container.textContent).toContain("hello.txt");
    expect(container.textContent).toContain("1 file");
    expect(container.textContent).toContain("Create 24-hour link");
  });

  it("rejects more than ten files", () => {
    const container = document.createElement("div");
    document.body.append(container);
    const root = createRoot(container);
    mounted.push({ container, unmount: () => root.unmount() });

    act(() => root.render(<UploadWorkspace />));
    const input = container.querySelector<HTMLInputElement>("#anonymous-files");
    Object.defineProperty(input, "files", {
      configurable: true,
      value: Array.from({ length: 11 }, (_, index) => new File(["x"], `file-${index}.txt`)),
    });
    act(() => input?.dispatchEvent(new Event("change", { bubbles: true })));

    expect(container.textContent).toContain("Choose no more than 10 files per transfer.");
    expect(container.querySelector<HTMLButtonElement>(".primary-button")?.disabled).toBe(true);
  });
});
