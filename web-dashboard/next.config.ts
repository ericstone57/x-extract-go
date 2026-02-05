import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Enable static export for embedding in Go binary
  output: "export",

  // Output to build directory instead of out
  distDir: "build",

  // Disable image optimization for static export
  images: {
    unoptimized: true,
  },

  // Set base path to empty since we're serving from Go server root
  basePath: "",

  // Trailing slash for better compatibility
  trailingSlash: true,

  // Suppress error overlay for expected proxy errors when backend is offline
  // This prevents the Next.js error overlay from showing when the Go server is not running
  reactStrictMode: true,
};

export default nextConfig;

