import { socialCard, socialCardSize } from "@/lib/social-card";
import { shareLinkDescription, shareLinkTitle } from "@/lib/share-link-metadata";
import { isTransferResult, type ShareResult } from "@/lib/types";

export const alt = "Aurea Link file share";
export const size = socialCardSize;
export const contentType = "image/png";

export default async function ShareOpenGraphImage({ params }: { params: Promise<{ code: string }> }) {
  const { code } = await params;
  const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

  try {
    const response = await fetch(`${apiBaseURL}/v1/shares/${encodeURIComponent(code)}`, { cache: "no-store" });
    if (!response.ok) throw new Error("share unavailable");
    const result = await response.json() as ShareResult;
    const multipleFiles = isTransferResult(result) && result.files.length > 1;
    return socialCard({
      eyebrow: multipleFiles ? "Aurea Link transfer" : "Aurea Link file",
      title: shareLinkTitle(result),
      description: shareLinkDescription(result),
    });
  } catch {
    return socialCard({
      eyebrow: "Aurea Link file",
      title: "A file was shared with you.",
      description: "Open the link to view this private handoff. No account required.",
    });
  }
}
