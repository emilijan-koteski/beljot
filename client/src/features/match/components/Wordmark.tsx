/**
 * Top-center "Beljot.online" wordmark, intentionally not localized, with the suffix in brass.
 */
export function Wordmark() {
  return (
    <div
      className="pointer-events-none absolute top-5 left-1/2 z-9 hidden -translate-x-1/2 items-center gap-2 select-none md:flex"
      style={{
        color: "var(--ink)",
        fontSize: 14,
        letterSpacing: "0.18em",
      }}
      data-testid="wordmark"
    >
      <span className="font-semibold uppercase opacity-80">
        Beljot<span style={{ color: "var(--brass)" }}>.online</span>
      </span>
    </div>
  );
}
