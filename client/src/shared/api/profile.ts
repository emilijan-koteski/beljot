import { axiosClient } from "@/shared/api/axiosClient";
import type { SeasonRank } from "@/shared/types/apiTypes";
import type { CardDeck } from "@/shared/types/matchTypes";

export interface ProfileResponse {
  id: number;
  username: string;
  avatarUrl: string | null;
  avatarLargeUrl: string | null;
  usernameChangedAt?: string | null;
  languagePreference: string;
  cardDeckPreference: CardDeck;
  soundEnabled: boolean;
  musicEnabled: boolean;
  soundVolume: number;
  musicVolume: number;
  allowNonFriendInvites: boolean;
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
  seasonRank: SeasonRank | null;
}

/**
 * The public projection of a player's profile, returned when :id is not the viewer.
 * A strict subset of {@link ProfileResponse} with none of the private figures.
 */
export interface PublicProfileResponse {
  id: number;
  username: string;
  avatarUrl: string | null;
  avatarLargeUrl: string | null;
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
  seasonRank: SeasonRank | null;
}

/** A partial preferences update; the server validates and writes each field independently. */
interface UpdatePreferencesRequest {
  languagePreference?: string;
  cardDeckPreference?: CardDeck;
  soundEnabled?: boolean;
  musicEnabled?: boolean;
  soundVolume?: number;
  musicVolume?: number;
  allowNonFriendInvites?: boolean;
}

/** Only the fields the server wrote, so untouched preferences are never clobbered on merge. */
interface UpdatePreferencesResponse {
  languagePreference?: string;
  cardDeckPreference?: CardDeck;
  soundEnabled?: boolean;
  musicEnabled?: boolean;
  soundVolume?: number;
  musicVolume?: number;
  allowNonFriendInvites?: boolean;
}

export function getProfile(userId: number): Promise<ProfileResponse> {
  return axiosClient.get(`/users/${userId}/profile`);
}

/** Fetch another player's public profile from the same URL as getProfile, typed to the narrower shape. */
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

interface UpdateUsernameResponse {
  username: string;
  usernameChangedAt: string;
}

export function updateUsername(
  userId: number,
  req: UpdateUsernameRequest,
): Promise<UpdateUsernameResponse> {
  return axiosClient.patch(`/users/${userId}/username`, req);
}

/** The two public URLs of a freshly uploaded avatar (128 and 256 px WebP). */
interface UploadAvatarResponse {
  avatarUrl: string | null;
  avatarLargeUrl: string | null;
}

const AVATAR_UPLOAD_TIMEOUT_MS = 60_000;

/**
 * Upload (or replace) the caller's avatar as multipart field `avatar`, unprocessed.
 * `onProgress` receives the sent fraction, 0 to 1.
 */
export function uploadAvatar(
  userId: number,
  file: File,
  onProgress?: (fraction: number) => void,
): Promise<UploadAvatarResponse> {
  const form = new FormData();
  form.append("avatar", file);
  return axiosClient.put(`/users/${userId}/avatar`, form, {
    headers: { "Content-Type": "multipart/form-data" },
    timeout: AVATAR_UPLOAD_TIMEOUT_MS,
    onUploadProgress: (event) => {
      if (onProgress && event.total) onProgress(Math.min(1, event.loaded / event.total));
    },
  });
}

/** Remove the caller's avatar. 204 whether or not one was set. */
export function removeAvatar(userId: number): Promise<void> {
  return axiosClient.delete(`/users/${userId}/avatar`);
}
