import type { Declaration } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";

// Visual differentiation by point tier — small / mid / jackpot / match-winner.
// `pts` colors the "+N" figure and `text` the meld title + summary, so each
// tier's text matches its chrome.
const TIER_STYLES: Record<
  0 | 1 | 2 | 3,
  { eyebrow: string; border: string; tint: string; pts: string; text: string }
> = {
  0: {
    eyebrow: "var(--brass-deep)",
    border: "var(--border)",
    tint: "var(--surface)",
    pts: "var(--ink)",
    text: "var(--ink)",
  },
  1: {
    eyebrow: "var(--brass-deep)",
    border: "var(--border-2)",
    tint: "var(--surface)",
    pts: "var(--ink)",
    text: "var(--ink)",
  },
  2: {
    eyebrow: "var(--accent)",
    border: "var(--accent)",
    tint: "rgba(25,101,54,0.05)",
    pts: "var(--accent)",
    text: "var(--ink)",
  },
  // The instant-win bela (+1001) — dark-red treatment so the card that ends
  // the whole match on the spot reads apart from the green jackpot melds.
  3: {
    eyebrow: "var(--danger)",
    border: "var(--danger)",
    tint: "rgba(139,42,31,0.05)",
    pts: "var(--danger)",
    text: "var(--danger)",
  },
};

function MeldCard({ meld }: { meld: Declaration }) {
  const { ui } = useRules();
  const tier = TIER_STYLES[meld.tier];
  return (
    <div
      data-testid={`meld-${meld.id}`}
      style={{
        background: tier.tint,
        border: `1px solid ${tier.border}`,
        borderRadius: 14,
        padding: "14px 16px",
        display: "flex",
        flexDirection: "column",
        gap: 8,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <span
          className="font-display"
          style={{
            fontSize: 16,
            fontWeight: 600,
            color: tier.text,
            letterSpacing: -0.2,
            flex: "1 1 auto",
            minWidth: 0,
            lineHeight: 1.2,
          }}
        >
          {meld.name}
        </span>
        <span style={{ flexShrink: 0, display: "inline-flex", alignItems: "baseline", gap: 4 }}>
          <span
            className="font-display tabular-nums"
            style={{
              fontSize: 22,
              fontWeight: 700,
              color: tier.pts,
              letterSpacing: -0.5,
            }}
          >
            +{meld.pts}
          </span>
          <span style={{ fontSize: 11, color: "var(--ink-mute)" }}>{ui.pts}</span>
        </span>
      </div>
      <div
        className="font-mono"
        style={{
          fontSize: 10.5,
          letterSpacing: 2,
          textTransform: "uppercase",
          color: tier.eyebrow,
          fontWeight: 600,
        }}
      >
        {ui.meldKinds[meld.kind]}
      </div>
      <div style={{ fontSize: 13.5, lineHeight: 1.5, color: tier.text }}>{meld.summary}</div>
      <div style={{ fontSize: 12.5, lineHeight: 1.5, color: "var(--ink-dim)" }}>{meld.detail}</div>
    </div>
  );
}

/** Grid of every declaration (one column on phones, two on tablet+). */
export function MeldsGrid() {
  const { declarations } = useRules();
  return (
    <div data-testid="melds-grid" className="grid grid-cols-1 gap-3 md:grid-cols-2">
      {declarations.map((d) => (
        <MeldCard key={d.id} meld={d} />
      ))}
    </div>
  );
}
