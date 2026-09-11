import type { Metadata } from "next";
import { FolderInviteView } from "@/components/folder-invite-view";
import type { FolderInvitePreview } from "@/lib/types";

function inviteMetadata(title: string, description: string, path: string): Metadata {
  const image = `${path}/opengraph-image`;
  return {
    title,
    description,
    robots: { index: false, follow: false },
    openGraph: {
      url: path,
      title,
      description,
      type: "website",
      siteName: "Aurea Link",
      images: [{ url: image, width: 1200, height: 630, alt: "Aurea Link folder invitation" }],
    },
    twitter: { card: "summary_large_image", title, description, images: [image] },
  };
}

export async function generateMetadata({ params }: { params: Promise<{ code: string }> }): Promise<Metadata> {
  const { code } = await params;
  const path = `/join/${encodeURIComponent(code)}`;
  const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");
  try {
    const response = await fetch(`${apiBaseURL}/v1/folder-invites/${encodeURIComponent(code)}`, { cache: "no-store" });
    if (!response.ok) throw new Error("invite unavailable");
    const { invite } = await response.json() as { invite: FolderInvitePreview };
    return inviteMetadata(
      `${invite.ownerName} invited you to “${invite.folderName}”`,
      `Join as a ${invite.role === "CONTRIBUTOR" ? "contributor" : "viewer"} through Aurea Link.`,
      path,
    );
  } catch {
    return inviteMetadata("Folder invitation", "You were invited to collaborate in Aurea Link.", path);
  }
}

export default async function FolderInvitePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  return <FolderInviteView code={code} />;
}
