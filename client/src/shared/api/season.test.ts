import { beforeEach, describe, expect, it, vi } from "vitest";

import { axiosClient } from "@/shared/api/axiosClient";
import {
  getCurrentSeason,
  getSeasonArchive,
  getSeasonLeaderboard,
  getSeasons,
} from "@/shared/api/season";

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

  it("defaults to season=current when no selector is passed", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(10, 0);

    // "current" and a bare positive integer are the ONLY values the server
    // allowlist accepts — anything else ("2026Q3", "previous") is a 400 by
    // design, so this literal is load-bearing rather than cosmetic.
    expect(get.mock.calls[0]![1]).toMatchObject({ params: { season: "current" } });
  });

  // Story 13.3: the prior-season selector is the season's numeric id, sent
  // as-is — parseLeaderboardQuery accepts positive integers, nothing else.
  it("sends a picked season's id verbatim", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(25, 0, 5);

    expect(get.mock.calls[0]![0]).toBe("/leaderboard");
    expect(get.mock.calls[0]![1]).toEqual({ params: { season: 5, limit: 25, offset: 0 } });
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

  it("getSeasons issues GET /seasons with no params", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasons();

    // The picker's feed (Story 13.3). Static path, no selector — the server
    // returns every window newest-first.
    expect(get).toHaveBeenCalledWith("/seasons");
  });

  it("getSeasonArchive issues GET /users/:id/seasons for the subject", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonArchive(42);

    // The SUBJECT's id in the path — main.go registers /users/:id/seasons on
    // the season handler (Story 13.3).
    expect(get).toHaveBeenCalledWith("/users/42/seasons");
  });
});
