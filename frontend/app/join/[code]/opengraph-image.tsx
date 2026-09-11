import { socialCard, socialCardSize } from "@/lib/social-card";
import type { FolderInvitePreview } from "@/lib/types";

export const alt = "Aurea Link folder invitation";
export const size = socialCardSize;
export const contentType = "image/png";

export default async function InviteOpenGraphImage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

  try {
    const response = await fetch(`${apiBaseURL}/v1/folder-invites/${encodeURIComponent(code)}`, { cache: "no-store" });
    if (!response.ok) throw new Error("invite unavailable");
    const { invite } = await response.json() as { invite: FolderInvitePreview };
    return socialCard({
      eyebrow: "Aurea Link invitation",
      title: `${invite.ownerName} invited you.`,
      description: `Join “${invite.folderName}” as a ${invite.role === "CONTRIBUTOR" ? "contributor" : "viewer"}.`,
    });
  } catch {
    return socialCard({
      eyebrow: "Aurea Link invitation",
      title: "You were invited to collaborate.",
      description: "Open the link to view this folder invitation.",
    });
  }
}
