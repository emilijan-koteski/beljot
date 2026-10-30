import { Crown } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  seasonRankLabel,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";
import type { LeaderboardRow } from "@/shared/types/apiTypes";

const MEDAL = [SEASON_TIER_COLOR.gold, SEASON_TIER_COLOR.silver, SEASON_TIER_COLOR.bronze] as const;

const STEP_HEIGHT = {
  sm: ["h-16", "h-10", "h-7"],
  lg: ["h-24", "h-16", "h-12"],
} as const;

interface LeaderboardPodiumProps {
  top: LeaderboardRow[];
  viewerId: number | null;
  size: "sm" | "lg";
}

/** The season's top three on steps: first in the middle and highest, each with a medal ring, rank and SP. */
export function LeaderboardPodium({ top, viewerId, size }: LeaderboardPodiumProps) {
  const { t } = useTranslation();
  const [first, second, third] = top;
  const order = [second, first, third];
  return (
    <section
      className="grid grid-cols-3 items-end gap-2 pt-4"
      aria-label={t("season.leaderboard.topThree")}
      data-testid="leaderboard-podium"
    >
      {order.map((row, i) => {
        if (row === undefined) return <span key={`empty-${i}`} />;
        const place = row.position;
        const medal = MEDAL[Math.min(place, 3) - 1] ?? MEDAL[2];
        const sp = seasonSpOrZero(row.sp);
        const tier = normalizeSeasonTier(row.tier, sp);
        const self = viewerId !== null && row.userId === viewerId;
        const avatar = size === "lg" ? (place === 1 ? 72 : 60) : place === 1 ? 60 : 48;
        return (
          <div
            key={row.userId}
            className="flex min-w-0 flex-col items-center gap-1.5"
            data-testid="leaderboard-podium-place"
            data-place={place}
          >
            <span className="relative flex">
              {place === 1 && (
                <Crown
                  className="absolute -top-5 left-1/2 size-5 -translate-x-1/2"
                  style={{ color: medal }}
                  aria-hidden="true"
                />
              )}
              <span
                className="flex rounded-full p-0.5"
                style={{ boxShadow: `0 0 0 2px ${medal}` }}
                data-testid="leaderboard-podium-ring"
              >
                <Avatar name={row.username} avatarUrl={row.avatarUrl} size={avatar} ring={medal} />
              </span>
              <span
                className="text-surface absolute -right-1 -bottom-1 flex size-5.5 items-center justify-center rounded-full text-2xs font-bold"
                style={{ background: medal }}
                aria-hidden="true"
              >
                {place}
              </span>
            </span>
            <Link
              to={self ? "/profile" : `/players/${row.userId}`}
              className="text-ink max-w-full truncate text-sm font-semibold underline-offset-2 hover:underline"
            >
              {row.username}
            </Link>
            <span
              className="-mt-1 text-2xs font-semibold"
              style={{ color: SEASON_TIER_COLOR[tier] }}
            >
              {seasonRankLabel(t, tier, normalizeSeasonDivision(tier, row.division))}
            </span>
            <div
              className={cn(
                "flex w-full justify-center rounded-t-lg border border-b-0 pt-1.5",
                STEP_HEIGHT[size][place - 1] ?? STEP_HEIGHT[size][2],
              )}
              style={{
                borderColor: `color-mix(in srgb, ${medal} 45%, transparent)`,
                background: `color-mix(in srgb, ${medal} 14%, var(--surface))`,
              }}
            >
              <span className="text-ink text-sm font-bold tabular-nums">
                {sp.toLocaleString()}{" "}
                <span className="text-ink-mute text-2xs font-semibold">
                  {t("season.leaderboard.spUnit")}
                </span>
              </span>
            </div>
          </div>
        );
      })}
    </section>
  );
}
