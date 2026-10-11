import { useCallback, useEffect, useRef, useState } from "react";

import {
  type AudioVolumeField,
  persistAudioPreferences,
  resolveAudioVolume,
} from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { playSfx, setMusicVolume } from "./audioEngine";

export const COMMIT_COALESCE_MS = 250;

interface AudioVolumeControl {
  value: number;
  onValueChange: (value: number) => void;
  onValueCommitted: (value: number) => void;
}

/**
 * Drives one audio channel's volume slider, saving one PATCH per release and coalescing held-key commits.
 * Commits with no thumb movement since the last one are ignored.
 */
export function useAudioVolume(field: AudioVolumeField): AudioVolumeControl {
  const stored = resolveAudioVolume(useAuthStore((s) => s.user?.[field]));
  const [draft, setDraft] = useState<number | null>(null);
  const movedSinceLastCommit = useRef(false);
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
      movedSinceLastCommit.current = true;
      setDraft(volume);
      if (field === "musicVolume") setMusicVolume(volume);
    },
    [field],
  );

  const onValueCommitted = useCallback(
    (next: number) => {
      if (!movedSinceLastCommit.current) return;
      movedSinceLastCommit.current = false;
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
      if (!movedSinceLastCommit.current || field !== "musicVolume") return;
      setMusicVolume(resolveAudioVolume(useAuthStore.getState().user?.musicVolume));
    },
    [field, save],
  );

  return { value: draft ?? stored, onValueChange, onValueCommitted };
}
