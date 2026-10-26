import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { Avatar } from "@/shared/components/ui/avatar";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  seasonRankLabel,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/** Coerces a possibly absent server integer to a finite number, treating 0 as real. */
function finiteOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? Math.max(0, value as number) : 0;
}

type Props = {
  position: number;
  userId: number;
  username: string;
  avatarUrl?: string | null;
  sp: number;
  tier: string;
  division: number | null;
  gamesPlayed?: number;
  isSelf?: boolean;
  className?: string;
};

/**
 * One leaderboard row, used for both the list and the pinned own-row.
 * A single sr-only sentence carries every value; visible cells are aria-hidden except the username link.
 */
export function LeaderboardRow({
  position,
  userId,
  username,
  avatarUrl,
  sp,
  tier,
  division,
  gamesPlayed,
  isSelf = false,
  className,
}: Props) {
  const { t } = useTranslation();

  const total = finiteOrZero(sp);
  const safeTier = normalizeSeasonTier(tier, total);
  const rankName = seasonRankLabel(t, safeTier, normalizeSeasonDivision(safeTier, division));
  const games = gamesPlayed === undefined ? undefined : finiteOrZero(gamesPlayed);

  return (
    <li
      data-testid="leaderboard-row"
      data-user-id={userId}
      {...(isSelf ? { "data-self": "true" } : {})}
      className={cn(
        "border-border flex items-center gap-2.5 rounded-md border px-2.5 py-1.5",
        isSelf ? "bg-accent-soft border-accent/40" : "bg-surface-elevated",
        className,
      )}
    >
      <span className="sr-only" data-testid="leaderboard-row-summary">
        {t("season.leaderboard.rowAria", {
          position,
          username,
          tier: rankName,
          sp: total.toLocaleString(),
        })}
        {games !== undefined && ` ${t("season.leaderboard.gamesAria", { games })}`}
        {isSelf && ` ${t("season.leaderboard.you")}`}
      </span>

      <span
        data-testid="leaderboard-position"
        aria-hidden="true"
        className="text-ink-mute w-7 shrink-0 text-right text-xs tabular-nums"
      >
        {position}
      </span>

      <TierBadge tier={safeTier} size="sm" data-testid="leaderboard-tier-badge" />

      <Avatar name={username} avatarUrl={avatarUrl} size={24} />

      <Link
        data-testid="leaderboard-username"
        to={isSelf ? "/profile" : `/players/${userId}`}
        aria-label={
          isSelf
            ? t("season.leaderboard.yourProfileAria")
            : t("friends.viewProfileAria", { username })
        }
        title={username}
        className="text-ink focus-visible:ring-ring/70 min-w-0 flex-1 truncate rounded-sm text-sm font-medium underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
      >
        {username}
      </Link>

      {isSelf && (
        <span
          data-testid="leaderboard-you"
          aria-hidden="true"
          className="bg-accent/15 text-accent shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-semibold tracking-[0.3px] uppercase"
        >
          {t("season.leaderboard.you")}
        </span>
      )}

      <span
        data-testid="leaderboard-tier"
        aria-hidden="true"
        title={t("season.leaderboard.columns.tier")}
        className="shrink-0 text-xs font-semibold"
        style={{ color: SEASON_TIER_COLOR[safeTier] }}
      >
        {rankName}
      </span>

      <span
        data-testid="leaderboard-sp"
        aria-hidden="true"
        title={t("season.leaderboard.columns.sp")}
        className="text-ink shrink-0 text-xs font-semibold tabular-nums"
      >
        {t("season.leaderboard.spValue", { sp: total.toLocaleString() })}
      </span>

      {games !== undefined && (
        <span
          data-testid="leaderboard-games"
          aria-hidden="true"
          title={t("season.leaderboard.columns.games")}
          className="text-ink-mute hidden w-10 shrink-0 text-right text-xs tabular-nums sm:inline"
        >
          {games.toLocaleString()}
        </span>
      )}
    </li>
  );
}
