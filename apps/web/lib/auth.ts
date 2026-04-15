const COOKIE = "auth_token";

export function setToken(token: string) {
  const maxAge = 60 * 60 * 24;
  document.cookie = `${COOKIE}=${encodeURIComponent(token)}; path=/; max-age=${maxAge}; samesite=lax`;
}

export function getToken(): string | null {
  if (typeof document === "undefined") return null;
  const row = document.cookie.split("; ").find((c) => c.startsWith(`${COOKIE}=`));
  if (!row) return null;
  return decodeURIComponent(row.slice(COOKIE.length + 1));
}

export function clearToken() {
  document.cookie = `${COOKIE}=; path=/; max-age=0`;
}
