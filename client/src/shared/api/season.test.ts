import { beforeEach, describe, expect, it, vi } from "vitest";

import { axiosClient } from "@/shared/api/axiosClient";
import {
  getCurrentSeason,
  getSeasonArchive,
  getSeasonLeaderboard,
  getSeasons,
} from "@/shared/api/season";

describe("season API request contract", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("getSeasonLeaderboard issues GET /leaderboard with season=current and the page window", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(25, 50);

    expect(get).toHaveBeenCalledTimes(1);
    const [url, config] = get.mock.calls[0]!;
    expect(url).toBe("/leaderboard");
    expect(config).toEqual({ params: { season: "current", limit: 25, offset: 50 } });
  });

  it("defaults to season=current when no selector is passed", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(10, 0);

    expect(get.mock.calls[0]![1]).toMatchObject({ params: { season: "current" } });
  });

  it("sends a picked season's id verbatim", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonLeaderboard(25, 0, 5);

    expect(get.mock.calls[0]![0]).toBe("/leaderboard");
    expect(get.mock.calls[0]![1]).toEqual({ params: { season: 5, limit: 25, offset: 0 } });
  });

  it("passes limit and offset through unchanged, without clamping", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

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

    expect(get).toHaveBeenCalledWith("/seasons");
  });

  it("getSeasonArchive issues GET /users/:id/seasons for the subject", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await getSeasonArchive(42);

    expect(get).toHaveBeenCalledWith("/users/42/seasons");
  });
});
