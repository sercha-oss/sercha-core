import type { NextConfig } from "next";
import packageJson from "./package.json";

const nextConfig: NextConfig = {
  output: "export",
  images: {
    unoptimized: true,
  },
  trailingSlash: true,
  env: {
    APP_VERSION: packageJson.version,
  },
  // Rewrites only work in dev mode (npm run dev), ignored for static export
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
      {
        source: "/health/",
        destination: "http://localhost:8080/health",
      },
      {
        source: "/health",
        destination: "http://localhost:8080/health",
      },
      {
        source: "/ready/",
        destination: "http://localhost:8080/ready",
      },
      {
        source: "/ready",
        destination: "http://localhost:8080/ready",
      },
      {
        source: "/version/",
        destination: "http://localhost:8080/version",
      },
      {
        source: "/version",
        destination: "http://localhost:8080/version",
      },
    ];
  },
};

export default nextConfig;
