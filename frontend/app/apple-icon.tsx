import { ImageResponse } from "next/og";
import { aureaLogoDataURL } from "@/lib/social-card";

export const size = { width: 180, height: 180 };
export const contentType = "image/png";

export default function AppleIcon() {
  return new ImageResponse(
    <div style={{
      width: "100%",
      height: "100%",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      background: "#e2efe8",
    }}>
      <img src={aureaLogoDataURL} width={128} height={128} alt="" />
    </div>,
    { ...size },
  );
}
