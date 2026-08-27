import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { normalizeSeasonTier, SEASON_TIER_COLOR } from "@/shared/lib/seasonTier";
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
  /** Optional: a caller too narrow to spend a column on it may leave it off. */
  gamesPlayed?: number;
  /** True for the viewer's own row: tinted ground, a "you" pill, and `data-self`. */
  isSelf?: boolean;
  className?: string;
};

/**
 * One leaderboard row — the SINGLE renderer behind both places a standing
 * appears: the leaderboard page's list, and the pinned own-row that page shows
 * when the viewer sits outside the loaded pages.
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
 * ACCESSIBILITY: ONE SPOKEN SUMMARY, EVERYTHING ELSE HIDDEN — EXCEPT THE LINK.
 * The visible cells are terse by design — a bare "4", "Gold", "4,000 SP", "31" —
 * which reads as a meaningless number soup cell by cell. So the row carries an
 * `sr-only` sentence naming every value, and every visible cell (plus the badge)
 * is `aria-hidden`. The earlier version put an `aria-label` on the `<li>` while
 * leaving the children exposed, which is the one arrangement that can be
 * announced TWICE — label and contents — depending on the AT.
 *
 * The USERNAME is the deliberate exception, because it is now a link, and
 * `aria-hidden` on a focusable element is the one thing that arrangement must
 * never do: it hides the control from the accessibility tree while leaving it
 * in the tab order, so a keyboard screen-reader user lands on something that
 * announces nothing at all. It carries its own label instead.
 *
 * The tier NAME is rendered as text beside the badge (Story 13.2 shipped the
 * badge alone, with the tier only in a tooltip), which is also what TierBadge's
 * own `aria-hidden` rests on — it justifies hiding itself with "every surface
 * renders the tier NAME as text beside it", and now this one does too.
 *
 * THE USERNAME LINKS TO THE PLAYER'S PROFILE, and the viewer's own row links to
 * `/profile` rather than to `/players/<their own id>`: the self page is the
 * richer surface (linked accounts, the deck picker, the editable username), and
 * the public page would show the viewer a read-only, friend-button-less copy of
 * themselves. `/players/:id` is otherwise the same target the friend list, the
 * partner/rival cards and match history's seat chips all use.
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
      {/* The row's spoken summary. Every visible cell below is aria-hidden —
          the ONE exception being the username link, which cannot be (see the
          header) and carries its own destination label instead. */}
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

      <Link
        data-testid="leaderboard-username"
        to={isSelf ? "/profile" : `/players/${userId}`}
        // NOT aria-hidden (see the header): the row summary above already names
        // this player, so the link states its DESTINATION instead of repeating
        // the name on its own. The pinned own-row can fall back to "You" as its
        // username, which is why the self case takes a label of its own rather
        // than interpolating whatever name is on hand.
        aria-label={
          isSelf
            ? t("season.leaderboard.yourProfileAria")
            : t("friends.viewProfileAria", { username })
        }
        // The player's OWN name, so a truncated one can still be read on hover.
        // It used to carry the tier name instead, which meant hovering "kiro"
        // reported "Gold" — the wrong tooltip on the wrong element.
        title={username}
        className="text-ink focus-visible:ring-ring/50 min-w-0 flex-1 truncate rounded-sm text-sm font-medium underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
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

      {/* The tier as WORDS, not only as a coloured shield. The badge alone
          asked every reader to have memorised eight ramp colours, which is a
          quiz rather than a label — and the sr-only summary was the only place
          the tier was ever spelled out. Same treatment as SeasonArchiveRow's
          tier cell, so the two season lists read identically. */}
      <span
        data-testid="leaderboard-tier"
        aria-hidden="true"
        title={t("season.leaderboard.columns.tier")}
        className="shrink-0 text-xs font-semibold"
        style={{ color: SEASON_TIER_COLOR[safeTier] }}
      >
        {tierName}
      </span>

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
