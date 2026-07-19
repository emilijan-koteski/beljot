import { Check, Flame, Sparkle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { formatCoins } from "@/shared/lib/formatCoins";
import { cn } from "@/shared/lib/utils";

interface DailyRewardDialogProps {
  open: boolean;
  amount: number;
  streakDay: number;
  newBalance: number;
  onClose: () => void;
}

// The reward grows each day and caps on day 14 (server: wallet.DailyCap). At and
// past the cap the dialog swaps the brass strike for the gold "streak maxed"
// treatment; the streak counter itself keeps climbing for the "Day N" line.
const STREAK_MAX = 14;

/**
 * Struck-brass coin token with the embossed Beljot monogram — the coin-wallet
 * currency motif (same "B" as the face-down card). A single circle built from
 * radial gradients + layered inset shadows; `tone` swaps the standard brass
 * strike for the gold milestone strike. Size-parameterised so the 92px hero and
 * the 22px balance-row chip share one source.
 */
function CoinMedallion({
  size = 96,
  tone = "brass",
  glyph = "B",
}: {
  size?: number;
  tone?: "brass" | "gold";
  glyph?: string;
}) {
  const gold = tone === "gold";
  const face = gold
    ? "radial-gradient(circle at 38% 30%, #fbeec0 0%, #e8c25a 50%, #a07d1a 100%)"
    : "radial-gradient(circle at 38% 30%, #f4e3b8 0%, #c9a876 52%, #8a6a3c 100%)";
  const rim = gold ? "#a07d1a" : "#8a6a3c";
  const halo = gold ? "rgba(232,194,90,0.45)" : "rgba(201,168,118,0.40)";
  const ink = gold ? "#6e520f" : "#5a4322";
  return (
    <span
      aria-hidden="true"
      style={{
        position: "relative",
        width: size,
        height: size,
        display: "inline-flex",
        flexShrink: 0,
      }}
    >
      {/* soft outer glow */}
      <span
        style={{
          position: "absolute",
          inset: -size * 0.28,
          borderRadius: "50%",
          background: `radial-gradient(circle, ${halo} 0%, transparent 68%)`,
        }}
      />
      {/* coin body */}
      <span
        style={{
          position: "relative",
          width: size,
          height: size,
          borderRadius: "50%",
          background: face,
          border: `${Math.max(2, size * 0.025)}px solid ${rim}`,
          boxShadow: `inset 0 ${size * 0.03}px ${size * 0.05}px rgba(255,255,255,0.6), inset 0 -${size * 0.04}px ${size * 0.07}px rgba(0,0,0,0.28), 0 ${size * 0.08}px ${size * 0.18}px -${size * 0.06}px rgba(90,67,34,0.7)`,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        {/* engraved inner rim */}
        <span
          style={{
            position: "absolute",
            inset: size * 0.1,
            borderRadius: "50%",
            border: `1px solid ${rim}88`,
            boxShadow: "inset 0 1px 2px rgba(0,0,0,0.18)",
          }}
        />
        <span
          className="font-display"
          style={{
            fontSize: size * 0.46,
            fontWeight: 700,
            color: ink,
            letterSpacing: -1,
            textShadow: "0 1px 0 rgba(255,255,255,0.55), 0 -1px 1px rgba(0,0,0,0.25)",
          }}
        >
          {glyph}
        </span>
      </span>
    </span>
  );
}

/**
 * 14-day login-streak track. Cleared days fill brass with a check, today rings
 * in gold, future days are dashed wells; day 14 always carries the sparkle as
 * the max-reward milestone.
 */
function StreakTrack({ day, total = STREAK_MAX }: { day: number; total?: number }) {
  return (
    <div className="flex items-center justify-center gap-[5px]" aria-hidden="true">
      {Array.from({ length: total }, (_, i) => {
        const n = i + 1;
        const done = n < day;
        const today = n === day;
        const milestone = n === total;
        return (
          <span
            key={n}
            style={{
              width: today ? 18 : 13,
              height: today ? 18 : 13,
              borderRadius: "50%",
              flexShrink: 0,
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              background: today
                ? "radial-gradient(circle at 38% 30%, #fbeec0, #e8c25a 60%, #a07d1a)"
                : done
                  ? "radial-gradient(circle at 38% 30%, #ead9b0, #c9a876 60%, #9c7d4e)"
                  : "var(--surface-3)",
              border: today
                ? "1.5px solid #a07d1a"
                : done
                  ? "1px solid #9c7d4e"
                  : "1px dashed var(--border-2)",
              boxShadow: today
                ? "0 0 0 3px rgba(232,194,90,0.28), 0 3px 8px -2px rgba(160,125,26,0.6)"
                : "none",
              color: today ? "#6e520f" : done ? "#5a4322" : "var(--ink-off)",
            }}
          >
            {milestone ? (
              <Sparkle size={today ? 11 : 8} />
            ) : done ? (
              <Check size={8} strokeWidth={3.4} />
            ) : null}
          </span>
        );
      })}
    </div>
  );
}

/**
 * DailyRewardDialog is the persistent daily-login reward modal (AC #2). It is
 * fully controlled and opened programmatically by the gate — never via a
 * trigger. It stays open until the player clicks Continue: `disablePointerDismissal`
 * blocks outside-click dismissal and the no-op `onOpenChange` swallows every
 * library-initiated close (outside press / Escape / focus-out), so the only
 * close path is the explicit Continue button. There is no auto-dismiss timer.
 * The button only dismisses — the coins are already credited server-side by
 * the time the dialog opens (hence "Continue", not "Collect").
 *
 * Visual: the "Medallion" direction from the Daily Reward redesign — a struck
 * coin hero, the 14-day streak track, a wallet-balance ledger row, and the
 * felt-green Continue CTA, on the lobby's parchment + brass system.
 */
export function DailyRewardDialog({
  open,
  amount,
  streakDay,
  newBalance,
  onClose,
}: DailyRewardDialogProps) {
  const { t } = useTranslation();

  const milestone = streakDay >= STREAK_MAX;
  const tone = milestone ? "gold" : "brass";

  return (
    <Dialog open={open} disablePointerDismissal onOpenChange={() => {}}>
      <DialogContent
        showCloseButton={false}
        data-testid="daily-reward-dialog"
        className={cn(
          "block overflow-hidden rounded-[var(--radius-lg)] border border-border bg-surface p-0 ring-0 sm:max-w-[380px]",
          milestone
            ? "shadow-[0_0_0_1px_rgba(232,194,90,0.18)_inset,0_30px_70px_-28px_rgba(14,58,36,0.45)]"
            : "shadow-[0_0_0_1px_rgba(201,168,118,0.16)_inset,0_30px_70px_-28px_rgba(14,58,36,0.45)]",
        )}
      >
        {/* top brass hairline accent */}
        <div className="h-[3px] bg-[linear-gradient(90deg,transparent,var(--brass)_28%,var(--team-a-fill)_50%,var(--brass)_72%,transparent)]" />
        {/* faint top halo */}
        <div
          aria-hidden="true"
          className={cn(
            "pointer-events-none absolute inset-x-0 top-0 h-[180px]",
            milestone
              ? "bg-[radial-gradient(ellipse_70%_100%_at_50%_0%,rgba(232,194,90,0.18),transparent_70%)]"
              : "bg-[radial-gradient(ellipse_70%_100%_at_50%_0%,rgba(201,168,118,0.16),transparent_70%)]",
          )}
        />

        <div className="relative flex flex-col items-center gap-4 px-[26px] pt-[26px] pb-6 text-center">
          <DialogTitle className="text-brass-deep text-[11px] font-bold tracking-[2px] uppercase">
            {t(milestone ? "rewards.dailyReward.streakMaxed" : "rewards.dailyReward.title")}
          </DialogTitle>

          <CoinMedallion size={92} tone={tone} />

          <div>
            <div
              data-testid="daily-reward-amount"
              className={cn(
                "font-display tabular text-[46px] leading-none font-bold tracking-[-1.5px]",
                milestone ? "text-team-a" : "text-ink",
              )}
            >
              +{formatCoins(amount)}
            </div>
            <DialogDescription className="text-ink-dim mt-1 text-sm tracking-[0.3px]">
              {t("rewards.dailyReward.coinsAdded")}
            </DialogDescription>
          </div>

          <div className="w-full">
            <StreakTrack day={Math.min(streakDay, STREAK_MAX)} />
            <div className="text-ink-dim mt-3 inline-flex items-center gap-1.5 text-[12.5px]">
              <Flame className="text-brass-deep size-3.5" aria-hidden="true" />
              {t("rewards.dailyReward.streak", { streak: streakDay })}
            </div>
          </div>

          <div className="bg-border h-px w-full" />

          <div className="flex w-full items-center justify-between">
            <span className="text-ink-mute inline-flex items-center gap-2 text-[13px]">
              <CoinMedallion size={22} tone={tone} />
              {t("rewards.dailyReward.newBalance")}
            </span>
            <span className="font-display tabular text-ink text-[18px] font-bold">
              {formatCoins(newBalance)}
            </span>
          </div>

          <Button
            onClick={onClose}
            size="cta"
            className="w-full"
            data-testid="daily-reward-continue"
          >
            {t("rewards.dailyReward.continue")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
