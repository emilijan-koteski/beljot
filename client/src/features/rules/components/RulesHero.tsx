import type { Fact as FactData } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";

function Fact({ label, value, caption }: FactData) {
  return (
    <div
      style={{
        flex: "1 1 140px",
        minWidth: 140,
        padding: "12px 14px",
        background: "var(--surface)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-lg)",
        display: "flex",
        flexDirection: "column",
        gap: 2,
      }}
    >
      <span
        className="font-mono"
        style={{
          fontSize: 10,
          letterSpacing: 2,
          textTransform: "uppercase",
          color: "var(--brass-deep)",
          fontWeight: 700,
        }}
      >
        {label}
      </span>
      <span
        className="font-display tabular-nums"
        style={{
          fontSize: 26,
          fontWeight: 700,
          color: "var(--ink)",
          letterSpacing: -0.6,
          lineHeight: 1.1,
        }}
      >
        {value}
      </span>
      <span style={{ fontSize: 12, color: "var(--ink-dim)" }}>{caption}</span>
    </div>
  );
}

/** Page intro: eyebrow, title, lede, and the quick-facts row. */
export function RulesHero() {
  const { ui } = useRules();
  return (
    <header style={{ display: "flex", flexDirection: "column", gap: 18, paddingBottom: 28 }}>
      <div
        className="font-mono"
        style={{
          fontSize: 11,
          letterSpacing: 2.6,
          textTransform: "uppercase",
          color: "var(--brass-deep)",
          fontWeight: 600,
        }}
      >
        {ui.heroEyebrow}
      </div>

      <h1
        className="font-display"
        style={{
          margin: 0,
          fontSize: 48,
          fontWeight: 700,
          letterSpacing: -1.4,
          lineHeight: 1.05,
          color: "var(--ink)",
        }}
      >
        {ui.heroTitle}
      </h1>

      <p
        style={{ margin: 0, fontSize: 17, lineHeight: 1.6, color: "var(--ink-dim)", maxWidth: 640 }}
      >
        {ui.heroIntro}
      </p>

      <div style={{ display: "flex", gap: 10, flexWrap: "wrap", paddingTop: 4 }}>
        {ui.facts.map((f, i) => (
          <Fact key={i} label={f.label} value={f.value} caption={f.caption} />
        ))}
      </div>
    </header>
  );
}
