import { axiosClient } from "@/shared/api/axiosClient";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

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
