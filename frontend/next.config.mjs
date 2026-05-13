/** @type {import('next').NextConfig} */
const internal =
  process.env.PORTFOLIO_API_INTERNAL_URL ?? "http://127.0.0.1:8080";

const nextConfig = {
  // workerThreads caused static generation to call revalidateTag IPC with an
  // undefined port (http://localhost:undefined) during `next build`.
  experimental: {
    webpackBuildWorker: false,
    workerThreads: false,
  },
  async rewrites() {
    const base = internal.replace(/\/$/, "");
    return [
      {
        source: "/api-go/:path*",
        destination: `${base}/:path*`,
      },
    ];
  },
};

export default nextConfig;
