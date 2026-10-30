import { ArrowRight, Flame, Snowflake } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type { CareerStreak } from "@/shared/api/career";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { cn } from "@/shared/lib/utils";

import { WinRateRing } from "./WinRateRing";

type CareerCardProps = {
  games: number;
  wins: number;
  losses: number;
  abandoned: number;
  streak?: CareerStreak;
  statsError?: boolean;
  subjectIsSelf?: boolean;
};

function Count({
  testId,
  label,
  value,
  className,
}: {
  testId: string;
  label: string;
  value: number;
  className?: string;
}) {
  return (
    <div className="flex flex-col" data-testid={testId} data-value={String(value)}>
      <span className="text-ink-mute text-2xs font-semibold tracking-wider uppercase">{label}</span>
      <span className={cn("text-ink text-lg font-bold tabular-nums", className)}>{value}</span>
    </div>
  );
}

function OutcomeSplit({
  wins,
  losses,
  abandoned,
}: {
  wins: number;
  losses: number;
  abandoned: number;
}) {
  const total = wins + losses + abandoned;
  if (total === 0) return null;
  const share = (n: number) => `${(n / total) * 100}%`;
  return (
    <span
      className="bg-surface-sunken flex h-2 overflow-hidden rounded-full"
      aria-hidden="true"
      data-testid="profile-outcome-split"
    >
      <span className="bg-accent" style={{ width: share(wins) }} />
      <span className="bg-danger/70" style={{ width: share(losses) }} />
      <span className="bg-ink-mute/50" style={{ width: share(abandoned) }} />
    </span>
  );
}

function StreakLine({ streak, subjectIsSelf }: { streak: CareerStreak; subjectIsSelf: boolean }) {
  const { t } = useTranslation();
  if (streak.length === 0 || streak.kind === "none") return null;
  const hot = streak.kind === "win";
  const subtitle = subjectIsSelf
    ? t(hot ? "profile.streak.winSubtitle" : "profile.streak.lossSubtitle")
    : t(hot ? "profile.streak.winSubtitlePublic" : "profile.streak.lossSubtitlePublic");
  return (
    <div
      className={cn(
        "flex items-center gap-2.5 rounded-lg px-3 py-2",
        hot ? "bg-accent-soft text-accent-deep" : "bg-surface-sunken text-ink-dim",
      )}
      data-testid="profile-streak"
      data-streak-kind={streak.kind}
      data-streak-length={streak.length}
    >
      {hot ? <Flame className="size-4 shrink-0" /> : <Snowflake className="size-4 shrink-0" />}
      <span className="flex min-w-0 flex-1 flex-col leading-tight">
        <span className="text-xs font-semibold">
          {t(hot ? "profile.streak.winTitle" : "profile.streak.lossTitle", {
            count: streak.length,
          })}
        </span>
        <span className="text-2xs opacity-80">{subtitle}</span>
      </span>
      {subjectIsSelf && (
        <Link
          to="/lobby"
          className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold underline-offset-2 hover:underline"
          data-testid="profile-streak-play"
        >
          {t("profile.streak.play")}
          <ArrowRight className="size-3.5" />
        </Link>
      )}
    </div>
  );
}

/** Career totals: the win-rate ring beside matches, wins, losses and abandoned, the outcome split, and any streak. */
export function CareerCard({
  games,
  wins,
  losses,
  abandoned,
  streak,
  statsError = false,
  subjectIsSelf = true,
}: CareerCardProps) {
  const { t } = useTranslation();
  const rate = games === 0 ? null : Math.round((wins / games) * 100);

  return (
    <section
      className="bg-surface border-border flex flex-col gap-3.5 rounded-xl border p-4 shadow-xs"
      data-testid="profile-stats"
    >
      <Eyebrow>{t("profile.stats.eyebrow")}</Eyebrow>
      {statsError ? (
        <p className="text-danger m-0 text-sm" data-testid="profile-stats-error">
          {t("profile.stats.error")}
        </p>
      ) : (
        <>
          <div className="flex items-center gap-4">
            <WinRateRing rate={rate} />
            <div className="grid flex-1 grid-cols-2 gap-x-3 gap-y-2.5">
              <Count
                testId="profile-stat-games-played"
                label={t("profile.stats.totalGamesPlayed")}
                value={games}
              />
              <Count
                testId="profile-stat-wins"
                label={t("profile.stats.wins")}
                value={wins}
                className="text-accent-deep"
              />
              <Count
                testId="profile-stat-losses"
                label={t("profile.stats.losses")}
                value={losses}
                className="text-danger/80"
              />
              <Count
                testId="profile-stat-abandoned"
                label={t("profile.stats.abandoned")}
                value={abandoned}
                className="text-ink-dim"
              />
            </div>
          </div>
          <OutcomeSplit wins={wins} losses={losses} abandoned={abandoned} />
        </>
      )}
      {streak && <StreakLine streak={streak} subjectIsSelf={subjectIsSelf} />}
    </section>
  );
}
