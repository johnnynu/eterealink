import type { NextConfig } from "next";

const apiBaseURL = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

const nextConfig: NextConfig = {
  output: "standalone",
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiBaseURL}/:path*`,
      },
    ];
  },
};

export default nextConfig;
