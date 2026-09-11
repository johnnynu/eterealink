import { socialCard, socialCardSize } from "@/lib/social-card";

export const alt = "Aurea Link";
export const size = socialCardSize;
export const contentType = "image/png";

export default function OpenGraphImage() {
	return socialCard({
		eyebrow: "Aurea Link",
		title: "Files shared simply.",
		description: "Private file sharing with temporary links and no account required.",
	});
}
