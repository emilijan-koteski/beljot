import { axiosClient } from "@/shared/api/axiosClient";
import type {
  CurrentSeasonResponse,
  LeaderboardResponse,
  SeasonArchiveResponse,
  SeasonsListResponse,
} from "@/shared/types/apiTypes";

/** The leaderboard's season selector: `"current"` for the active window, or a prior season's id. */
export type SeasonSelector = number | "current";

/** GET /seasons/current: the active season window plus the caller's own record. */
export function getCurrentSeason(): Promise<CurrentSeasonResponse> {
  return axiosClient.get("/seasons/current");
}

/** GET /leaderboard: one SP-ordered page of a season plus the caller's own position. */
export function getSeasonLeaderboard(
  limit: number,
  offset: number,
  season: SeasonSelector = "current",
): Promise<LeaderboardResponse> {
  return axiosClient.get("/leaderboard", {
    params: { season, limit, offset },
  });
}

/** GET /seasons: every season window, newest first. */
export function getSeasons(): Promise<SeasonsListResponse> {
  return axiosClient.get("/seasons");
}

/** GET /users/:id/seasons: the subject's ended, played seasons, newest first. */
export function getSeasonArchive(userId: number): Promise<SeasonArchiveResponse> {
  return axiosClient.get(`/users/${userId}/seasons`);
}
