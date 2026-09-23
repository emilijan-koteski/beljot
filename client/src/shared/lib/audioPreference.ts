import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * Every audio preference on the account: the two on/off switches (migration
 * 000025) and the 0-100 level of each channel (migration 000026).
 */
export interface AudioPreferences {
  soundEnabled: boolean;
  musicEnabled: boolean;
  soundVolume: number;
  musicVolume: number;
}
export type AudioPreferenceField = keyof AudioPreferences;
export type AudioSwitchField = "soundEnabled" | "musicEnabled";
export type AudioVolumeField = "soundVolume" | "musicVolume";
export type AudioPreferencesPatch = Partial<AudioPreferences>;

const FIELDS: readonly AudioPreferenceField[] = [
  "soundEnabled",
  "musicEnabled",
  "soundVolume",
  "musicVolume",
];

/**
 * The level both channels play at until the player picks another — and the
 * level the game played at before volumes existed, so an account that never
 * touched a slider hears no change. Matches the server column default.
 */
export const DEFAULT_AUDIO_VOLUME = 70;

/**
 * The effective value of an audio switch. Missing means ON: both switches
 * default on server-side, so an absent field (a cast HTTP response, a user
 * object built before the field existed) must never read as "muted".
 */
export function resolveAudioEnabled(value: boolean | null | undefined): boolean {
  return value ?? true;
}

/**
 * The effective value of an audio volume: a whole number clamped to 0-100.
 * Missing (or not a finite number) means the default of 70, never silence —
 * the server defaults both columns to 70, so an absent field is an old or
 * partial user object, not a player who turned the channel down.
 */
export function resolveAudioVolume(value: number | null | undefined): number {
  if (typeof value !== "number" || !Number.isFinite(value)) return DEFAULT_AUDIO_VOLUME;
  return Math.min(100, Math.max(0, Math.round(value)));
}

function resolveField<K extends AudioPreferenceField>(
  field: K,
  value: AudioPreferences[K] | null | undefined,
): AudioPreferences[K] {
  if (field === "soundVolume" || field === "musicVolume") {
    return resolveAudioVolume(value as number | null | undefined) as AudioPreferences[K];
  }
  return resolveAudioEnabled(value as boolean | null | undefined) as AudioPreferences[K];
}

function assign<K extends AudioPreferenceField>(
  target: AudioPreferencesPatch,
  field: K,
  value: AudioPreferences[K] | undefined,
): void {
  target[field] = value;
}

/**
 * Monotonic tokens for in-flight audio writes, one per field.
 *
 * Per FIELD rather than one shared counter because the entry points write
 * overlapping subsets: the Settings rows each write one switch or one volume,
 * the HUD mute writes both switches. A late failure of a music-only toggle
 * must still revert music even though a sound toggle (or a volume change) was
 * made after it — only a later write of the SAME field supersedes it.
 * Module-level for the same reason as the deck helper: the in-match dialog,
 * the HUD button and the profile panel are different components writing the
 * same server fields.
 */
const latestRequest: Record<AudioPreferenceField, number> = {
  soundEnabled: 0,
  musicEnabled: 0,
  soundVolume: 0,
  musicVolume: 0,
};
let sequence = 0;

/**
 * Persist any subset of the audio preferences, optimistically.
 *
 * The auth store is what the audio engine and every control read, so writing
 * it before the request is what makes a switch or a volume apply instantly,
 * mid-hand. A volume is resolved (clamped to a whole 0-100) before it is
 * compared or sent. Fields whose value would not change are dropped, so the
 * PATCH body carries only the preferences that actually move (and nothing at
 * all is sent when none do).
 *
 * A failed request reverts each of its fields to the value it replaced —
 * unless a later write of that field has superseded it, in which case that
 * write owns the outcome. The failure is silent, like the deck and language
 * pickers: the visible state is already consistent with the store.
 */
export async function persistAudioPreferences(patch: AudioPreferencesPatch): Promise<void> {
  const user = useAuthStore.getState().user;
  if (!user) return;

  const body: AudioPreferencesPatch = {};
  const previous: AudioPreferencesPatch = {};
  for (const field of FIELDS) {
    const requested = patch[field];
    if (requested === undefined) continue;
    const next = resolveField(field, requested);
    const current = resolveField(field, user[field]);
    if (next === current) continue;
    assign(body, field, next);
    assign(previous, field, current);
  }
  const fields = FIELDS.filter((field) => body[field] !== undefined);
  if (fields.length === 0) return;

  const request = ++sequence;
  for (const field of fields) latestRequest[field] = request;
  useAuthStore.getState().setUser({ ...user, ...body });

  try {
    await updatePreferences(user.id, body);
  } catch {
    const current = useAuthStore.getState().user;
    if (current?.id !== user.id) return;
    const revert: AudioPreferencesPatch = {};
    for (const field of fields) {
      if (latestRequest[field] === request) assign(revert, field, previous[field]);
    }
    if (Object.keys(revert).length === 0) return;
    useAuthStore.getState().setUser({ ...current, ...revert });
  }
}

/** Test seam: reset the in-flight sequences between cases. */
export function resetAudioPreferenceRequestSequence(): void {
  sequence = 0;
  for (const field of FIELDS) latestRequest[field] = 0;
}
