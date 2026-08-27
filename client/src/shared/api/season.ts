import { axiosClient } from "@/shared/api/axiosClient";
import type { CurrentSeasonResponse, LeaderboardResponse } from "@/shared/types/apiTypes";

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
 * GET /api/v1/leaderboard?season=current — one SP-ordered page of the active
 * season plus the caller's own position (Story 13.2).
 *
 * `season=current` is sent EXPLICITLY even though the server also accepts its
 * absence: it is the only value the allowlist takes today, and spelling it out
 * is what makes the URL stable when Story 13.3 adds real prior-season selectors.
 *
 * PULL-ONLY. Standings have no WebSocket push by design — the widget polls and
 * the page refetches on mount. Nothing in `useWsDispatch` invalidates this key.
 *
 * The server rejects `limit` outside 1..50 and a negative `offset` with a 400
 * rather than clamping, so callers pass values they mean.
 */
export function getSeasonLeaderboard(limit: number, offset: number): Promise<LeaderboardResponse> {
  return axiosClient.get("/leaderboard", {
    params: { season: "current", limit, offset },
  });
}
