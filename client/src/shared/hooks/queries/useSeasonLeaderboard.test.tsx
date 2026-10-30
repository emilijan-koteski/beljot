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
  getSeasons: vi.fn(),
}));

const mockGet = vi.mocked(getSeasonLeaderboard);

function rows(from: number, n: number): LeaderboardRow[] {
  return Array.from({ length: n }, (_, i) => ({
    position: from + i,
    userId: from + i,
    username: `p${from + i}`,
    avatarUrl: null,
    sp: 10_000 - (from + i) * 10,
    tier: "gold",
    division: 2,
    gamesPlayed: 5,
    gamesWon: 2,
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
    expect(mockGet).toHaveBeenCalledWith(10, 0, "current");
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

    const entry = qc
      .getQueryCache()
      .find({ queryKey: queryKeys.season.leaderboard(10, "current") });
    const options = entry?.observers[0]?.options as
      { refetchInterval?: number; refetchOnWindowFocus?: boolean } | undefined;
    expect(options?.refetchInterval).toBeGreaterThan(0);
    expect(options?.refetchInterval).toBeGreaterThanOrEqual(30_000);
    expect(options?.refetchOnWindowFocus).toBe(true);
  });

  it("keys entries by page size and season so surfaces cannot collide", () => {
    expect(queryKeys.season.leaderboard(10, "current")).not.toEqual(
      queryKeys.season.leaderboard(25, "current"),
    );
    expect(queryKeys.season.leaderboard(25, "current")).not.toEqual(
      queryKeys.season.leaderboard(25, 5),
    );
    expect(queryKeys.season.leaderboard(10, "current")).toEqual([
      "season",
      "leaderboard",
      10,
      "current",
    ]);
    expect(queryKeys.season.leaderboard(10, "current").slice(0, 2)).toEqual([
      ...queryKeys.season.leaderboardAll(),
    ]);
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
    expect(mockGet).toHaveBeenCalledWith(25, 0, "current");
  });

  it("threads a picked season id into every page request", async () => {
    mockGet
      .mockResolvedValueOnce(page({ items: rows(1, 25), total: 30 }))
      .mockResolvedValueOnce(page({ items: rows(26, 5), total: 30, offset: 25 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25, 5), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockGet).toHaveBeenCalledWith(25, 0, 5);

    await result.current.fetchNextPage();
    expect(mockGet).toHaveBeenLastCalledWith(25, 25, 5);
  });

  it("advances the offset by the rows already loaded, not by pageSize x pages", async () => {
    mockGet
      .mockResolvedValueOnce(page({ items: rows(1, 20), total: 100 }))
      .mockResolvedValueOnce(page({ items: rows(21, 25), total: 100, offset: 20 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await result.current.fetchNextPage();

    expect(mockGet).toHaveBeenLastCalledWith(25, 20, "current");
  });

  it("stops paging when an empty page comes back", async () => {
    mockGet.mockResolvedValue(page({ items: [], total: 50 }));

    const { result } = renderHook(() => useSeasonLeaderboardInfiniteQuery(25), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

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
      expect(result.current.hasNextPage).toBe(false);
    });
  });

  it("keys separately from the widget query at the same page size", () => {
    expect([...queryKeys.season.leaderboard(25, "current"), "infinite"]).not.toEqual(
      queryKeys.season.leaderboard(25, "current"),
    );
  });
});
