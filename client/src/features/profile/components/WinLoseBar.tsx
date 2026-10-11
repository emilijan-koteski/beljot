type WinLoseBarProps = {
  winPct: number;
};

/** Two-segment ratio bar: the win share in felt-green, the remaining loss share in translucent red. */
export function WinLoseBar({ winPct }: WinLoseBarProps) {
  const win = Math.max(0, Math.min(100, winPct));
  return (
    <div
      className="flex h-1.5 overflow-hidden rounded-full"
      style={{ background: "var(--surface-3)" }}
    >
      <span className="block h-full" style={{ width: `${win}%`, background: "var(--accent)" }} />
      <span
        className="block h-full"
        style={{ width: `${100 - win}%`, background: "rgba(139,42,31,0.18)" }}
      />
    </div>
  );
}
