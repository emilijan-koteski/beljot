import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import { SectionHeader } from "@/features/profile/components/SectionHeader";
import { queryKeys } from "@/shared/api/queryKeys";
import type { SeasonSelector } from "@/shared/api/season";
import { LeaderboardRow } from "@/shared/components/season/LeaderboardRow";
import { Chips } from "@/shared/components/ui/chips";
import {
  useSeasonLeaderboardInfiniteQuery,
  useSeasonsQuery,
} from "@/shared/hooks/queries/useSeasonLeaderboard";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import { useAuthStore } from "@/shared/stores/authStore";
import type { LeaderboardRow as LeaderboardRowData } from "@/shared/types/apiTypes";

/** Rows per request on the full page. Comfortably under the server's cap of 50. */
const PAGE_SIZE = 25;

/**
 * Re-arm delay for the boundary effect, matching RankBanner's. Only reachable
 * when this client's clock runs ahead of the server's, where the early refetch
 * returns the same still-active window.
 */
const SEASON_BOUNDARY_RETRY_MS = 5 * 60 * 1000;

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

  // The season picker (Story 13.3). `null` means "the current window" — the
  // DEFAULT, and deliberately not a season id: the page must render the active
  // ladder before (and even without) the seasons list resolving, and
  // `season=current` keeps the request URL independent of which quarter it is.
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const seasonsQuery = useSeasonsQuery();
  const seasons = seasonsQuery.data?.items ?? [];
  const queryClient = useQueryClient();

  // The same shared 30s tick the lobby's RankBanner uses. THE PAGE MUST OBSERVE
  // THE BOUNDARY TOO: this is the only surface whose entire content is the
  // ladder, and RankBanner — the sole other observer — is unmounted while this
  // route is active. Without the subscription a page left open across the
  // quarter boundary keeps the "Current" marker on the dead window and keeps
  // rendering its frozen standings under the present-tense heading.
  const tick = useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  // Re-derived on every render, and the tick subscription above is what makes
  // renders happen on a 30s cadence — so this ages instead of freezing at mount.
  const nowMs = Date.now();

  // The window covering "now" — identified by its own timestamps rather than
  // by list position, so a pre-created future row could never mislabel it.
  const currentId = seasons.find(
    (s) => Date.parse(s.startedAt) <= nowMs && nowMs < Date.parse(s.endsAt),
  )?.id;

  // The page's own boundary effect, mirroring RankBanner's.
  //
  // IT WATCHES THE NEWEST WINDOW WE KNOW OF, NOT "the current one": at the
  // moment the boundary passes, NO listed window covers now (that is the whole
  // problem — the next one exists only on the server), so keying on `currentId`
  // would go undefined exactly when the effect needs to fire. Once the newest
  // endsAt we hold is in the past, this page's list AND its `current` ladder
  // are both provably stale. The refetch brings the new window in, its endsAt
  // is in the future, and the guard is re-armed for the next quarter. The `at`
  // stamp is the same clock-skew escape RankBanner carries.
  let newestEndsAt: string | undefined;
  for (const s of seasons) {
    if (newestEndsAt === undefined || Date.parse(s.endsAt) > Date.parse(newestEndsAt)) {
      newestEndsAt = s.endsAt;
    }
  }
  const invalidatedForRef = useRef<{ endsAt: string; at: number } | null>(null);
  useEffect(() => {
    if (newestEndsAt === undefined) return;
    const end = Date.parse(newestEndsAt);
    const now = Date.now();
    if (!Number.isFinite(end) || now < end) return;
    const last = invalidatedForRef.current;
    if (last?.endsAt === newestEndsAt && now - last.at < SEASON_BOUNDARY_RETRY_MS) return;
    invalidatedForRef.current = { endsAt: newestEndsAt, at: now };
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.leaderboardAll() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.list() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.current() });
  }, [tick, newestEndsAt, queryClient]);

  // PAST-SEASON COPY. A frozen quarter must not be described in the present
  // tense ("who is climbing fastest this season", "nobody has earned SP yet
  // this season" — the latter is simply false about a season that is over).
  // Decided from the selected window's OWN endsAt, so it stays right even while
  // `currentId` is still resolving.
  const selectedSeason = selectedId === null ? undefined : seasons.find((s) => s.id === selectedId);
  const isPastSeason = selectedSeason !== undefined && Date.parse(selectedSeason.endsAt) <= nowMs;

  const seasonParam: SeasonSelector = selectedId === null ? "current" : selectedId;
  const query = useSeasonLeaderboardInfiniteQuery(PAGE_SIZE, seasonParam);
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
        <p className="text-ink-dim m-0">
          {t(isPastSeason ? "season.leaderboard.emptyPast" : "season.leaderboard.empty")}
        </p>
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
        sub={t(isPastSeason ? "season.leaderboard.subPast" : "season.leaderboard.sub", {
          season: selectedSeason?.name ?? "",
        })}
      />

      {/* Season picker (Story 13.3): newest-first straight off the server's
          order, season tokens rendered VERBATIM. Absent until the list
          resolves — the page defaults to the current window regardless, so
          there is nothing to pick from an empty or failed list (a picker whose
          only row is unclickable would be noise, and the picker failing must
          not take the ladder with it). Picking the current window maps back to
          the `current` selector rather than its id, so the default cache entry
          is reused.

          `seasons.length > 1`, NOT `> 0`: on a fresh deployment — and through
          the whole of the product's first quarter — the list holds exactly one
          window, and a picker offering a single already-selected chip is a
          control that cannot do anything.

          When `currentId` is undefined (no listed window covers this client's
          clock — skew right at a boundary, or a list fetched a moment before
          the rollover) the value falls back to the sentinel so no chip reads as
          selected, and picking any window sends its id. That path is correct
          rather than merely tolerable: an explicit id is exactly what the
          reader asked for, and `selectedId` still collapses back to `current`
          as soon as a covering window is identifiable. */}
      {seasons.length > 1 && (
        <Chips
          value={selectedId ?? currentId ?? -1}
          onValueChange={(id) =>
            setSelectedId(currentId !== undefined && id === currentId ? null : id)
          }
          options={seasons.map((s) => ({
            value: s.id,
            label:
              s.id === currentId ? (
                <>
                  {s.name}
                  <span className="text-[10px] tracking-[0.3px] uppercase opacity-70">
                    {t("season.picker.current")}
                  </span>
                </>
              ) : (
                s.name
              ),
          }))}
          ariaLabel={t("season.picker.label")}
          testId="leaderboard-season-picker"
          className="mb-4"
        />
      )}

      {body}
    </div>
  );
}
