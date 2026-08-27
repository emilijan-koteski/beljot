import { axiosClient } from "@/shared/api/axiosClient";
import type {
  CurrentSeasonResponse,
  LeaderboardResponse,
  SeasonArchiveResponse,
  SeasonsListResponse,
} from "@/shared/types/apiTypes";

/**
 * The leaderboard's season selector: `"current"` for the active window (the
 * default everywhere), or a season id from {@link getSeasons} for a prior
 * season (Story 13.3). An unknown id is a 404 SEASON_NOT_FOUND, a malformed one
 * a 400 — the server never silently falls back to the current window.
 */
export type SeasonSelector = number | "current";

/**
 * GET /api/v1/seasons/current — the active season window plus the CALLER'S OWN
 * record (Story 13.1). Auth-gated and keyed off the JWT subject, so there is no
 * id to pass; Story 13.2's leaderboard is the endpoint that returns other
 * players.
 *
 * A player who has not played this season gets the zero state (0 SP, "iron"),
 * never a 404 — at 0 SP a player is Iron, which is a real tier.
 */
export function getCurrentSeason(): Promise<CurrentSeasonResponse> {
  return axiosClient.get("/seasons/current");
}

/**
 * GET /api/v1/leaderboard?season=current|<id> — one SP-ordered page of one
 * season plus the caller's own position (Story 13.2; the prior-season selector
 * landed with Story 13.3).
 *
 * `season` is sent EXPLICITLY even for the default: the URL shape stays stable
 * whichever window is being read, and "current" names the active one without
 * the client having to know its id.
 *
 * PULL-ONLY. Standings have no WebSocket push by design — the widget polls and
 * the page refetches on mount. Nothing in `useWsDispatch` invalidates this key.
 *
 * The server rejects `limit` outside 1..50 and a negative `offset` with a 400
 * rather than clamping, so callers pass values they mean.
 */
export function getSeasonLeaderboard(
  limit: number,
  offset: number,
  season: SeasonSelector = "current",
): Promise<LeaderboardResponse> {
  return axiosClient.get("/leaderboard", {
    params: { season, limit, offset },
  });
}

/**
 * GET /api/v1/seasons — every season window, newest-first (Story 13.3). Feeds
 * the leaderboard page's season picker; the names are machine tokens rendered
 * verbatim.
 */
export function getSeasons(): Promise<SeasonsListResponse> {
  return axiosClient.get("/seasons");
}

/**
 * GET /api/v1/users/:id/seasons — the subject's ENDED, PLAYED seasons,
 * newest-first (Story 13.3). Public to any authenticated viewer, like the
 * profile endpoints. An unknown id answers `{ items: [] }` with a 200 — the
 * profile query owns the not-found surface — and the archive section is
 * simply absent from the DOM when the list is empty.
 */
export function getSeasonArchive(userId: number): Promise<SeasonArchiveResponse> {
  return axiosClient.get(`/users/${userId}/seasons`);
}
