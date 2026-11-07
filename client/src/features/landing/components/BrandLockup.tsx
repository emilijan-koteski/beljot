import { Link } from "react-router";

import { WORDMARK_PER_FONT_SIZE } from "@/shared/components/brand/brandSizes";
import { BrandWordmark } from "@/shared/components/brand/BrandWordmark";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

type BrandLockupProps = {
  size?: number;
  wordmarkSize?: number;
  className?: string;
  showSuffix?: boolean;
};

// Logo tile and "Beljot" wordmark linking home: /lobby when signed in, / otherwise.
export function BrandLockup({
  size = 32,
  wordmarkSize = 24,
  className,
  showSuffix = false,
}: BrandLockupProps) {
  const to = useAuthStore((s) => s.user) ? "/lobby" : "/";
  return (
    <Link
      to={to}
      aria-label="Beljot"
      className={cn(
        "flex items-center gap-2.5 rounded-md transition-opacity hover:opacity-80 focus-visible:ring-ring/70 focus-visible:outline-none focus-visible:ring-3",
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
      <span className="text-ink inline-flex">
        <span className="sr-only">Beljot{showSuffix ? ".online" : ""}</span>
        <BrandWordmark
          height={Math.round(wordmarkSize * WORDMARK_PER_FONT_SIZE)}
          suffix={showSuffix}
        />
      </span>
    </Link>
  );
}
