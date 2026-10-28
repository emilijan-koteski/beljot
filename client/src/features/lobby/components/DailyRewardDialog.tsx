import { Check, Flame, Sparkle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AppDialog } from "@/shared/components/ui/app-dialog";
import { formatCoins } from "@/shared/lib/formatCoins";
import { cn } from "@/shared/lib/utils";

interface DailyRewardDialogProps {
  open: boolean;
  amount: number;
  streakDay: number;
  onClose: () => void;
}

const STREAK_MAX = 14;

/** The Beljot coin: a brass disc with the "B" monogram, drawn from tokens. */
function Coin() {
  return (
    <span
      aria-hidden="true"
      className="from-brass to-brass-deep text-brass-ink ring-brass-soft flex size-18 items-center justify-center rounded-full bg-linear-to-b text-3xl font-bold shadow-md ring-4"
    >
      B
    </span>
  );
}

/** The 14-day login streak: cleared days filled with a check, today ringed, day 14 marked with a sparkle. */
function StreakTrack({ day }: { day: number }) {
  return (
    <div className="flex items-center justify-center gap-1.5" aria-hidden="true">
      {Array.from({ length: STREAK_MAX }, (_, i) => {
        const n = i + 1;
        const done = n < day;
        const today = n === day;
        return (
          <span
            key={n}
            data-state={done ? "done" : today ? "today" : "upcoming"}
            className={cn(
              "flex items-center justify-center rounded-full",
              today ? "bg-brass text-brass-ink ring-brass-soft size-4 ring-3" : "size-3",
              done && "bg-brass text-brass-ink",
              !done && !today && "border-brass/50 text-brass border border-dashed",
            )}
          >
            {n === STREAK_MAX ? (
              <Sparkle className="size-2" />
            ) : (
              done && <Check className="size-2" strokeWidth={4} />
            )}
          </span>
        );
      })}
    </div>
  );
}

/** The once-a-day coin reward on the celebration shell: the amount, the streak and one Continue. */
export function DailyRewardDialog({ open, amount, streakDay, onClose }: DailyRewardDialogProps) {
  const { t } = useTranslation();
  const milestone = streakDay >= STREAK_MAX;

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      tone="celebration"
      art={<Coin />}
      title={t(milestone ? "rewards.dailyReward.streakMaxed" : "rewards.dailyReward.title")}
      primary={{
        label: t("rewards.dailyReward.continue"),
        onClick: onClose,
        testId: "daily-reward-continue",
      }}
      testId="daily-reward-dialog"
    >
      <p
        data-testid="daily-reward-amount"
        className={cn(
          "m-0 text-5xl leading-none font-bold tracking-tight tabular-nums",
          milestone ? "text-team-a" : "text-ink",
        )}
      >
        +{formatCoins(amount)}
      </p>
      <div className="mt-5">
        <StreakTrack day={Math.min(streakDay, STREAK_MAX)} />
      </div>
      <p className="text-ink-dim mt-2 mb-0 inline-flex items-center gap-1 text-xs">
        <Flame className="text-brass-deep size-3.5" aria-hidden="true" />
        {t("rewards.dailyReward.streak", { streak: streakDay })}
      </p>
    </AppDialog>
  );
}
