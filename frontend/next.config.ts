import type { NextConfig } from "next";

const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");
const firebaseProjectID = process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID?.trim();

const nextConfig: NextConfig = {
  output: "standalone",
  allowedDevOrigins: ["127.0.0.1", "localhost"],
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
