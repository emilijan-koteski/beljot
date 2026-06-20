import "dayjs/locale/en";
import "dayjs/locale/hr";
import "dayjs/locale/mk";
import "dayjs/locale/sr";

import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import updateLocale from "dayjs/plugin/updateLocale";

dayjs.extend(relativeTime);
dayjs.extend(updateLocale);

// Threshold table — when "just now" stops, when minutes/hours/days kick in.
// Tightened so "just now" covers the natural "few seconds" window without the
// awkward dayjs default "a few seconds ago".
dayjs.updateLocale("en", {
  relativeTime: {
    future: "in %s",
    past: "%s ago",
    s: "just now",
    m: "1 minute",
    mm: "%d minutes",
    h: "1 hour",
    hh: "%d hours",
    d: "1 day",
    dd: "%d days",
    M: "1 month",
    MM: "%d months",
    y: "1 year",
    yy: "%d years",
  },
});

dayjs.updateLocale("hr", {
  relativeTime: {
    future: "za %s",
    past: "prije %s",
    s: "upravo sad",
    m: "1 minutu",
    mm: "%d minuta",
    h: "1 sat",
    hh: "%d sati",
    d: "1 dan",
    dd: "%d dana",
    M: "1 mjesec",
    MM: "%d mjeseci",
    y: "1 godinu",
    yy: "%d godina",
  },
});

dayjs.updateLocale("sr", {
  relativeTime: {
    future: "za %s",
    past: "pre %s",
    s: "upravo sad",
    m: "1 minut",
    mm: "%d minuta",
    h: "1 sat",
    hh: "%d sati",
    d: "1 dan",
    dd: "%d dana",
    M: "1 mesec",
    MM: "%d meseci",
    y: "1 godinu",
    yy: "%d godina",
  },
});

dayjs.updateLocale("mk", {
  relativeTime: {
    future: "за %s",
    past: "пред %s",
    s: "сега",
    m: "1 минута",
    mm: "%d минути",
    h: "1 час",
    hh: "%d часа",
    d: "1 ден",
    dd: "%d дена",
    M: "1 месец",
    MM: "%d месеци",
    y: "1 година",
    yy: "%d години",
  },
});

// dayjs ships Serbian-Latin as `sr` and Serbian-Cyrillic as `sr-cyrl`. The
// project's `sr.json` is Latin-script copy, so the default mapping is a
// straight pass-through.
export function applyDayjsLocale(lang: string | undefined): void {
  const short = (lang ?? "en").toLowerCase().split("-")[0] ?? "en";
  dayjs.locale(short);
}

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;

/**
 * Localized relative time. "just now" / "5 minutes ago" / "in 3 hours".
 * Falls back to a localized absolute date (e.g. "May 3, 2026") when the
 * timestamp is more than 7 days away — at that point "X days ago" stops
 * communicating anything useful.
 */
export function fromNow(iso: string | number | Date, now: number = Date.now()): string {
  const d = dayjs(iso);
  if (!d.isValid()) return "";
  const diff = Math.abs(now - d.valueOf());
  if (diff > SEVEN_DAYS_MS) {
    return d.format("ll"); // localized short date — "May 3, 2026" / "3. svi 2026."
  }
  return d.from(now);
}

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

/**
 * Compact age string — "now" / "5m" / "2h" / "3d" / falls through to fromNow
 * for older values. Used for room age chips where a long string would blow
 * out the meta row.
 */
export function ageCompact(iso: string | number | Date, now: number = Date.now()): string {
  const d = dayjs(iso);
  if (!d.isValid()) return "";
  const diff = now - d.valueOf();
  if (diff < MINUTE_MS) {
    // Localized "just now" — read the active dayjs locale's `s` token (set in
    // the updateLocale calls above: en "just now", mk "сега", hr/sr
    // "upravo sad") so the compact form is localized, not a hardcoded "now".
    const justNow = dayjs.Ls[dayjs.locale()]?.relativeTime?.s;
    return typeof justNow === "string" ? justNow : "now";
  }
  if (diff < HOUR_MS) return `${Math.floor(diff / MINUTE_MS)}m`;
  if (diff < DAY_MS) return `${Math.floor(diff / HOUR_MS)}h`;
  if (diff < SEVEN_DAYS_MS) return `${Math.floor(diff / DAY_MS)}d`;
  return d.format("ll");
}
