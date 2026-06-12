import { axiosClient } from "@/shared/api/axiosClient";

export type MatchOutcome = "win" | "loss" | "abandoned";

export interface MatchPlayer {
  seat: number;
  // Bot seats arrive as {userId: 0, username: "", isBot: true} — check with
  // `isBot === true` (explicit comparison, never truthiness on userId) and
  // render the localized seat-derived bot name.
  userId: number;
  username: string;
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

export interface MatchListItem {
  id: number;
  variant: string;
  matchMode: string;
  startedAt: string;
  completedAt: string;
  status: string;
  winnerTeam: number;
  teamAScore: number;
  teamBScore: number;
  /** True when at least one seat was a bot (Story 10.3) — history rows show a marker. */
  hasBots: boolean;
  abandonedBy?: number;
  viewerSeat: number;
  outcome: MatchOutcome;
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

export interface MatchListParams {
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
    // "all" is the server default, so it is omitted to keep the URL clean.
    params: { limit, offset, outcome: outcome === "all" ? undefined : outcome, sort },
  });
}
