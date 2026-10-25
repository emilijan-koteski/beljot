import { axiosClient } from "@/shared/api/axiosClient";
import type { MatchMode, Variant } from "@/shared/types/matchTypes";

export type MatchOutcome = "win" | "loss" | "abandoned";

/** Why the match ended — "surrender" and "abandonment" rows carry a muted
 *  "ended early" marker in history, with distinct wording per reason. */
export type MatchEndReason = "natural" | "surrender" | "abandonment";

/** A seat in a match; bot seats have userId 0, so detect them with `isBot === true`. */
export interface MatchPlayer {
  seat: number;
  userId: number;
  username: string;
  avatarUrl: string | null;
  isBot: boolean;
}

export interface MatchHandView {
  handNumber: number;
  teamACardPoints: number;
  teamBCardPoints: number;
  teamADeclPoints: number;
  teamBDeclPoints: number;
  lastTrickTeam: number;
  lastTrickBonus: number;
  capot: boolean;
  capotTeam?: number;
  capotBonus: number;
  failedContract: boolean;
  contractingTeam: number;
  teamAHandTotal: number;
  teamBHandTotal: number;
}

/**
 * A viewer-relative match history row. Optional fields may be absent from older servers,
 * so compare them explicitly rather than by truthiness.
 */
export interface MatchListItem {
  id: number;
  variant: Variant;
  matchMode: MatchMode;
  startedAt: string;
  completedAt: string;
  status: string;
  winnerTeam: number;
  teamAScore: number;
  teamBScore: number;
  hasBots: boolean;
  abandonedBy?: number;
  viewerSeat: number;
  outcome: MatchOutcome;
  endReason?: MatchEndReason;
  declarationsEnabled?: boolean;
  stopAtTarget?: boolean;
  declarationsTransfer?: boolean;
  clockwise?: boolean;
  players: MatchPlayer[];
  hands: MatchHandView[];
}

export interface MatchesListResponse {
  items: MatchListItem[];
  total: number;
  limit: number;
  offset: number;
}

/** Viewer-relative outcome filter; "all" leaves the result set unfiltered. */
export type MatchFilter = "all" | "win" | "loss" | "abandoned";

/** Completed-at ordering: "new" (newest first, default) or "old". */
export type MatchSort = "new" | "old";

interface MatchListParams {
  outcome?: MatchFilter;
  sort?: MatchSort;
}

export function getUserMatches(
  userId: number,
  limit: number,
  offset: number,
  { outcome = "all", sort = "new" }: MatchListParams = {},
): Promise<MatchesListResponse> {
  return axiosClient.get(`/users/${userId}/matches`, {
    params: { limit, offset, outcome: outcome === "all" ? undefined : outcome, sort },
  });
}

/** The most recent match in a room that the caller played; 404s otherwise. */
export function getRoomLastMatch(roomId: number): Promise<MatchListItem> {
  return axiosClient.get(`/rooms/${roomId}/last-match`);
}
