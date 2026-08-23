import type { TFunction } from "i18next";

const MONTH_KEYS = [
  "jan",
  "feb",
  "mar",
  "apr",
  "may",
  "jun",
  "jul",
  "aug",
  "sep",
  "oct",
  "nov",
  "dec",
] as const;

/**
 * Format an ISO date string using i18n month names + a locale-specific format
 * template. Independent of the runtime's Intl/CLDR coverage so MK and HR
 * display correctly even in environments whose Intl data silently falls back
 * to en-US for those locales (observed on some Windows browser builds).
 *
 * Returns "" if `iso` cannot be parsed; callers gate display with a truthy
 * check ({formattedDate && <p>…</p>}), matching the behaviour of the
 * try/catch'd Intl.DateTimeFormat path this helper replaced.
 */
export function formatLocalizedDate(
  iso: string,
  t: TFunction,
  variant: "long" | "short" = "long",
): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const monthKey = MONTH_KEYS[date.getMonth()];
  const month = t(`date.${variant === "long" ? "monthLong" : "monthShort"}.${monthKey}`);
  return t("date.format", {
    month,
    day: date.getDate(),
    year: date.getFullYear(),
  });
}

/**
 * 24-hour HH:MM for an ISO timestamp, locale-neutral (matches the match-row
 * clock in the design). Returns "" for an unparseable input.
 *
 * Shared rather than profile-local because the match-stats card that renders it
 * is mounted from the profile, the room lobby, and the end-of-match dialog.
 */
export function formatClockTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
