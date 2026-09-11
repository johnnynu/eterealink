import { ImageResponse } from "next/og";

export const socialCardSize = { width: 1200, height: 630 };

export const aureaLogoDataURL = `data:image/svg+xml,${encodeURIComponent(`
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256">
  <path fill="#1f7458" opacity=".2" d="M138.54 149.46C106.62 96.25 149.18 43.05 239.63 48.37 245 138.82 191.75 181.39 138.54 149.46ZM16.26 88.26c-3.8 64.61 34.21 95 72.21 72.21C111.27 122.47 80.87 84.46 16.26 88.26Z"/>
  <path fill="#1f7458" d="M247.63 47.89a8 8 0 0 0-7.52-7.52c-51.76-3-93.32 12.74-111.18 42.22-11.8 19.48-11.78 43.16-.16 65.74a71.37 71.37 0 0 0-14.17 26.95L98.33 159c7.82-16.33 7.52-33.36-1-47.49C84.09 89.73 53.62 78 15.79 80.27a8 8 0 0 0-7.52 7.52c-2.23 37.83 9.46 68.3 31.25 81.5A45.82 45.82 0 0 0 63.44 176 54.58 54.58 0 0 0 87 170.33l25 25V224a8 8 0 0 0 16 0v-29.49a55.61 55.61 0 0 1 12.27-35 73.91 73.91 0 0 0 33.31 8.4 60.9 60.9 0 0 0 31.83-8.86C234.89 141.21 250.67 99.65 247.63 47.89ZM86.06 146.74l-24.41-24.4a8 8 0 0 0-11.31 11.31l24.41 24.41c-9.61 3.18-18.93 2.39-26.94-2.46C32.47 146.31 23.79 124.32 24 96c28.31-.25 50.31 8.47 59.6 23.81C88.45 127.82 89.24 137.14 86.06 146.74Zm111.06-1.36c-13.4 8.11-29.15 8.73-45.15 2l53.69-53.7a8 8 0 0 0-11.31-11.32L140.65 136c-6.76-16-6.15-31.76 2-45.15 13.94-23 47-35.8 89.33-34.83C232.94 98.34 220.14 131.44 197.12 145.38Z"/>
</svg>`)}`;

function compact(value: string, maximum: number) {
  const normalized = value.replace(/\s+/g, " ").trim();
  return normalized.length > maximum ? `${normalized.slice(0, maximum - 1).trimEnd()}…` : normalized;
}

export function socialCard({ eyebrow, title, description }: {
  eyebrow: string;
  title: string;
  description: string;
}) {
  return new ImageResponse(
    <div style={{
      width: "100%",
      height: "100%",
      display: "flex",
      alignItems: "stretch",
      padding: "68px",
      background: "#f7f5ef",
      color: "#17231e",
      fontFamily: "sans-serif",
    }}>
      <div style={{
        width: "100%",
        display: "flex",
        alignItems: "center",
        padding: "58px 64px",
        border: "2px solid #d9ded9",
        borderRadius: 44,
        background: "#fffdf8",
      }}>
        <div style={{
          width: 174,
          height: 174,
          flexShrink: 0,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          borderRadius: 46,
          background: "#e2efe8",
        }}>
          {/* ImageResponse renders this embedded SVG directly into the generated PNG. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={aureaLogoDataURL} width={126} height={126} alt="" />
        </div>
        <div style={{ display: "flex", flexDirection: "column", marginLeft: 54, minWidth: 0 }}>
          <div style={{ display: "flex", alignItems: "center", color: "#1f7458", fontSize: 25, fontWeight: 700, letterSpacing: "1px", textTransform: "uppercase" }}>
            {compact(eyebrow, 42)}
          </div>
          <div style={{ marginTop: 16, fontSize: 58, lineHeight: 1.06, fontWeight: 750, letterSpacing: "-2.6px" }}>
            {compact(title, 72)}
          </div>
          <div style={{ marginTop: 18, color: "#52635b", fontSize: 27, lineHeight: 1.3 }}>
            {compact(description, 110)}
          </div>
        </div>
      </div>
    </div>,
    { ...socialCardSize },
  );
}
