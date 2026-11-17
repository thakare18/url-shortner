import type { NextConfig } from "next";

// Feature: same-origin proxy connects Next.js to Go; the browser needs no CORS setup.
const backend = process.env.BACKEND_URL || "http://127.0.0.1:8080";
const config: NextConfig = {
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${backend}/api/:path*` },
      { source: "/s/:id", destination: `${backend}/redirect/:id` },
    ];
  },
};
export default config;
