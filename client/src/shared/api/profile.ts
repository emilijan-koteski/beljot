import { axiosClient } from "@/shared/api/axiosClient";

export interface ProfileResponse {
  id: number;
  username: string;
  // When the username was last changed; absent/null if never. Drives the
  // client-side change-cooldown UX (see shared/lib/usernameChange).
  usernameChangedAt?: string | null;
  languagePreference: string;
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
}

export interface UpdatePreferencesRequest {
  languagePreference: string;
}

export function getProfile(userId: number): Promise<ProfileResponse> {
  return axiosClient.get(`/users/${userId}/profile`);
}

export function updatePreferences(
  userId: number,
  prefs: UpdatePreferencesRequest,
): Promise<{ languagePreference: string }> {
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
