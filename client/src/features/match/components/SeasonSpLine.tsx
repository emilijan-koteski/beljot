import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import {
  formatSpChange,
  normalizeSeasonDivision,
  normalizeSeasonTier,
  seasonRankLabel,
} from "@/shared/lib/seasonTier";
import type { SeasonSettlement } from "@/shared/stores/matchStore";

type Props = {
  settlement: SeasonSettlement;
};

/**
 * Match-end Season Points line showing the signed SP change and resulting rank, with copy picked by the settlement reason.
 * The change is never clamped, so a loss can never render as "0 SP".
 */
export function SeasonSpLine({ settlement }: Props) {
  const { t } = useTranslation();

  const change = formatSpChange(settlement.spChange);
  const gained = settlement.spChange > 0;
  const tier = normalizeSeasonTier(settlement.rankTier, settlement.newSeasonSp);

  let text: string;
  let namesRank = false;
  if (settlement.reason === "partner_abandoned" && settlement.spChange < 0) {
    text = t("season.result.partnerAbandoned", { change });
  } else if (settlement.reason === "abandoned") {
    text = t("season.result.abandoned", { change });
  } else {
    const rank = seasonRankLabel(t, tier, normalizeSeasonDivision(tier, settlement.rankDivision));
    text = t("season.result.line", { change, rank });
    namesRank = true;
  }

  const color = gained ? "var(--accent)" : "var(--ink-mute)";
  const trend = gained ? "gain" : settlement.spChange < 0 ? "loss" : "none";

  return (
    <div
      className="font-display inline-flex items-center gap-2 rounded-full px-3.5 py-1.5"
      style={{
        color,
        border: `1px solid color-mix(in srgb, ${color} 40%, transparent)`,
        background: `color-mix(in srgb, ${color} 8%, transparent)`,
      }}
      role="status"
      data-testid="season-sp-line"
      data-sp-change={settlement.spChange}
      data-reason={settlement.reason}
      data-trend={trend}
    >
      {namesRank && <TierBadge tier={tier} size="sm" data-testid="season-sp-line-badge" />}
      <span className="text-base font-semibold tabular-nums">{text}</span>
    </div>
  );
}
