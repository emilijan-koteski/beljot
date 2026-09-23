import { useCallback, useEffect, useRef, useState } from "react";

import {
  type AudioVolumeField,
  persistAudioPreferences,
  resolveAudioVolume,
} from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { playSfx, setMusicVolume } from "./audioEngine";

/**
 * How long commits must go quiet before a held-back one is saved. Base UI
 * commits on every keydown, so a held arrow key auto-repeats commits far
 * faster than this.
 */
export const COMMIT_COALESCE_MS = 250;

export interface AudioVolumeControl {
  /** What the slider shows: the drag in progress, else the stored volume. */
  value: number;
  /** Live, on every movement of the thumb. */
  onValueChange: (value: number) => void;
  /** Once per release (and once per keyboard step, auto-repeat included). */
  onValueCommitted: (value: number) => void;
}

/**
 * One audio channel's volume slider, shared by the in-match Settings dialog and
 * the profile Audio panel.
 *
 * While the thumb moves, the value lives in a LOCAL draft and the store keeps
 * the pre-drag volume. That is what lets `persistAudioPreferences` compute the
 * right rollback target and send exactly one PATCH on release, instead of one
 * per pixel. Music drafts go straight to the engine so the drag is heard live;
 * the store-driven level in `useMatchAudio` then settles on the committed (or,
 * after a failed PATCH, reverted) value.
 *
 * A saved commit persists the value, and on the sound-effects slider plays one
 * card sound at the new level as a preview. The engine drops that preview
 * outside a match, so the profile panel stays silent — the game makes no sound
 * off the match page.
 *
 * Commits are coalesced: an isolated one is saved at once, but further commits
 * arriving within `COMMIT_COALESCE_MS` of the previous one (a held arrow key)
 * are held — the draft keeps showing the latest, music keeps following it live
 * — and saved once, when commits go quiet. So a held key sends one leading and
 * one trailing PATCH and preview, not one per auto-repeat. A value still held
 * at unmount is saved then, without a preview.
 *
 * A commit with no `onValueChange` since the previous one is ignored: Base UI
 * also commits when nothing moved (a thumb click without dragging, End at the
 * maximum), reporting its last changed value — which may be a level the store
 * has since reverted away from.
 */
export function useAudioVolume(field: AudioVolumeField): AudioVolumeControl {
  const stored = resolveAudioVolume(useAuthStore((s) => s.user?.[field]));
  const [draft, setDraft] = useState<number | null>(null);
  const changed = useRef(false);
  const pending = useRef<number | null>(null);
  const quietTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const save = useCallback(
    (volume: number, preview: boolean) => {
      void persistAudioPreferences({ [field]: volume });
      if (field === "musicVolume") setMusicVolume(volume);
      else if (preview) playSfx("cardPlay");
    },
    [field],
  );

  const onValueChange = useCallback(
    (next: number) => {
      const volume = resolveAudioVolume(next);
      changed.current = true;
      setDraft(volume);
      if (field === "musicVolume") setMusicVolume(volume);
    },
    [field],
  );

  const onValueCommitted = useCallback(
    (next: number) => {
      if (!changed.current) return;
      changed.current = false;
      const volume = resolveAudioVolume(next);
      const inBurst = quietTimer.current !== null;
      if (quietTimer.current !== null) clearTimeout(quietTimer.current);
      quietTimer.current = setTimeout(() => {
        quietTimer.current = null;
        const held = pending.current;
        if (held === null) return;
        pending.current = null;
        save(held, true);
        setDraft(null);
      }, COMMIT_COALESCE_MS);
      if (inBurst) {
        pending.current = volume;
        return;
      }
      save(volume, true);
      setDraft(null);
    },
    [save],
  );

  // Unmounted with a commit still held (the dialog closed mid key-repeat): save
  // it now. Unmounted mid-drag with nothing committed: hand the music back to
  // the stored level rather than leaving it on an unsaved draft.
  useEffect(
    () => () => {
      if (quietTimer.current !== null) {
        clearTimeout(quietTimer.current);
        quietTimer.current = null;
      }
      const held = pending.current;
      pending.current = null;
      if (held !== null) {
        save(held, false);
        return;
      }
      if (!changed.current || field !== "musicVolume") return;
      setMusicVolume(resolveAudioVolume(useAuthStore.getState().user?.musicVolume));
    },
    [field, save],
  );

  return { value: draft ?? stored, onValueChange, onValueCommitted };
}
