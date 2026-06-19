import { Coins } from "lucide-react";
import { useTranslation } from "react-i18next";

import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";

interface StakePillProps {
  /** The total match pot — sum of every human player's buy-in. */
  amount: number;
}

const PANEL_BG = "var(--panel-dark, rgba(20,45,30,0.85))";
const INK = "var(--ink-light, #f5f2e8)";

/**
 * Compact match-stake (pot) chip for the table HUD — a brass-bordered pill
 * matching the ScorePanel / TrumpIndicator chrome, showing the gold coin glyph
 * and the formatted pot total. Icon-only unit (the coin glyph conveys "coins",
 * mirroring the room-card buy-in label); the localized "Stake" word rides the
 * aria-label for screen readers.
 *
 * Positionless by design: the caller anchors it (top-right beside the mobile
 * hamburger / beneath the desktop trump indicator) and sets its z-tier (Z.HUD).
 */
export function StakePill({ amount }: StakePillProps) {
  const { t } = useTranslation();
  const formatted = formatCoins(amount);

  return (
    <div
      className="font-display inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5"
      style={{
        background: PANEL_BG,
        border: "1px solid rgba(201,168,118,0.4)",
        boxShadow: "0 4px 14px rgba(0,0,0,0.3)",
        backdropFilter: "blur(8px)",
        WebkitBackdropFilter: "blur(8px)",
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
