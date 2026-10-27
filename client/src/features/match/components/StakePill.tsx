import { Coins } from "lucide-react";
import { useTranslation } from "react-i18next";

import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";

import { floatingPanelStyle } from "../lib/floatingPanel";

interface StakePillProps {
  amount: number;
}

const INK = "var(--ink-light)";

/**
 * Positionless brass pill showing the match stake with a coin glyph; the caller anchors it.
 */
export function StakePill({ amount }: StakePillProps) {
  const { t } = useTranslation();
  const formatted = formatCoins(amount);

  return (
    <div
      className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5"
      style={{
        ...floatingPanelStyle(),
        color: INK,
      }}
      aria-live="polite"
      aria-label={`${t("match.stake.label")}: ${formatted}`}
      data-testid="match-stake"
    >
      <Coins className="size-4 shrink-0" style={{ color: COIN_GOLD }} aria-hidden="true" />
      <span className="text-sm font-bold tabular-nums" data-testid="match-stake-amount">
        {formatted}
      </span>
    </div>
  );
}
