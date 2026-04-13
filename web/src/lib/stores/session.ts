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

export function setGuestName(name: string): void {
  const trimmed = name.trim();
  document.cookie = `${COOKIE_NAME}=${encodeURIComponent(trimmed)}; path=/; max-age=${String(MAX_AGE)}; SameSite=Lax`;
}
