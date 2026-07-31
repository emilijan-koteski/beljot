import { useTranslation } from "react-i18next";

import { CardLadder } from "@/features/rules/components/CardLadder";
import { MeldsGrid } from "@/features/rules/components/MeldsGrid";
import type { RuleBlock as RuleBlockType } from "@/features/rules/content/types";
import { useRules } from "@/features/rules/RulesContext";
import { HONOR_TIER_BANDS, HONOR_TIER_COLOR, type HonorTier } from "@/shared/lib/honor";

function NoteBlock({ text }: { text: string }) {
  const { ui } = useRules();
  return (
    <div
      style={{
        padding: "12px 14px",
        background: "var(--brass-soft)",
        border: "1px dashed var(--border-2)",
        borderRadius: 10,
        display: "flex",
        alignItems: "flex-start",
        gap: 10,
        fontSize: 12.5,
        lineHeight: 1.55,
        color: "var(--ink-dim)",
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
          flexShrink: 0,
          paddingTop: 1,
        }}
      >
        {ui.noteLabel}
      </span>
      <span>{text}</span>
    </div>
  );
}

/** "95-100" / "85-94" … derived from the single client copy of the tier bands. */
function tierRange(tier: HonorTier): string {
  const band = HONOR_TIER_BANDS.find((b) => b.tier === tier);
  if (!band) return "";
  return `${band.from}-${band.to === 100 ? 100 : band.to - 1}`;
}

/**
 * The honour-tier ladder. Names come from the shared `profile.honor.tier.*`
 * i18n keys (the same strings every badge and meter shows — the rules must
 * never call a tier by a different word), coloured with the tier's own ramp
 * hue; ranges are derived from HONOR_TIER_BANDS. Only the sentence tails and
 * the host note are authored per locale.
 */
function TiersBlock({ block }: { block: Extract<RuleBlockType, { kind: "tiers" }> }) {
  const { t } = useTranslation();
  return (
    <div
      style={{
        padding: "14px 16px 14px 18px",
        background: "var(--surface-2)",
        border: "1px solid var(--border)",
        borderLeft: "3px solid var(--accent)",
        borderRadius: 12,
        display: "flex",
        flexDirection: "column",
        gap: 4,
      }}
    >
      <div
        className="font-display"
        style={{ fontSize: 14.5, fontWeight: 600, color: "var(--ink)", letterSpacing: -0.1 }}
      >
        {block.title}
      </div>
      <ul
        style={{
          margin: 0,
          padding: 0,
          listStyle: "none",
          display: "flex",
          flexDirection: "column",
          gap: 3,
        }}
      >
        {block.items.map((it) => (
          <li
            key={it.tier}
            data-testid={`rules-tier-${it.tier}`}
            style={{ fontSize: 13.5, lineHeight: 1.55, color: "var(--ink-dim)" }}
          >
            <strong style={{ fontWeight: 700, color: HONOR_TIER_COLOR[it.tier] }}>
              {t(`profile.honor.tier.${it.tier}`)}
            </strong>{" "}
            <span className="font-mono" style={{ fontSize: 12 }}>
              ({tierRange(it.tier)})
            </span>{" "}
            {it.d}
          </li>
        ))}
      </ul>
      <div style={{ fontSize: 13.5, lineHeight: 1.55, color: "var(--ink-dim)" }}>{block.text}</div>
    </div>
  );
}

function CardBlock() {
  const { cardsTrump, cardsPlain, ui } = useRules();
  return (
    <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2">
      <CardLadder
        testId="ladder-trump"
        title={ui.ladderTrumpTitle}
        accent={{
          eyebrow: ui.ladderTrumpEyebrow,
          ink: "var(--accent)",
          edge: "rgba(25,101,54,0.45)",
          tint: "rgba(25,101,54,0.10)",
          softTint: "rgba(25,101,54,0.05)",
        }}
        suit="C"
        rows={cardsTrump}
      />
      <CardLadder
        testId="ladder-plain"
        title={ui.ladderPlainTitle}
        accent={{
          eyebrow: ui.ladderPlainEyebrow,
          ink: "var(--brass-deep)",
          edge: "var(--border-2)",
          tint: "var(--brass-soft)",
          softTint: "rgba(201,168,118,0.08)",
        }}
        suit="H"
        rows={cardsPlain}
      />
    </div>
  );
}

/** Renders one content block by kind. New kinds are added in `content/`. */
export function RuleBlock({ block }: { block: RuleBlockType }) {
  switch (block.kind) {
    case "p":
      return (
        <p style={{ margin: 0, fontSize: 15, lineHeight: 1.65, color: "var(--ink-dim)" }}>
          {block.text}
        </p>
      );

    case "rule":
      return (
        <div
          style={{
            padding: "14px 16px 14px 18px",
            background: "var(--surface-2)",
            border: "1px solid var(--border)",
            borderLeft: "3px solid var(--accent)",
            borderRadius: 12,
            display: "flex",
            flexDirection: "column",
            gap: 4,
          }}
        >
          <div
            className="font-display"
            style={{ fontSize: 14.5, fontWeight: 600, color: "var(--ink)", letterSpacing: -0.1 }}
          >
            {block.title}
          </div>
          <div style={{ fontSize: 13.5, lineHeight: 1.55, color: "var(--ink-dim)" }}>
            {block.text}
          </div>
        </div>
      );

    case "note":
      return <NoteBlock text={block.text} />;

    case "tiers":
      return <TiersBlock block={block} />;

    case "steps":
      return (
        <ol
          style={{
            margin: 0,
            padding: 0,
            listStyle: "none",
            display: "flex",
            flexDirection: "column",
            gap: 10,
          }}
        >
          {block.items.map((it, i) => (
            <li
              key={i}
              style={{
                display: "grid",
                gridTemplateColumns: "36px 1fr",
                gap: 14,
                padding: "12px 14px",
                background: "var(--surface-2)",
                border: "1px solid var(--border)",
                borderRadius: 12,
              }}
            >
              <span
                className="font-mono"
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                  width: 28,
                  height: 28,
                  borderRadius: 8,
                  background: "var(--accent-soft)",
                  color: "var(--accent)",
                  fontSize: 12,
                  fontWeight: 700,
                  letterSpacing: 0.6,
                }}
              >
                {String(i + 1).padStart(2, "0")}
              </span>
              <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
                <span
                  className="font-display"
                  style={{
                    fontSize: 14.5,
                    fontWeight: 600,
                    color: "var(--ink)",
                    letterSpacing: -0.1,
                  }}
                >
                  {it.t}
                </span>
                <span style={{ fontSize: 13, lineHeight: 1.55, color: "var(--ink-dim)" }}>
                  {it.d}
                </span>
              </div>
            </li>
          ))}
        </ol>
      );

    case "cards":
      return <CardBlock />;

    case "melds":
      return <MeldsGrid />;

    default:
      return null;
  }
}
