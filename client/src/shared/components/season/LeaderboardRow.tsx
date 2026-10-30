import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  seasonRankLabel,
  seasonWinRate,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/** Coerces a possibly absent server integer to a finite number, treating 0 as real. */
function finiteOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? Math.max(0, value as number) : 0;
}

type Layout = "list" | "table";

type Props = {
  position: number;
  userId: number;
  username: string;
  avatarUrl?: string | null;
  sp: number;
  tier: string;
  division: number | null;
  gamesPlayed?: number;
  gamesWon?: number;
  isSelf?: boolean;
  layout?: Layout;
  showRank?: boolean;
  className?: string;
};

const TABLE_COLUMNS = {
  rank: "grid-cols-[3rem_minmax(0,1fr)_9rem_6rem_4.5rem_6rem]",
  plain: "grid-cols-[2.5rem_minmax(0,1fr)_6rem_4.5rem_6rem]",
} as const;

function tableColumns(showRank: boolean): string {
  return showRank ? TABLE_COLUMNS.rank : TABLE_COLUMNS.plain;
}

/** The column labels over a table of `LeaderboardRow`s in the `table` layout. */
export function LeaderboardTableHeader({ showRank = false }: { showRank?: boolean }) {
  const { t } = useTranslation();
  return (
    <div
      aria-hidden="true"
      className={cn(
        "text-ink-mute border-border bg-surface-sunken/60 grid items-center gap-3 border-b px-4 py-2 font-mono text-2xs font-semibold tracking-caps uppercase",
        tableColumns(showRank),
      )}
    >
      <span className="text-right">#</span>
      <span>{t("season.leaderboard.columns.player")}</span>
      {showRank && <span>{t("season.leaderboard.columns.tier")}</span>}
      <span className="text-right">{t("season.leaderboard.columns.record")}</span>
      <span className="text-right">{t("season.leaderboard.columns.winRate")}</span>
      <span className="text-right">{t("season.leaderboard.columns.sp")}</span>
    </div>
  );
}

/** A season record as wins in green, a dash, then losses in red. */
export function SeasonRecord({ won, played }: { won: number; played: number }) {
  return (
    <span data-testid="leaderboard-record" className="tabular-nums">
      <span className="text-accent-deep font-semibold">{won}</span>
      <span className="text-ink-mute">–</span>
      <span className="text-danger/80 font-semibold">{played - won}</span>
    </span>
  );
}

function SeasonPoints({ sp }: { sp: number }) {
  const { t } = useTranslation();
  return (
    <span
      data-testid="leaderboard-sp"
      className="text-ink shrink-0 text-sm font-bold whitespace-nowrap tabular-nums"
    >
      {sp.toLocaleString()}{" "}
      <span className="text-ink-mute text-2xs font-semibold">{t("season.leaderboard.spUnit")}</span>
    </span>
  );
}

/**
 * One leaderboard row: a phone list item, or a table row on wider screens; also the pinned own row.
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
  gamesWon,
  isSelf = false,
  layout = "list",
  showRank = false,
  className,
}: Props) {
  const { t } = useTranslation();

  const total = finiteOrZero(sp);
  const safeTier = normalizeSeasonTier(tier, total);
  const rankName = seasonRankLabel(t, safeTier, normalizeSeasonDivision(safeTier, division));
  const played = gamesPlayed === undefined ? undefined : finiteOrZero(gamesPlayed);
  const won =
    played === undefined || gamesWon === undefined
      ? undefined
      : Math.min(finiteOrZero(gamesWon), played);
  const rate = won === undefined || played === undefined ? undefined : seasonWinRate(won, played);
  const table = layout === "table";

  const rank = (
    <span
      data-testid="leaderboard-tier"
      aria-hidden="true"
      className="shrink-0 font-semibold"
      style={{ color: SEASON_TIER_COLOR[safeTier] }}
    >
      {rankName}
    </span>
  );

  const name = (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <Link
        data-testid="leaderboard-username"
        to={isSelf ? "/profile" : `/players/${userId}`}
        aria-label={
          isSelf
            ? t("season.leaderboard.yourProfileAria")
            : t("friends.viewProfileAria", { username })
        }
        title={username}
        className="text-ink focus-visible:ring-ring/70 min-w-0 truncate rounded-sm text-sm font-semibold underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
      >
        {username}
      </Link>
      {isSelf && (
        <Badge tone="accent" size="sm" data-testid="leaderboard-you" aria-hidden="true">
          {t("season.leaderboard.you")}
        </Badge>
      )}
    </span>
  );

  return (
    <li
      data-testid="leaderboard-row"
      data-user-id={userId}
      {...(isSelf ? { "data-self": "true" } : {})}
      className={cn(
        table
          ? cn("grid items-center gap-3 px-4 py-2.5", tableColumns(showRank))
          : "flex items-center gap-2.5 px-3 py-2.5",
        isSelf ? "bg-accent-soft/70" : table && "hover:bg-surface-sunken/50",
        isSelf && !table && "rounded-lg",
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
        {won !== undefined &&
          played !== undefined &&
          ` ${t("season.leaderboard.recordAria", { won, lost: played - won })}`}
        {isSelf && ` ${t("season.leaderboard.you")}`}
      </span>

      <span
        data-testid="leaderboard-position"
        aria-hidden="true"
        className={cn(
          "shrink-0 text-right font-semibold tabular-nums",
          table ? "text-sm" : "w-7 text-xs",
          isSelf ? "text-accent-deep" : "text-ink-mute",
        )}
      >
        {position}
      </span>

      <span className={cn("flex min-w-0 items-center gap-2.5", !table && "flex-1")}>
        <Avatar name={username} avatarUrl={avatarUrl} size={table ? 32 : 34} />
        <span className="flex min-w-0 flex-col leading-tight">
          {name}
          {!showRank && (
            <span aria-hidden="true" className="inline-flex items-center gap-1.5 text-2xs">
              {rank}
              {!table && won !== undefined && played !== undefined && (
                <span className="text-ink-mute inline-flex items-center gap-1">
                  <SeasonRecord won={won} played={played} />
                  <span>·</span>
                  <span data-testid="leaderboard-win-rate">{rate}%</span>
                </span>
              )}
            </span>
          )}
        </span>
      </span>

      {table && showRank && (
        <span aria-hidden="true" className="flex items-center gap-2 text-xs">
          <TierBadge tier={safeTier} size="sm" glow={false} data-testid="leaderboard-tier-badge" />
          {rank}
        </span>
      )}

      {table && (
        <>
          <span aria-hidden="true" className="text-right text-sm">
            {won !== undefined && played !== undefined && (
              <SeasonRecord won={won} played={played} />
            )}
          </span>
          <span
            aria-hidden="true"
            className="text-ink-dim text-right text-sm tabular-nums"
            data-testid={rate === undefined ? undefined : "leaderboard-win-rate"}
          >
            {rate !== undefined && `${rate}%`}
          </span>
        </>
      )}

      <span aria-hidden="true" className={cn(table && "text-right")}>
        <SeasonPoints sp={total} />
      </span>
    </li>
  );
}
