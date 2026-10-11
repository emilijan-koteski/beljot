import { useSyncExternalStore } from "react";

import { ageCompact, fromNow } from "@/shared/lib/relativeTime";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";

type Props = {
  iso: string | number | Date;
  variant?: "verbose" | "compact";
  className?: string;
  title?: string;
};

/** Returns the ISO string for a timestamp, or "" when it is missing or unparseable so callers never throw. */
function toIsoOrEmpty(iso: string | number | Date | null | undefined): string {
  if (typeof iso === "string") return iso;
  if (iso == null) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toISOString();
}

/** Renders a localized relative timestamp that re-renders on a single shared 30s tick. */
export function RelativeTime({ iso, variant = "verbose", className, title }: Props) {
  useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  const fullIso = toIsoOrEmpty(iso);
  if (!fullIso) {
    return <time className={className} />;
  }

  const text = variant === "compact" ? ageCompact(iso) : fromNow(iso);

  return (
    <time dateTime={fullIso} title={title ?? fullIso} className={className}>
      {text}
    </time>
  );
}
