import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";

/** Every audio preference on the account: two on/off switches and each channel's 0-100 volume. */
interface AudioPreferences {
  soundEnabled: boolean;
  musicEnabled: boolean;
  soundVolume: number;
  musicVolume: number;
}
type AudioPreferenceField = keyof AudioPreferences;
export type AudioSwitchField = "soundEnabled" | "musicEnabled";
export type AudioVolumeField = "soundVolume" | "musicVolume";
type AudioPreferencesPatch = Partial<AudioPreferences>;

const FIELDS: readonly AudioPreferenceField[] = [
  "soundEnabled",
  "musicEnabled",
  "soundVolume",
  "musicVolume",
];

export const DEFAULT_AUDIO_VOLUME = 70;

/** The effective value of an audio switch. Missing means ON, matching the server default. */
export function resolveAudioEnabled(value: boolean | null | undefined): boolean {
  return value ?? true;
}

/**
 * The effective value of an audio volume: a whole number clamped to 0-100.
 * Missing or non-finite means the server default of 70, never silence.
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

const latestRequestByField: Record<AudioPreferenceField, number> = {
  soundEnabled: 0,
  musicEnabled: 0,
  soundVolume: 0,
  musicVolume: 0,
};
let sequence = 0;

/**
 * Optimistically persists the changed audio preferences via the auth store.
 * A failed request reverts each field unless a later write of that field superseded it.
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
  for (const field of fields) latestRequestByField[field] = request;
  useAuthStore.getState().setUser({ ...user, ...body });

  try {
    await updatePreferences(user.id, body);
  } catch {
    const current = useAuthStore.getState().user;
    if (current?.id !== user.id) return;
    const revert: AudioPreferencesPatch = {};
    for (const field of fields) {
      if (latestRequestByField[field] === request) assign(revert, field, previous[field]);
    }
    if (Object.keys(revert).length === 0) return;
    useAuthStore.getState().setUser({ ...current, ...revert });
  }
}

/** Test seam: reset the in-flight sequences between cases. */
export function resetAudioPreferenceRequestSequence(): void {
  sequence = 0;
  for (const field of FIELDS) latestRequestByField[field] = 0;
}
