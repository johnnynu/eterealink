import type { NextConfig } from "next";

const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");
const firebaseProjectID = process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID?.trim();

const securityHeaders = [
  { key: "Content-Security-Policy", value: "base-uri 'self'; frame-ancestors 'none'; object-src 'none'" },
  { key: "Permissions-Policy", value: "camera=(), geolocation=(), microphone=()" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Strict-Transport-Security", value: "max-age=31536000" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
];

const nextConfig: NextConfig = {
  output: "standalone",
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiBaseURL}/:path*`,
      },
      ...(firebaseProjectID ? [{
        source: "/__/auth/:path*",
        destination: `https://${firebaseProjectID}.firebaseapp.com/__/auth/:path*`,
      }] : []),
    ];
  },
};

export default nextConfig;
