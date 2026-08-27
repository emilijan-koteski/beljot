import { axiosClient } from "@/shared/api/axiosClient";
import type { SeasonRank } from "@/shared/types/apiTypes";
import type { CardDeck } from "@/shared/types/matchTypes";

export interface ProfileResponse {
  id: number;
  username: string;
  // When the username was last changed; absent/null if never. Drives the
  // client-side change-cooldown UX (see shared/lib/usernameChange).
  usernameChangedAt?: string | null;
  languagePreference: string;
  // Card deck (Story 12.4) — PRIVATE, self-only, deliberately absent from
  // PublicProfileResponse below. Mirrors the server DTO; the auth envelope is
  // what the renderer actually reads.
  cardDeckPreference: CardDeck;
  createdAt: string;
  totalGamesPlayed: number;
  wins: number;
  losses: number;
  abandoned: number;
  // XP & level (Story 9.5). level is derived server-side from totalXp;
  // xpIntoLevel / xpForNextLevel drive the profile XP-bar fill
  // (fill = xpIntoLevel / xpForNextLevel). Server-authoritative, never recomputed.
  totalXp: number;
  level: number;
  xpIntoLevel: number;
  xpForNextLevel: number;
  // Honor (Story 9.7). Unlike walletBalance / totalXp these are PUBLIC-SAFE by
  // design — honor exists so other players can judge reliability, and Epic 11's
  // public profile can carry them verbatim.
  //
  // honorScore is server-recomputed on every response (never the lagging
  // honor_score column). honorTier / honorTrendDirection are stable machine
  // tokens, mapped to i18n labels client-side. The Completed/Abandoned totals
  // are raw undecayed lifetime counts. isNewPlayer hides the score and tier
  // behind a "New Player" chip while the counts still render.
  honorScore: number;
  honorTier: string;
  honorCompletedTotal: number;
  honorAbandonedTotal: number;
  isNewPlayer: boolean;
  honorTrendDelta: number;
  honorTrendDirection: string;
  // Seasonal rank (Story 13.3): the subject's standing in the ACTIVE season,
  // or null when they have not played in it — the chip hides on null. The key
  // is ALWAYS present on the wire (no omitempty server-side), so null means
  // "no standing", never "an older server". Public-safe: the public shape
  // below carries it verbatim.
  seasonRank: SeasonRank | null;
}

/**
 * The PUBLIC projection of a player's profile (Story 11.3). The server returns
 * this shape from GET /users/:id/profile when :id is NOT the authenticated
 * viewer. It is a strict subset of {@link ProfileResponse}: identity,
 * member-since, progression (level + XP) and the full honor section, but NONE of
 * the private figures — no walletBalance, loginStreakDays, languagePreference or
 * usernameChangedAt. Same URL as getProfile; the narrower type is the point.
 */
export interface PublicProfileResponse {
  id: number;
  username: string;
  createdAt: string;
  totalGamesPlayed: number;
  wins: number;
  losses: number;
  abandoned: number;
  totalXp: number;
  level: number;
  xpIntoLevel: number;
  xpForNextLevel: number;
  honorScore: number;
  honorTier: string;
  honorCompletedTotal: number;
  honorAbandonedTotal: number;
  isNewPlayer: boolean;
  honorTrendDelta: number;
  honorTrendDirection: string;
  // Seasonal rank (Story 13.3) — public per the epic AC. Same null semantics
  // as the self shape.
  seasonRank: SeasonRank | null;
}

/**
 * A PARTIAL preferences update — both fields are optional, and the server
 * validates and writes each independently (Story 12.4). Sending only the deck
 * leaves the language untouched and vice versa; a body with neither is a 400.
 */
export interface UpdatePreferencesRequest {
  languagePreference?: string;
  cardDeckPreference?: CardDeck;
}

/**
 * The server echoes back ONLY the fields it wrote, so both keys are optional
 * here too. Merging a `cardDeckPreference: undefined` into the cached user is a
 * no-op; merging an echoed `""` would have clobbered the untouched preference.
 */
export interface UpdatePreferencesResponse {
  languagePreference?: string;
  cardDeckPreference?: CardDeck;
}

export function getProfile(userId: number): Promise<ProfileResponse> {
  return axiosClient.get(`/users/${userId}/profile`);
}

/**
 * Fetch another player's public profile. Hits the same URL as getProfile — the
 * server decides the shape from whether :id is the authenticated viewer — and
 * only attaches the narrower {@link PublicProfileResponse} type. The public page
 * never requests its own viewer's id here.
 */
export function getPublicProfile(userId: number): Promise<PublicProfileResponse> {
  return axiosClient.get(`/users/${userId}/profile`);
}

export function updatePreferences(
  userId: number,
  prefs: UpdatePreferencesRequest,
): Promise<UpdatePreferencesResponse> {
  return axiosClient.patch(`/users/${userId}/preferences`, prefs);
}

export interface UpdateUsernameRequest {
  username: string;
}

export interface UpdateUsernameResponse {
  username: string;
  usernameChangedAt: string;
}

export function updateUsername(
  userId: number,
  req: UpdateUsernameRequest,
): Promise<UpdateUsernameResponse> {
  return axiosClient.patch(`/users/${userId}/username`, req);
}
