import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { LeaderboardRow } from "@/shared/components/season/LeaderboardRow";
import { useSeasonLeaderboardQuery } from "@/shared/hooks/queries/useSeasonLeaderboard";

/** The lobby widget is a TOP TEN — the server's own default, sent explicitly. */
const WIDGET_SIZE = 10;

/**
 * The lobby's seasonal leaderboard panel (Story 13.2 AC1): the top ten by Season
 * Points, always visible on the way past, with a link into the full page.
 *
 * AMBIENT, NOT LIVE. It polls (see useSeasonLeaderboardQuery) rather than
 * subscribing: standings are pull-only by epic decision and there is no
 * WebSocket event for them.
 *
 * The viewer's own row is marked here the same way it is on the full page — but
 * this widget never PINS an off-page row. Ten slots in a lobby aside are
 * ambient motivation; a player ranked 340th does not need their own line
 * squeezed under a top ten they are not in, and the panel's link is right there.
 *
 * Card idiom copied from FriendList so the lobby's panels read as one family.
 */
export function LeaderboardPanel() {
  const { t } = useTranslation();
  const query = useSeasonLeaderboardQuery(WIDGET_SIZE);

  const items = query.data?.items ?? [];
  // ONE RULE FOR "IS THIS ME", SHARED WITH LeaderboardPage: the SERVER's viewer
  // block decides, never the auth store's id.
  //
  // This panel used to compare `row.userId === authStore.user.id`, which is a
  // second, subtly different contract on the same question — the server marks a
  // standing only for a player with SP, so an authed player with a 0-SP row (or
  // a soft-deleted account on a stale token) got a highlighted row here while
  // the full page, one click away, showed them unmarked. The server block is the
  // single source; if it is null, nothing is mine.
  const standing = query.data?.viewer ?? null;

  // THE ERROR BRANCH MUST NOT DESTROY ROWS THE READER ALREADY HAS. This panel
  // polls every 60s, so `isError` goes true on any single transient failure —
  // and checking it before `data` replaced a populated top ten with a one-line
  // error until the next tick. React Query keeps the last successful data, so a
  // refetch failure over existing rows keeps the rows.
  const showError = query.isError && items.length === 0;

  let body;
  if (query.isPending) {
    body = (
      <p
        className="text-ink-mute px-2.5 py-2 text-xs"
        data-testid="leaderboard-panel-loading"
        role="status"
        aria-busy="true"
      >
        {t("season.leaderboard.loading")}
      </p>
    );
  } else if (showError) {
    body = (
      <p
        className="text-destructive px-2.5 py-2 text-xs"
        data-testid="leaderboard-panel-error"
        role="status"
      >
        {t("season.leaderboard.error")}
      </p>
    );
  } else if (items.length === 0) {
    body = (
      <p className="text-ink-dim px-2.5 py-2 text-xs" data-testid="leaderboard-panel-empty">
        {t("season.leaderboard.empty")}
      </p>
    );
  } else {
    body = (
      <ul
        className="m-0 flex list-none flex-col gap-1.5 p-0"
        data-testid="leaderboard-panel-list"
        aria-label={t("season.leaderboard.title")}
      >
        {items.map((row) => (
          <LeaderboardRow
            key={row.userId}
            position={row.position}
            userId={row.userId}
            username={row.username}
            sp={row.sp}
            tier={row.tier}
            // No games column: the aside is 320px wide, and AC1 asks for
            // position, username, tier badge and SP.
            isSelf={standing !== null && row.userId === standing.userId}
          />
        ))}
      </ul>
    );
  }

  return (
    <div
      className="bg-surface border-border mb-3.5 rounded-lg border p-3.5"
      data-testid="leaderboard-panel"
    >
      <div className="mb-2 flex items-center gap-2">
        <h2 className="text-ink-dim text-xs font-semibold">{t("season.leaderboard.heading")}</h2>
        <Link
          to="/leaderboard"
          data-testid="leaderboard-panel-view-all"
          className="text-accent focus-visible:ring-ring/50 ml-auto shrink-0 rounded-md text-xs underline-offset-2 transition-colors hover:underline focus-visible:ring-3 focus-visible:outline-none"
        >
          {t("season.leaderboard.viewAll")}
        </Link>
      </div>

      {/* A poll that fails over rows we already have is reported inline rather
          than replacing them (see showError above) — quiet, but not silent. */}
      {query.isError && items.length > 0 && (
        <p
          className="text-ink-mute mb-1.5 px-2.5 text-[11px]"
          data-testid="leaderboard-panel-stale"
          role="status"
        >
          {t("season.leaderboard.stale")}
        </p>
      )}

      {body}
    </div>
  );
}
