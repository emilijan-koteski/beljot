import { RuleBlock } from "@/features/rules/components/RuleBlock";
import type { RuleSection } from "@/features/rules/content/types";

type Props = {
  idx: number;
  section: RuleSection;
  registerRef: (id: string, el: HTMLElement | null) => void;
};

/** One numbered chapter (§01…§06): header + content blocks. */
export function Chapter({ idx, section, registerRef }: Props) {
  return (
    <section
      ref={(el) => registerRef(section.id, el)}
      id={section.id}
      style={{
        scrollMarginTop: 84,
        display: "flex",
        flexDirection: "column",
        gap: 18,
        padding: "36px 0 32px",
        borderTop: idx === 0 ? "none" : "1px solid var(--border)",
      }}
    >
      <header style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        <div
          className="font-mono"
          style={{
            fontSize: 10,
            letterSpacing: "0.18em",
            textTransform: "uppercase",
            color: "var(--brass-deep)",
            fontWeight: 600,
          }}
        >
          {String(idx + 1).padStart(2, "0")} &nbsp; {section.label}
        </div>
        <h2
          className=""
          style={{
            margin: 0,
            fontSize: 28,
            fontWeight: 700,
            letterSpacing: "-0.025em",
            lineHeight: 1.2,
            color: "var(--ink)",
          }}
        >
          {section.title}
        </h2>
        <p
          style={{
            margin: "4px 0 0",
            fontSize: 16,
            lineHeight: 1.6,
            color: "var(--ink-dim)",
            maxWidth: 640,
          }}
        >
          {section.lede}
        </p>
      </header>

      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        {section.blocks.map((b, i) => (
          <RuleBlock key={i} block={b} />
        ))}
      </div>
    </section>
  );
}
