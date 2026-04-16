import { clearSessionMarker } from "@/lib/auth";

const CROSS_ORIGIN_TOKEN_KEY = "portfolio_api_bearer";

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

/** True when the configured public API origin differs from the dashboard origin (session cookies are not sent on fetch). */
export function isCrossOriginPublicAPI(): boolean {
  if (typeof window === "undefined") return false;
  const pub = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (!pub) return false;
  const u = pub.replace(/\/$/, "");
  if (!u.startsWith("http://") && !u.startsWith("https://")) return false;
  try {
    return new URL(u).origin !== window.location.origin;
  } catch {
    return false;
  }
}

function crossOriginBearer(): string | null {
  if (typeof window === "undefined") return null;
  return sessionStorage.getItem(CROSS_ORIGIN_TOKEN_KEY);
}

export function setCrossOriginBearerToken(token: string) {
  if (typeof window === "undefined") return;
  sessionStorage.setItem(CROSS_ORIGIN_TOKEN_KEY, token);
}

export function clearCrossOriginBearerToken() {
  if (typeof window === "undefined") return;
  sessionStorage.removeItem(CROSS_ORIGIN_TOKEN_KEY);
}

export async function apiFetch(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  const bearer = crossOriginBearer();
  if (bearer) headers.set("Authorization", `Bearer ${bearer}`);
  if (!headers.has("Content-Type") && init.body) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(`${base()}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });
  if (res.status === 401) {
    clearSessionMarker();
    clearCrossOriginBearerToken();
    if (typeof window !== "undefined") {
      window.location.href = "/login";
    }
    throw new Error("unauthorized");
  }
  return res;
}
