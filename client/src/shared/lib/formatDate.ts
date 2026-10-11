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
 * Formats an ISO date with i18n month names and a locale template, avoiding Intl
 * gaps for MK and HR. Returns "" if `iso` cannot be parsed.
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

/** Locale-neutral 24-hour HH:MM for an ISO timestamp. Returns "" for an unparseable input. */
export function formatClockTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
