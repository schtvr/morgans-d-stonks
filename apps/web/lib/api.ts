import { clearToken, getToken } from "@/lib/auth";

/**
 * API base URL for fetches.
 * - If NEXT_PUBLIC_API_URL is set: call that origin directly (you must configure CORS on portfolio-api).
 * - Otherwise: same-origin `/api-go` (Next rewrites to PORTFOLIO_API_INTERNAL_URL) — works from any LAN host without CORS.
 */
export function apiBaseUrl(): string {
  const pub = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (pub) {
    return pub.replace(/\/$/, "");
  }
  if (typeof window !== "undefined") {
    return "/api-go";
  }
  return (
    process.env.PORTFOLIO_API_INTERNAL_URL?.replace(/\/$/, "") ??
    "http://127.0.0.1:8080"
  );
}

const base = () => apiBaseUrl();

export async function apiFetch(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(`${base()}${path}`, { ...init, headers });
  if (res.status === 401) {
    clearToken();
    if (typeof window !== "undefined") {
      window.location.href = "/login";
    }
    throw new Error("unauthorized");
  }
  return res;
}
