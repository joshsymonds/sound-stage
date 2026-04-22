/* eslint-disable unicorn/no-document-cookie -- document.cookie is the standard API; CookieStore has poor browser support */
const COOKIE_NAME = "guest_name";
const MAX_AGE = 60 * 60 * 24 * 7; // 7 days

export function getGuestName(): string | null {
  const match = document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${COOKIE_NAME}=`));

  if (!match) return null;

  const value = decodeURIComponent(match.split("=")[1] ?? "");
  return value || null;
}

// Append the Secure attribute only when the page itself is served over
// HTTPS — dev (`vite dev` over http://localhost) needs the cookie to be
// readable, and Secure cookies on a non-HTTPS page are silently dropped.
function secureFlag(): string {
  return typeof location !== "undefined" && location.protocol === "https:" ? "; Secure" : "";
}

export function setGuestName(name: string): void {
  const trimmed = name.trim();
  document.cookie = `${COOKIE_NAME}=${encodeURIComponent(trimmed)}; path=/; max-age=${String(MAX_AGE)}; SameSite=Lax${secureFlag()}`;
}

export function clearGuestName(): void {
  // max-age=0 expires the cookie immediately. SameSite + Secure must match
  // the original write or some browsers won't accept the deletion.
  document.cookie = `${COOKIE_NAME}=; path=/; max-age=0; SameSite=Lax${secureFlag()}`;
}
