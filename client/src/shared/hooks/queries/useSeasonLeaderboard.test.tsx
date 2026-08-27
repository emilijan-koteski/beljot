import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { queryKeys } from "@/shared/api/queryKeys";
import { getSeasonLeaderboard } from "@/shared/api/season";
import {
  useSeasonLeaderboardInfiniteQuery,
  useSeasonLeaderboardQuery,
} from "@/shared/hooks/queries/useSeasonLeaderboard";
import type { LeaderboardResponse, LeaderboardRow } from "@/shared/types/apiTypes";

vi.mock("@/shared/api/season", () => ({
  getSeasonLeaderboard: vi.fn(),
}));

const mockGet = vi.mocked(getSeasonLeaderboard);

function rows(from: number, n: number): LeaderboardRow[] {
  return Array.from({ length: n }, (_, i) => ({
    position: from + i,
    userId: from + i,
    username: `p${from + i}`,
    sp: 10_000 - (from + i) * 10,
    tier: "gold",
    gamesPlayed: 5,
  }));
}

function page(over: Partial<LeaderboardResponse> = {}): LeaderboardResponse {
  return { items: [], total: 0, limit: 25, offset: 0, viewer: null, ...over };
}

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

describe("useSeasonLeaderboardQuery (widget)", () => {
  beforeEach(() => mockGet.mockReset());

  it("reads the requested page size from offset zero", async () => {
    mockGet.mockResolvedValue(page({ items: rows(1, 10), total: 10, limit: 10 }));

    const { result } = renderHook(() => useSeasonLeaderboardQuery(10), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockGet).toHaveBeenCalledWith(10, 0);
  });

  it("polls on an interval, unlike the pushed useCurrentSeason", async () => {
    mockGet.mockResolvedValue(page());

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => useSeasonLeaderboardQuery(10), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={qc}>{children}</QueryClientProvider>
      ),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Standings are PULL-ONLY by epic decision: other players' SP has no push
    // channel, so a missing interval freezes the widget for the whole session.
    // Read off the live observer rather than by advancing timers, which would
    // make this a slow and flaky clock test.
    const entry = qc.getQueryCache().find({ queryKey: queryKeys.season.leaderboard(10) });
    const options = entry?.observers[0]?.options as
      | { refetchInterval?: number; refetchOnWindowFocus?: boolean }
      | undefined;
    expect(options?.refetchInterval).toBeGreaterThan(0);
    // Deliberately much longer than useLobbyStats' 10s: a ladder built from whole
    // matches moves on the order of minutes.
    expect(options?.refetchInterval).toBeGreaterThanOrEqual(30_000);
    expect(options?.refetchOnWindowFocus).toBe(true);
  });

  it("keys entries by page size so the widget and the page cannot collide", () => {
    // The widget reads 10 rows and the full page 25. One shared key would let a
    // 10-row response be served to a component expecting 25.
    expect(queryKeys.season.leaderboard(10)).not.toEqual(queryKeys.season.leaderboard(25));
    expect(queryKeys.season.leaderboard(10)).toEqual(["season", "leaderboard", 10]);
  });
});

describe("useSeasonLeaderboardInfiniteQuery (page)", () => {
  beforeEach(() => mockGet.mockReset());

  it("starts at offset zero", async () => {
    mockGet.mockResolvedValue(page({ items: rows(1, 25), total: 100 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockGet).toHaveBeenCalledWith(25, 0);
  });

  // THE ARITHMETIC THE HOOK'S OWN COMMENT CALLS OUT: the next offset is the
  // number of rows ALREADY LOADED, not pageSize * pages. Those agree only while
  // every page comes back full.
  it("advances the offset by the rows already loaded, not by pageSize x pages", async () => {
    mockGet
      .mockResolvedValueOnce(page({ items: rows(1, 20), total: 100 })) // SHORT page
      .mockResolvedValueOnce(page({ items: rows(21, 25), total: 100, offset: 20 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await result.current.fetchNextPage();

    // 20, not 25: asking for offset 25 would silently skip rows 21-25.
    expect(mockGet).toHaveBeenLastCalledWith(25, 20);
  });

  it("stops paging when an empty page comes back", async () => {
    mockGet.mockResolvedValue(page({ items: [], total: 50 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // The `items.length === 0` guard. Without it, an offset past the end (total
    // still reporting 50) would keep advertising a next page forever and the
    // Load more button would loop on empty responses.
    expect(result.current.hasNextPage).toBe(false);
  });

  it("stops paging once the loaded rows reach the total", async () => {
    mockGet.mockResolvedValue(page({ items: rows(1, 25), total: 25 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.hasNextPage).toBe(false);
  });

  it("keeps paging while loaded rows are short of the total", async () => {
    mockGet.mockResolvedValue(page({ items: rows(1, 25), total: 60 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.hasNextPage).toBe(true);
  });

  // A TOTAL THAT SHRINKS between pages (someone deleted their account, or a
  // 0-SP row left the ladder). `getNextPageParam` reads the LAST page's total,
  // so paging must stop rather than chase a number that no longer exists — this
  // is exactly the disagreement that used to leave a dead Load more button when
  // the page compared against pages[0].total instead.
  it("stops paging when the total shrinks below the rows already loaded", async () => {
    mockGet
      .mockResolvedValueOnce(page({ items: rows(1, 25), total: 60 }))
      .mockResolvedValueOnce(page({ items: rows(26, 5), total: 28, offset: 25 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.hasNextPage).toBe(true);

    await result.current.fetchNextPage();

    await waitFor(() => {
      // 30 rows loaded, last page says 28 total -> no further page.
      expect(result.current.hasNextPage).toBe(false);
    });
  });

  it("keys separately from the widget query at the same page size", () => {
    // Same page size, DIFFERENT cached shape (pages[] vs a single response), so
    // the two must not share a cache entry.
    expect([...queryKeys.season.leaderboard(25), "infinite"]).not.toEqual(
      queryKeys.season.leaderboard(25),
    );
  });
});
