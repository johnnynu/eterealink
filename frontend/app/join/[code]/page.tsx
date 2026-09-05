import type { Metadata } from "next";
import { FolderInviteView } from "@/components/folder-invite-view";

export const metadata: Metadata = {
  title: "Folder invitation",
  robots: { index: false, follow: false },
};

export default async function FolderInvitePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  return <FolderInviteView code={code} />;
}
