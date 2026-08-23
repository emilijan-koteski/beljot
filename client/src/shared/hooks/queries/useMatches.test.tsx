import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import { queryKeys } from "@/shared/api/queryKeys";

const mockGetRoomLastMatch = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getRoomLastMatch: (roomId: number) => mockGetRoomLastMatch(roomId),
  getUserMatches: vi.fn(),
}));

import { useRoomLastMatchQuery } from "./useMatches";

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

/**
 * Mirrors the app-wide client (the 30s staleTime the hook must override, and a
 * `retry` default the hook must override too), minus the backoff — these tests
 * assert the retry PREDICATE, and real exponential delays would add seconds to
 * the suite for nothing.
 */
function makeAppLikeClient() {
  return new QueryClient({
    defaultOptions: { queries: { staleTime: 30_000, retry: 1, gcTime: 0, retryDelay: 0 } },
  });
}

describe("useRoomLastMatchQuery", () => {
  beforeEach(() => {
    mockGetRoomLastMatch.mockReset();
  });

  it("does not fetch while disabled", () => {
    const client = makeAppLikeClient();
    renderHook(() => useRoomLastMatchQuery(5, false), { wrapper: wrapper(client) });
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
  });

  it("does not fetch without a roomId", () => {
    const client = makeAppLikeClient();
    renderHook(() => useRoomLastMatchQuery(undefined, true), { wrapper: wrapper(client) });
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
  });

  // The client-wide staleTime is 30s and the key is per-ROOM, so without the
  // override a cached previous match would be served without a refetch.
  it("refetches on mount despite a warm cache (staleTime 0)", async () => {
    const client = makeAppLikeClient();
    client.setQueryData(queryKeys.matches.lastByRoom(5), { id: 1 });
    mockGetRoomLastMatch.mockResolvedValue({ id: 2 });

    renderHook(() => useRoomLastMatchQuery(5, true), { wrapper: wrapper(client) });

    await waitFor(() => expect(mockGetRoomLastMatch).toHaveBeenCalledTimes(1));
  });

  it.each([404, 400, 401, 403])(
    "treats a %i as a settled answer and calls the endpoint exactly once",
    async (statusCode) => {
      const client = makeAppLikeClient();
      mockGetRoomLastMatch.mockRejectedValue(new FetchError(statusCode, "CODE", "nope"));

      const { result } = renderHook(() => useRoomLastMatchQuery(5, true), {
        wrapper: wrapper(client),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
      expect(mockGetRoomLastMatch).toHaveBeenCalledTimes(1);
    },
  );

  it("retries a 500 twice before settling (three attempts total)", async () => {
    const client = makeAppLikeClient();
    mockGetRoomLastMatch.mockRejectedValue(new FetchError(500, "INTERNAL_ERROR", "boom"));

    const { result } = renderHook(() => useRoomLastMatchQuery(5, true), {
      wrapper: wrapper(client),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(mockGetRoomLastMatch).toHaveBeenCalledTimes(3);
  });

  // FetchError.status is 0 for a dropped connection / timeout — the one case
  // the retry actually exists for.
  it("retries a network failure", async () => {
    const client = makeAppLikeClient();
    mockGetRoomLastMatch.mockRejectedValue(new FetchError(0, "NETWORK_ERROR", "offline"));

    const { result } = renderHook(() => useRoomLastMatchQuery(5, true), {
      wrapper: wrapper(client),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(mockGetRoomLastMatch).toHaveBeenCalledTimes(3);
  });
});
