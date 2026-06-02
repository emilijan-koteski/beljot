import { Link } from "react-router";

import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

// Logo tile (the shared /beljot-logo.svg, same asset the app TopBar uses) +
// "Beljot" wordmark. The wordmark uses `text-ink`, so it auto-flips to light
// inside a `.felt-surface` and stays dark on parchment. Clicking it goes home —
// "/lobby" when signed in, the marketing landing ("/") otherwise.

type BrandLockupProps = {
  size?: number;
  wordmarkSize?: number;
  className?: string;
};

export function BrandLockup({ size = 34, wordmarkSize = 22, className }: BrandLockupProps) {
  const to = useAuthStore((s) => s.user) ? "/lobby" : "/";
  return (
    <Link
      to={to}
      aria-label="Beljot"
      className={cn(
        "flex items-center gap-2.5 rounded-md transition-opacity hover:opacity-80 focus-visible:ring-accent/50 focus-visible:outline-none focus-visible:ring-2",
        className,
      )}
    >
      <img
        src="/beljot-logo.svg"
        alt=""
        aria-hidden="true"
        className="shrink-0"
        style={{ width: size, height: size }}
      />
      <span
        className="font-display text-ink font-semibold tracking-[-0.3px]"
        style={{ fontSize: wordmarkSize }}
      >
        Beljot
      </span>
    </Link>
  );
}
