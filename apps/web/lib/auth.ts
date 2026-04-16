/** HttpOnly session is set by portfolio-api; this flag only drives middleware (non-secret). */
const SESSION_FLAG = "session_ok";

export function markSessionPresent() {
  if (typeof document === "undefined") return;
  const maxAge = 60 * 60 * 24;
  document.cookie = `${SESSION_FLAG}=1; path=/; max-age=${maxAge}; samesite=lax`;
}

export function clearSessionMarker() {
  if (typeof document === "undefined") return;
  document.cookie = `${SESSION_FLAG}=; path=/; max-age=0`;
}
