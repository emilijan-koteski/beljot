import { useEffect } from "react";

import { resolveAudioEnabled, resolveAudioVolume } from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

import { openAudioSession, preloadSfx, setMusicVolume, startMusic, stopMusic } from "./audioEngine";

/**
 * Runs the match page audio: session, sound-effect preload, and music while it is on and audible.
 * The music volume effect runs before the music effect so the first fade-in reads the stored level.
 */
export function useMatchAudio(): void {
  const soundEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.soundEnabled));
  const musicEnabled = resolveAudioEnabled(useAuthStore((s) => s.user?.musicEnabled));
  const musicVolume = resolveAudioVolume(useAuthStore((s) => s.user?.musicVolume));
  const musicAudible = musicEnabled && musicVolume > 0;

  useEffect(() => openAudioSession(), []);

  useEffect(() => {
    setMusicVolume(musicVolume);
  }, [musicVolume]);

  useEffect(() => {
    if (soundEnabled) void preloadSfx();
  }, [soundEnabled]);

  useEffect(() => {
    if (!musicAudible) return;
    startMusic();
    return () => stopMusic();
  }, [musicAudible]);
}
