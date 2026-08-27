import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { normalizeSeasonTier } from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/**
 * Coerce a possibly-absent server integer into a renderable one.
 *
 * `Number.isFinite`, never truthiness: 0 is a real Go value for every field on
 * this row (0 SP, 0 games), and `undefined` arrives when a client bundle is
 * newer than the server. Left raw that produced `NaN` on screen.
 *
 * DELIBERATELY NOT `seasonSpOrZero`. That helper lives in seasonTier.ts as part
 * of the SP ladder's display math, and this row also has to sanitize a MATCH
 * COUNT. Borrowing it worked only because it happens to be a generic clamp
 * today; the moment it gains anything SP-specific (a tier floor, a cap at the
 * Grandmaster band) the games column would inherit it silently.
 */
function finiteOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? Math.max(0, value as number) : 0;
}

type Props = {
  /** 1-based slot in the WHOLE season order, not an index into the page. */
  position: number;
  userId: number;
  username: string;
  sp: number;
  /** The server's raw tier token — normalized here, never trusted as-is. */
  tier: string;
  /** Omitted by the lobby widget, where the aside is too narrow to spend a column. */
  gamesPlayed?: number;
  /** True for the viewer's own row: tinted ground, a "you" pill, and `data-self`. */
  isSelf?: boolean;
  className?: string;
};

/**
 * One leaderboard row — the SINGLE renderer behind all three places a standing
 * appears: the lobby's top-10 widget, the full page's list, and the pinned
 * own-row the page shows when the viewer sits outside the loaded pages.
 *
 * That last case is why `username` is a plain prop rather than something read
 * from the row data: the server's viewer block deliberately carries no username
 * (the viewer is the authenticated caller, so the client already holds their
 * name in `authStore`), and the pinned row supplies it locally while rendering
 * through this exact component.
 *
 * The tier token arrives as a plain string and goes through `normalizeSeasonTier`
 * here — the version-skew guard. A newer server that ships a ninth tier must
 * still render a real colour and a real i18n label on a stale bundle, falling
 * back to the SP's own bucket, rather than printing `season.tier.mythic`.
 *
 * ACCESSIBILITY: ONE SPOKEN SUMMARY, EVERYTHING ELSE HIDDEN.
 * The visible cells are terse by design — a bare "4", "kiro", "4,000 SP", "31" —
 * which reads as a meaningless number soup cell by cell. So the row carries an
 * `sr-only` sentence naming every value, and every visible cell (plus the badge)
 * is `aria-hidden`. The earlier version put an `aria-label` on the `<li>` while
 * leaving the children exposed, which is the one arrangement that can be
 * announced TWICE — label and contents — depending on the AT.
 *
 * That also fixes what the badge's own `aria-hidden` was resting on. TierBadge
 * justifies hiding itself with "every surface renders the tier NAME as text
 * beside it" — true of RankBanner, false here, where the tier only ever appeared
 * as a tooltip. The sr-only summary is now that text.
 *
 * NOT a link. A leaderboard row is a standing, not a navigation affordance;
 * profile navigation lives in the friend list and in match history.
 */
export function LeaderboardRow({
  position,
  userId,
  username,
  sp,
  tier,
  gamesPlayed,
  isSelf = false,
  className,
}: Props) {
  const { t } = useTranslation();

  const total = finiteOrZero(sp);
  const safeTier = normalizeSeasonTier(tier, total);
  const tierName = t(`season.tier.${safeTier}`);
  const games = gamesPlayed === undefined ? undefined : finiteOrZero(gamesPlayed);

  return (
    <li
      data-testid="leaderboard-row"
      data-user-id={userId}
      // Present ONLY on the viewer's own row — `data-self` is the attribute, so
      // absence is the negative case rather than a "false" string.
      {...(isSelf ? { "data-self": "true" } : {})}
      className={cn(
        "border-border flex items-center gap-2.5 rounded-[10px] border px-2.5 py-1.5",
        isSelf ? "bg-accent-soft border-accent/40" : "bg-surface-elevated",
        className,
      )}
    >
      {/* The row's entire accessible name. Everything below is aria-hidden, so
          this sentence is announced once and nothing competes with it. */}
      <span className="sr-only" data-testid="leaderboard-row-summary">
        {t("season.leaderboard.rowAria", {
          position,
          username,
          tier: tierName,
          sp: total.toLocaleString(),
        })}
        {/* `games`, deliberately NOT `count`: i18next treats a `count` variable
            as a pluralization request and starts looking for `gamesAria_one` /
            `gamesAria_other` keys that do not exist in these bundles. */}
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

      <span
        data-testid="leaderboard-username"
        aria-hidden="true"
        // The player's OWN name, so a truncated one can still be read on hover.
        // It used to carry the tier name instead, which meant hovering "kiro"
        // reported "Gold" — the wrong tooltip on the wrong element.
        title={username}
        className="text-ink min-w-0 flex-1 truncate text-sm font-medium"
      >
        {username}
      </span>

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
        data-testid="leaderboard-sp"
        aria-hidden="true"
        title={t("season.leaderboard.columns.sp")}
        className="text-ink shrink-0 text-xs font-semibold tabular-nums"
      >
        {t("season.leaderboard.spValue", { sp: total.toLocaleString() })}
      </span>

      {/* Explicit undefined check, not truthiness: 0 games played is a real
          value that must still render as "0". */}
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
