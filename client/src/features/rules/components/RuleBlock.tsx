import { CardTable } from "@/features/rules/components/CardTable";
import { DiffMarker } from "@/features/rules/components/DiffMarker";
import { MeldsTable } from "@/features/rules/components/MeldsTable";
import { RoomOptions } from "@/features/rules/components/RoomOptions";
import { RuleList } from "@/features/rules/components/RuleList";
import { ScoreSum } from "@/features/rules/components/ScoreSum";
import { Timeline } from "@/features/rules/components/Timeline";
import type { RuleBlock as RuleBlockType } from "@/features/rules/content/types";
import { isVisibleFor, visibleFor } from "@/features/rules/content/visibility";
import { useRules, useRulesDense, useRulesVariant } from "@/features/rules/RulesContext";
import { InfoPopover } from "@/shared/components/ui/info-popover";
import { Z } from "@/shared/lib/zLayers";

/**
 * Renders one content block for the active variant tab, or nothing if it is scoped to the other variant.
 * Scoped items carry a difference marker naming what the other variant does.
 */
export function RuleBlock({ block }: { block: RuleBlockType }) {
  const variant = useRulesVariant();
  const dense = useRulesDense();
  const { ui } = useRules();

  if (!isVisibleFor(block, variant)) return null;
  const tone = dense ? "dark" : "light";

  switch (block.kind) {
    case "p":
      return (
        <p className="text-ink-dim m-0 text-sm leading-relaxed">
          {block.text}
          {block.otherVariantNote !== undefined && (
            <DiffMarker note={block.otherVariantNote} label={ui.diffLabel} tone={tone} />
          )}
          {block.help !== undefined && (
            <InfoPopover
              text={block.help}
              ariaLabel={ui.helpLabel}
              tone={tone}
              zIndex={dense ? Z.UTIL_POPOVER : undefined}
              testId="rules-help"
              className="ml-1.5"
            />
          )}
        </p>
      );

    case "steps":
      return <Timeline items={visibleFor(block.items, variant)} />;

    case "list":
      return <RuleList items={visibleFor(block.items, variant)} numbered={block.numbered} />;

    case "cards":
      return <CardTable />;

    case "melds":
      return <MeldsTable />;

    case "sum":
      return <ScoreSum />;

    case "options":
      return <RoomOptions />;

    default:
      return null;
  }
}
