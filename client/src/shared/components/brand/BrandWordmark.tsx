import { cn } from "@/shared/lib/utils";

import { WORDMARK } from "./wordmarkPaths";

type BrandWordmarkProps = {
  height: number;
  suffix?: boolean;
  className?: string;
};

/**
 * The outlined BELJOT wordmark: the name takes the text colour, the .ONLINE suffix the deep brass token.
 * It drops 9.7% of its height so the capitals, not a box stretched by the J's descender, centre beside the chip.
 */
export function BrandWordmark({ height, suffix = true, className }: BrandWordmarkProps) {
  return (
    <svg
      viewBox={suffix ? WORDMARK.viewBox : WORDMARK.nameViewBox}
      height={height}
      width={height * (suffix ? WORDMARK.aspect : WORDMARK.nameAspect)}
      className={cn("translate-y-[9.7%]", className)}
      aria-hidden="true"
      focusable="false"
      data-testid="brand-wordmark"
    >
      <path d={WORDMARK.name} fill="currentColor" />
      {suffix && <path d={WORDMARK.suffix} style={{ fill: "var(--brass-deep)" }} />}
    </svg>
  );
}
