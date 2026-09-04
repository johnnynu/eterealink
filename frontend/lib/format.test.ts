import { describe, expect, it } from "vitest";
import { formatBytes, timeRemaining } from "./format";

describe("formatBytes", () => {
  it("uses readable binary units", () => {
    expect(formatBytes(500)).toBe("500 B");
    expect(formatBytes(1536)).toBe("1.50 KB");
    expect(formatBytes(15 * 1024 * 1024)).toBe("15.0 MB");
  });
});

describe("timeRemaining", () => {
  const now = Date.parse("2026-09-02T12:00:00Z");

  it("counts down and rounds partial minutes up", () => {
    expect(timeRemaining("2026-09-03T11:30:00Z", now)).toBe("23h 30m remaining");
    expect(timeRemaining("2026-09-02T12:00:01Z", now)).toBe("1m remaining");
  });

  it("marks an elapsed timestamp as expired", () => {
    expect(timeRemaining("2026-09-02T12:00:00Z", now)).toBe("Expired");
  });
});
