export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes < 1024) return `${bytes} B`;

  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unit = units[0];

  for (let index = 1; index < units.length && value >= 1024; index += 1) {
    value /= 1024;
    unit = units[index];
  }

  return `${value >= 10 ? value.toFixed(1) : value.toFixed(2)} ${unit}`;
}

export function formatExpiry(value?: string): string {
  if (!value) return "No expiration";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function formatFileType(mimeType: string, originalName: string): string {
  const normalized = mimeType.toLowerCase();
  if (normalized === "application/pdf") return "PDF document";
  if (normalized.startsWith("image/")) return "Image";
  if (normalized.startsWith("video/")) return "Video";
  if (normalized.startsWith("audio/")) return "Audio";
  if (normalized.startsWith("text/")) return "Text document";
  if (normalized.includes("zip")) return "ZIP archive";
  if (normalized.includes("word")) return "Word document";
  if (normalized.includes("spreadsheet") || normalized.includes("excel")) return "Spreadsheet";
  if (normalized.includes("presentation") || normalized.includes("powerpoint")) return "Presentation";

  const extension = originalName.split(".").pop();
  if (extension && extension !== originalName && extension.length <= 8) return `${extension.toUpperCase()} file`;
  return "File";
}

export function formatRelativeDate(value?: string, now = Date.now()): string {
  if (!value) return "Unknown date";
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return "Unknown date";
  const elapsed = Math.max(0, now - timestamp);
  const minutes = Math.floor(elapsed / 60_000);
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(timestamp);
}

export function timeRemaining(value?: string, now = Date.now()): string {
  if (!value) return "Available";
  const remaining = new Date(value).getTime() - now;
  if (!Number.isFinite(remaining) || remaining <= 0) return "Expired";

  const totalMinutes = Math.max(1, Math.ceil(remaining / 60_000));
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;

  if (days > 0) return `${days}d ${hours}h remaining`;
  if (hours > 0) return `${hours}h ${minutes}m remaining`;
  return `${minutes}m remaining`;
}
