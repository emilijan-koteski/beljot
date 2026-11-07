import { WORDMARK } from "./wordmarkPaths";

type BrandWordmarkProps = {
  height: number;
  suffix?: boolean;
  className?: string;
};

/** The outlined BELJOT wordmark: the name takes the text colour, the .ONLINE suffix the deep brass token. */
export function BrandWordmark({ height, suffix = true, className }: BrandWordmarkProps) {
  return (
    <svg
      viewBox={suffix ? WORDMARK.viewBox : WORDMARK.nameViewBox}
      height={height}
      width={height * (suffix ? WORDMARK.aspect : WORDMARK.nameAspect)}
      className={className}
      aria-hidden="true"
      focusable="false"
      data-testid="brand-wordmark"
    >
      <path d={WORDMARK.name} fill="currentColor" />
      {suffix && <path d={WORDMARK.suffix} style={{ fill: "var(--brass-deep)" }} />}
    </svg>
  );
}
