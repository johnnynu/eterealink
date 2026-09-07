import type { Metadata } from "next";
import { ShareView } from "@/components/share-view";
import { shareLinkDescription, shareLinkTitle } from "@/lib/share-link-metadata";
import type { ShareResult } from "@/lib/types";

function shareMetadata(title: string, description: string): Metadata {
	return {
		title,
		description,
		robots: { index: false, follow: false },
		openGraph: {
			title,
			description,
			type: "website",
			siteName: "Eterealink",
			images: [{ url: "/opengraph-image", width: 1200, height: 630, alt: "Eterealink" }],
		},
		twitter: {
			card: "summary_large_image",
			title,
			description,
			images: ["/opengraph-image"],
		},
	};
}

const fallbackMetadata = shareMetadata("Shared file", "A file was shared with you through Eterealink.");

export async function generateMetadata({ params }: { params: Promise<{ code: string }> }): Promise<Metadata> {
	const { code } = await params;
	const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");
	try {
		const response = await fetch(`${apiBaseURL}/v1/shares/${encodeURIComponent(code)}`, { cache: "no-store" });
		if (!response.ok) return fallbackMetadata;
		const result = await response.json() as ShareResult;
		const title = shareLinkTitle(result);
		const description = shareLinkDescription(result);
		return shareMetadata(title, description);
	} catch {
		return fallbackMetadata;
	}
}

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
