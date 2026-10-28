import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { CardLadder } from "@/features/rules/components/CardLadder";
import { DiffMarker } from "@/features/rules/components/DiffMarker";
import { MeldsGrid } from "@/features/rules/components/MeldsGrid";
import type { RuleBlock as RuleBlockType } from "@/features/rules/content/types";
import { isVisibleFor, visibleFor } from "@/features/rules/content/visibility";
import { useRules, useRulesDense, useRulesVariant } from "@/features/rules/RulesContext";
import { HONOR_TIER_BANDS, HONOR_TIER_COLOR, type HonorTier } from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";

const RULE_CARD =
  "bg-surface-2 border-border border-l-accent flex flex-col gap-1 rounded-lg border border-l-3 py-3.5 pr-4 pl-4.5";

function NoteBlock({ text, marker }: { text: string; marker: ReactNode }) {
  const { ui } = useRules();
  return (
    <div className="bg-brass-soft border-brass text-ink-dim flex items-start gap-2.5 rounded-lg border border-dashed px-3.5 py-3 text-xs leading-relaxed">
      <span className="text-brass-deep shrink-0 pt-px font-mono text-2xs font-bold tracking-caps uppercase">
        {ui.noteLabel}
      </span>
      <span>
        {text}
        {marker}
      </span>
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
 * Honour-tier ladder using the shared `profile.honor.tier.*` names and ranges derived from HONOR_TIER_BANDS.
 */
function TiersBlock({
  block,
  marker,
}: {
  block: Extract<RuleBlockType, { kind: "tiers" }>;
  marker: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <div className={RULE_CARD}>
      <div className="text-ink text-sm font-semibold">
        {block.title}
        {marker}
      </div>
      <ul className="m-0 flex list-none flex-col gap-0.75 p-0">
        {block.items.map((it) => (
          <li
            key={it.tier}
            data-testid={`rules-tier-${it.tier}`}
            className="text-ink-dim text-sm leading-relaxed"
          >
            <strong className="font-bold" style={{ color: HONOR_TIER_COLOR[it.tier] }}>
              {t(`profile.honor.tier.${it.tier}`)}
            </strong>{" "}
            <span className="font-mono text-xs">({tierRange(it.tier)})</span> {it.d}
          </li>
        ))}
      </ul>
      <div className="text-ink-dim text-sm leading-relaxed">{block.text}</div>
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
        eyebrow={ui.ladderTrumpEyebrow}
        tone="trump"
        suit="C"
        rows={cardsTrump}
      />
      <CardLadder
        testId="ladder-plain"
        title={ui.ladderPlainTitle}
        eyebrow={ui.ladderPlainEyebrow}
        tone="plain"
        suit="H"
        rows={cardsPlain}
      />
    </div>
  );
}

/**
 * Renders one content block for the active variant tab, or nothing if it is scoped to the other variant.
 * Scoped blocks carry a difference marker naming what the other variant does.
 */
export function RuleBlock({ block }: { block: RuleBlockType }) {
  const activeVariant = useRulesVariant();
  const dense = useRulesDense();
  const { ui } = useRules();

  if (!isVisibleFor(block, activeVariant)) return null;

  const tone = dense ? "dark" : "light";
  const marker = block.otherVariantNote ? (
    <DiffMarker note={block.otherVariantNote} label={ui.diffLabel} tone={tone} />
  ) : null;

  switch (block.kind) {
    case "p":
      return (
        <p className={cn("text-ink-dim m-0 leading-relaxed", dense ? "text-sm" : "text-base")}>
          {block.text}
          {marker}
        </p>
      );

    case "rule":
      return (
        <div className={RULE_CARD}>
          <div className="text-ink text-sm font-semibold">
            {block.title}
            {marker}
          </div>
          <div className="text-ink-dim text-sm leading-relaxed">{block.text}</div>
        </div>
      );

    case "note":
      return <NoteBlock text={block.text} marker={marker} />;

    case "tiers":
      return <TiersBlock block={block} marker={marker} />;

    case "steps": {
      const visibleItems = visibleFor(block.items, activeVariant);
      return (
        <div className="flex flex-col gap-2">
          {marker ? <div className="flex justify-end">{marker}</div> : null}
          <ol className="m-0 flex list-none flex-col gap-2.5 p-0">
            {visibleItems.map((it, i) => (
              <li
                key={i}
                className="bg-surface-2 border-border grid grid-cols-[36px_1fr] gap-3.5 rounded-lg border px-3.5 py-3"
              >
                <span className="bg-accent-soft text-accent inline-flex size-7 items-center justify-center rounded-md font-mono text-xs font-bold tracking-wider">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <div className="flex flex-col gap-0.75">
                  <span className="text-ink text-sm font-semibold">
                    {it.t}
                    {it.otherVariantNote ? (
                      <DiffMarker note={it.otherVariantNote} label={ui.diffLabel} tone={tone} />
                    ) : null}
                  </span>
                  <span className="text-ink-dim text-sm leading-relaxed">{it.d}</span>
                </div>
              </li>
            ))}
          </ol>
        </div>
      );
    }

    case "cards":
      return <CardBlock />;

    case "melds":
      return <MeldsGrid />;

    default:
      return null;
  }
}
