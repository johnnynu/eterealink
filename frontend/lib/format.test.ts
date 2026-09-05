import { describe, expect, it } from "vitest";
import { formatBytes, formatFileType, formatRelativeDate, timeRemaining } from "./format";

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

describe("file metadata formatting", () => {
  it("uses human-readable file categories", () => {
    expect(formatFileType("application/pdf", "report.pdf")).toBe("PDF document");
    expect(formatFileType("image/webp", "photo.webp")).toBe("Image");
    expect(formatFileType("application/octet-stream", "backup.tar")).toBe("TAR file");
  });

  it("uses concise relative upload dates", () => {
    const now = Date.parse("2026-09-03T12:00:00Z");
    expect(formatRelativeDate("2026-09-03T11:58:00Z", now)).toBe("2m ago");
    expect(formatRelativeDate("2026-09-01T12:00:00Z", now)).toBe("2d ago");
  });
});
