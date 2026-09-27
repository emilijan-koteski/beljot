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
 * The match-end Season Points line (Story 13.5): the signed change and the rank
 * it lands on ("+24 SP · Gold 2"), shown beside the coin settlement on
 * MatchResult and on the abandoned ReconnectOverlay. ONE renderer for both, so
 * the reason variants and the colour rule cannot drift between them.
 *
 * THE REASON PICKS THE COPY. A teammate of the abandoning seat reads "Partner
 * abandoned: −12 SP (half loss)", the abandoner their own penalty plainly, and
 * everyone else the ordinary line. The half-loss copy needs an actual LOSS: a
 * teammate whose applied change is 0 (the 0 floor) or positive (a Capot bonus
 * over a small half loss) gets the ordinary line instead of "+3 SP (half loss)".
 *
 * `role="status"`: the line usually mounts AFTER its result screen (the SP event
 * trails match_end / match_abandoned), so it is a live region to be announced.
 *
 * THE CHANGE IS SIGNED AND NEVER CLAMPED. `formatSpChange` renders "+24", "−13"
 * (U+2212) or "0" straight from the server's number; it must never pass through
 * `seasonSpOrZero`, `seasonBarFill` or a `finiteOrZero` clamp, which would turn
 * every loss into "0 SP".
 *
 * A GAIN USES THE ACCENT COLOUR, a loss (or no change) a muted ink, never red:
 * dropping SP is the ordinary downside of a loss, not an alarm. Both tokens
 * re-root inside `.game-table`, so the pill reads correctly on dark felt.
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
      {/* Decoration beside the rank NAME (TierBadge's own aria-hidden rests on
          that), so it rides only the line that names the rank. */}
      {namesRank && <TierBadge tier={tier} size="sm" data-testid="season-sp-line-badge" />}
      <span className="text-base font-semibold tabular-nums">{text}</span>
    </div>
  );
}
