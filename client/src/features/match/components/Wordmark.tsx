/**
 * Top-center wordmark — "Beljot.online" in the table's serif tone.
 *
 * The brand is fixed (not localized) per design intent. The `.online` suffix
 * is rendered in the brass accent so the eye lands on the domain.
 */
export function Wordmark() {
  return (
    <div
      className="pointer-events-none absolute top-5 left-1/2 -translate-x-1/2 z-9 flex items-center gap-2 select-none"
      style={{
        fontFamily: "var(--font-body)",
        color: "var(--ink)",
        fontSize: 13,
        letterSpacing: 2,
      }}
      data-testid="wordmark"
    >
      <span className="font-semibold uppercase opacity-80">
        Beljot<span style={{ color: "var(--brass)" }}>.online</span>
      </span>
    </div>
  );
}
