import { useEffect } from "react";

import { resolveAudioEnabled, resolveAudioVolume } from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { openAudioSession, preloadSfx, setMusicVolume, startMusic, stopMusic } from "./audioEngine";

/**
 * Match-page audio lifecycle. Mounted once, by `MatchPage`, which makes the
 * match page the only place the game makes any sound:
 *
 * - opens an audio session for as long as the page is mounted (sound effects
 *   are dropped outside one, so a WS card event that arrives while the player
 *   is on another page stays silent) and installs the gesture unlock;
 * - preloads the sound effects while sound is on — never with it off, since
 *   the preload is what would create the audio context — and again the moment
 *   it is switched on;
 * - runs the music while the page is mounted AND the player has music on —
 *   toggling the preference mid-match fades it out or starts it immediately,
 *   and unmounting (lobby, room, back) stops it;
 * - keeps the engine's music level on the STORED music volume, so a committed
 *   slider value — or its silent revert after a failed PATCH — is what the
 *   music settles on. (A drag in progress drives the engine directly, through
 *   `useAudioVolume`, without touching the store.) A stored music volume of 0
 *   counts as music off: the playlist stops rather than streaming and decoding
 *   at zero gain (data, battery, and on iOS an audio session held over the
 *   player's own music).
 *
 * Sound-effect playback itself is triggered at the call sites (the WS
 * dispatcher and `MatchPage`); the engine reads the sound preference and
 * volume at play time, so the subscription here only drives the preload.
 */
export function useMatchAudio(): void {
  const soundEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.soundEnabled));
  const musicEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.musicEnabled));
  const musicVolume = resolveAudioVolume(useAuthStore((s) => s.user?.musicVolume));
  const musicAudible = musicEnabled && musicVolume > 0;

  useEffect(() => openAudioSession(), []);

  // Declared before the music effect so, on mount, the level is in place
  // before the first fade-in reads it.
  useEffect(() => {
    setMusicVolume(musicVolume);
  }, [musicVolume]);

  useEffect(() => {
    if (soundEnabled) void preloadSfx();
  }, [soundEnabled]);

  // Keyed on the boolean, not the raw volume, so moving between two non-zero
  // levels never restarts the playlist.
  useEffect(() => {
    if (!musicAudible) return;
    startMusic();
    return () => stopMusic();
  }, [musicAudible]);
}
