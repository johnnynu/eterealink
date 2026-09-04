import type { Metadata } from "next";
import { ShareView } from "@/components/share-view";

export const metadata: Metadata = {
  title: "Shared file",
  robots: { index: false, follow: false },
};

export default async function SharePage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  return (
    <div className="share-page">
      <div className="ambient ambient-one" />
      <div className="share-intro">
        <p className="hero-kicker"><span /> Private handoff</p>
        <h2>A file, sent simply.</h2>
        <p>No account required. This temporary link is checked each time it opens.</p>
      </div>
      <ShareView code={code} />
    </div>
  );
}
