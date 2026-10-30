import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { buttonVariants } from "@/shared/components/ui/button";
import {
  nextSeasonRank,
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_LINE,
  seasonBarFill,
  seasonRankLabel,
  seasonSpOrZero,
  seasonWinRate,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";
import type { CurrentSeasonResponse, LeaderboardViewer } from "@/shared/types/apiTypes";

interface SeasonStandingCardProps {
  standing: LeaderboardViewer | null;
  total: number;
  progress?: CurrentSeasonResponse;
  past: boolean;
  layout: "stack" | "row";
}

const CARD = "bg-surface border-border flex rounded-xl border p-4 shadow-xs";

/**
 * The viewer's season at a glance: rank and SP with the climb to the next rank, then place, record and win rate.
 * Without a standing it invites a first match in the running season and stays away from a past one.
 */
export function SeasonStandingCard({
  standing,
  total,
  progress,
  past,
  layout,
}: SeasonStandingCardProps) {
  const { t } = useTranslation();

  if (standing === null) {
    if (past) return null;
    return (
      <section className={cn(CARD, "flex-col gap-3")} data-testid="leaderboard-standing-empty">
        <div className="flex items-center gap-3">
          <TierBadge tier="iron" size="md" glow={false} />
          <div className="flex min-w-0 flex-col">
            <span className="text-ink text-base font-semibold">
              {t("season.leaderboard.notOnBoard")}
            </span>
            <span className="text-ink-mute text-xs">{t("season.leaderboard.notOnBoardSub")}</span>
          </div>
        </div>
        <Link to="/lobby" className={cn(buttonVariants({ size: "cta" }), "self-start")}>
          {t("nav.play")}
        </Link>
      </section>
    );
  }

  const sp = seasonSpOrZero(standing.sp);
  const tier = normalizeSeasonTier(standing.tier, sp);
  const division = normalizeSeasonDivision(tier, standing.division);
  const played = seasonSpOrZero(standing.gamesPlayed);
  const won = Math.min(seasonSpOrZero(standing.gamesWon), played);
  const next = nextSeasonRank(tier, division);
  const into = seasonSpOrZero(progress?.spIntoDivision);
  const forNext = seasonSpOrZero(progress?.spForNextDivision);
  const row = layout === "row";

  const stats: Array<{ key: string; label: string; value: string; sub: string }> = [
    {
      key: "place",
      label: t("season.leaderboard.place"),
      value: `#${standing.position.toLocaleString()}`,
      sub: t("season.leaderboard.placeOf", { total: total.toLocaleString() }),
    },
    {
      key: "record",
      label: t("season.leaderboard.record"),
      value: `${won}–${played - won}`,
      sub: t("season.leaderboard.played", { count: played }),
    },
    {
      key: "winRate",
      label: t("season.leaderboard.winRate"),
      value: `${seasonWinRate(won, played)}%`,
      sub: "",
    },
  ];

  return (
    <section
      className={cn(CARD, row ? "flex-row items-center gap-6" : "flex-col gap-3")}
      style={{ borderColor: SEASON_TIER_LINE[tier] }}
      data-testid="leaderboard-standing"
    >
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <TierBadge tier={tier} size="md" />
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <div className="flex items-baseline justify-between gap-2">
            <span className="text-base font-semibold" style={{ color: SEASON_TIER_COLOR[tier] }}>
              {seasonRankLabel(t, tier, division)}
            </span>
            <span className="text-ink text-xl font-bold tabular-nums">
              {sp.toLocaleString()}{" "}
              <span className="text-ink-mute text-2xs font-semibold">
                {t("season.leaderboard.spUnit")}
              </span>
            </span>
          </div>
          {past || progress === undefined ? (
            <span className="text-ink-mute text-xs">{t("season.leaderboard.finalRank")}</span>
          ) : (
            <div className="flex flex-col gap-1">
              <span className="bg-surface-sunken h-1.5 overflow-hidden rounded-full">
                <span
                  className="block h-full rounded-full"
                  style={{
                    width: `${Math.round(seasonBarFill(into, forNext) * 100)}%`,
                    background: SEASON_TIER_COLOR[tier],
                  }}
                />
              </span>
              <span className="text-ink-mute text-2xs tabular-nums">
                {next === null || forNext <= 0
                  ? t("season.banner.atTop")
                  : t("season.leaderboard.toNext", {
                      sp: Math.max(0, forNext - into),
                      rank: seasonRankLabel(t, next.tier, next.division),
                    })}
              </span>
            </div>
          )}
        </div>
      </div>
      <dl
        className={cn(
          "m-0 grid grid-cols-3 gap-2 text-center",
          row ? "w-90 shrink-0" : "border-border border-t pt-3",
        )}
      >
        {stats.map((s) => (
          <div key={s.key} className="flex flex-col" data-testid={`leaderboard-standing-${s.key}`}>
            <dt className="text-ink-mute text-2xs font-semibold tracking-wider uppercase">
              {s.label}
            </dt>
            <dd className="text-ink m-0 text-base font-bold tabular-nums">{s.value}</dd>
            {s.sub !== "" && <dd className="text-ink-mute m-0 text-2xs tabular-nums">{s.sub}</dd>}
          </div>
        ))}
      </dl>
    </section>
  );
}
