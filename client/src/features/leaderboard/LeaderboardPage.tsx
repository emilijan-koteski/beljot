import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import { SectionHeader } from "@/features/profile/components/SectionHeader";
import { LeaderboardRow } from "@/shared/components/season/LeaderboardRow";
import { useSeasonLeaderboardInfiniteQuery } from "@/shared/hooks/queries/useSeasonLeaderboard";
import { useAuthStore } from "@/shared/stores/authStore";
import type { LeaderboardRow as LeaderboardRowData } from "@/shared/types/apiTypes";

/** Rows per request on the full page. Comfortably under the server's cap of 50. */
const PAGE_SIZE = 25;

/**
 * The full seasonal leaderboard (Story 13.2 AC2), reached from the top nav's
 * "Leaderboard" tab.
 *
 * LOAD-MORE, NOT A NUMBERED PAGER: this codebase's pagination idiom is a single
 * button that appends the next offset page, with a "showing N of M" caption
 * underneath (profile match history). The four body branches — skeleton, error,
 * empty, list — are the same four, in the same order.
 *
 * The viewer's own row is MARKED wherever it appears in the list, and PINNED
 * below the list when it does not: a player ranked 340th would otherwise have to
 * page thirteen times to find out where they stand. The pinned row renders
 * through the same LeaderboardRow, with the username supplied from `authStore` —
 * the server's viewer block deliberately carries no name, because the viewer is
 * the authenticated caller.
 */
export function LeaderboardPage() {
  const { t } = useTranslation();
  const query = useSeasonLeaderboardInfiniteQuery(PAGE_SIZE);
  const viewer = useAuthStore((s) => s.user);

  const items = useMemo<LeaderboardRowData[]>(() => {
    if (!query.data) return [];
    // DEDUPED BY userId, not just concatenated. Offset paging over live data can
    // return the same player twice: if someone above the fold gains SP between
    // page 1 and page 2, every row below them shifts down one and the row that
    // was last on page 1 reappears first on page 2. That is the accepted
    // tradeoff of offset paging here (a keyset cursor is the alternative), but
    // duplicate React keys are not — they make React reconcile the wrong nodes
    // and warn in the console. First occurrence wins, which is the one whose
    // `position` matches the slot the reader already saw.
    const seen = new Set<number>();
    const out: LeaderboardRowData[] = [];
    for (const page of query.data.pages) {
      for (const row of page.items) {
        if (seen.has(row.userId)) continue;
        seen.add(row.userId);
        out.push(row);
      }
    }
    return out;
  }, [query.data]);

  // READ THE LAST PAGE, NOT THE FIRST. Both of these come from the response, and
  // every response carries a fresh copy — so on a reader who has paged three
  // deep, `pages[0]` is the oldest snapshot on hand. Their own position and the
  // season's total both move as matches settle, and showing the stalest value
  // available is a strange choice when a newer one is right there.
  const latest = query.data?.pages.at(-1);
  const standing = latest?.viewer ?? null;
  const total = latest?.total ?? 0;

  // Pin only when there IS a standing and it is not already on screen. Compared
  // by user id, not by position: the two come from separate queries, and a match
  // on identity is the thing that actually decides whether the row is visible.
  const onPage = standing !== null && items.some((row) => row.userId === standing.userId);
  const pinned = standing !== null && !onPage ? standing : null;

  // THE ERROR BRANCH MUST NOT DESTROY ROWS THE READER ALREADY HAS. Checked
  // before `data`, a failed `fetchNextPage()` — the click at the bottom of 75
  // loaded rows — replaced all of them with a single line of error text. React
  // Query keeps the successful pages, so the failure is reported beside them
  // instead, with a retry that re-runs the fetch that actually failed.
  const showError = query.isError && items.length === 0;

  let body;
  if (query.isPending) {
    body = (
      <div
        className="space-y-1.5"
        data-testid="leaderboard-loading"
        role="status"
        aria-busy="true"
        aria-label={t("season.leaderboard.loading")}
      >
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="bg-surface h-10 animate-pulse rounded-[10px]" />
        ))}
      </div>
    );
  } else if (showError) {
    body = (
      <p className="text-destructive text-sm" data-testid="leaderboard-error" role="status">
        {t("season.leaderboard.error")}
      </p>
    );
  } else if (items.length === 0) {
    body = (
      <div
        className="bg-surface border-border rounded-lg border border-dashed p-10 text-center text-sm"
        data-testid="leaderboard-empty"
      >
        <p className="text-ink-dim m-0">{t("season.leaderboard.empty")}</p>
      </div>
    );
  } else {
    // DRIVEN OFF hasNextPage, not `items.length < total`. The two disagree:
    // `getNextPageParam` decides from the LAST page's total while that
    // comparison used the first page's, so a total that shrank (a player deleted
    // their account) left a button whose click did nothing at all — React Query
    // has no next page param, so `fetchNextPage()` is a silent no-op. One source
    // for "is there more", and it is the one the fetch itself consults.
    const showLoadMore = query.hasNextPage;
    body = (
      <>
        <ul
          className="m-0 flex list-none flex-col gap-1.5 p-0"
          data-testid="leaderboard-list"
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
              gamesPlayed={row.gamesPlayed}
              isSelf={standing !== null && row.userId === standing.userId}
            />
          ))}
        </ul>

        {pinned && (
          <div className="mt-2.5" data-testid="leaderboard-pinned">
            <p className="text-ink-mute mb-1.5 text-[11px] tracking-[0.3px] uppercase">
              {t("season.leaderboard.yourPosition")}
            </p>
            <ul className="m-0 flex list-none flex-col p-0">
              <LeaderboardRow
                position={pinned.position}
                userId={pinned.userId}
                // From the auth store: the viewer IS the caller, so their name
                // never crosses the wire on this block. It falls back to the
                // "you" label rather than an empty string — the store can be
                // unhydrated (a hard refresh straight onto this URL) while the
                // query, which needs only the token, has already resolved, and a
                // nameless pinned row is worse than a generic one.
                username={viewer?.username || t("season.leaderboard.you")}
                sp={pinned.sp}
                tier={pinned.tier}
                gamesPlayed={pinned.gamesPlayed}
                isSelf
              />
            </ul>
          </div>
        )}

        {/* A failed fetchNextPage over rows we already have: reported here, with
            the rows left intact, and retryable. */}
        {query.isError && (
          <div className="mt-2.5 flex items-center gap-3" data-testid="leaderboard-page-error">
            <p className="text-destructive m-0 text-xs" role="status">
              {t("season.leaderboard.error")}
            </p>
            <button
              type="button"
              onClick={() => query.fetchNextPage()}
              className="text-accent cursor-pointer text-xs underline-offset-2 hover:underline"
              data-testid="leaderboard-retry"
            >
              {t("season.leaderboard.retry")}
            </button>
          </div>
        )}

        {showLoadMore && (
          <button
            type="button"
            onClick={() => query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
            className="bg-surface border-border text-ink hover:bg-surface-elevated mt-2.5 w-full rounded-lg border px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60"
            data-testid="leaderboard-load-more"
          >
            {query.isFetchingNextPage
              ? t("season.leaderboard.loading")
              : t("season.leaderboard.loadMore")}
          </button>
        )}

        {/* Doubles as the append announcement: clicking Load more inserts 25
            rows with no visual event a screen reader can observe, so this
            live region reports the new count. `aria-live` rather than
            role="status" on a caption that is always present, so only the
            CHANGE is spoken. */}
        <p
          className="text-ink-mute mt-4 text-center text-xs"
          data-testid="leaderboard-count"
          aria-live="polite"
          aria-atomic="true"
        >
          {t("season.leaderboard.showing", { shown: items.length, total })}
        </p>
      </>
    );
  }

  return (
    // Narrower than the lobby's max-w-330: a leaderboard row is four short
    // cells, and stretched across a 1330px column the name and the SP total end
    // up at opposite edges of the screen with nothing between them.
    <div className="mx-auto max-w-220 px-4 py-8 pb-32 md:px-7" data-testid="leaderboard-page">
      <SectionHeader
        eyebrow={t("season.leaderboard.eyebrow")}
        title={t("season.leaderboard.title")}
        sub={t("season.leaderboard.sub")}
      />
      {body}
    </div>
  );
}
