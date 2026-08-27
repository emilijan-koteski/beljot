import { beforeEach, describe, expect, it, vi } from "vitest";

import { axiosClient } from "@/shared/api/axiosClient";
import { getCurrentSeason, getSeasonLeaderboard } from "@/shared/api/season";

/**
 * THE REQUEST HALF OF THE CONTRACT.
 *
 * Every component test in this feature `vi.mock`s the whole `@/shared/api/season`
 * module and asserts only the ARGUMENTS the caller passed — never the URL or the
 * query params that actually go on the wire. The Go tests, on the other side,
 * build their own query strings. So the two suites verify a handshake neither of
 * them performs: renaming the path to `/leaderboards`, or sending
 * `season=2026Q3`, would ship a permanently 404-ing or 400-ing leaderboard with
 * both suites green.
 *
 * These tests spy on `axiosClient.get` — the single seam every request in this
 * module goes through — and pin the path and params verbatim against
 * server/internal/season/handler.go's route and `parseLeaderboardQuery` allowlist.
 */
describe("season API request contract", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("getSeasonLeaderboard issues GET /leaderboard with season=current and the page window", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(25, 50);

    expect(get).toHaveBeenCalledTimes(1);
    const [url, config] = get.mock.calls[0]!;
    // The path the api group registers (main.go). No /seasons/ prefix: the epic
    // AC specifies /leaderboard?season=current verbatim.
    expect(url).toBe("/leaderboard");
    expect(config).toEqual({ params: { season: "current", limit: 25, offset: 50 } });
  });

  it("always sends season=current, the only value the server allowlist accepts", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(10, 0);

    // Any other value — including a plausible "2026Q3" — is a 400 by design, so
    // this literal is load-bearing rather than cosmetic.
    expect(get.mock.calls[0]![1]).toMatchObject({ params: { season: "current" } });
  });

  it("passes limit and offset through unchanged, without clamping", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    // The server rejects out-of-range values with a 400 rather than clamping, so
    // the client must not quietly "fix" them either — a silent clamp here would
    // hide the very bug the 400 exists to surface.
    await getSeasonLeaderboard(50, 10_000);

    expect(get.mock.calls[0]![1]).toMatchObject({ params: { limit: 50, offset: 10_000 } });
  });

  it("getCurrentSeason issues GET /seasons/current with no params", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getCurrentSeason();

    expect(get).toHaveBeenCalledWith("/seasons/current");
  });
});
