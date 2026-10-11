import {
  DEFAULT_AUDIO_VOLUME,
  resolveAudioEnabled,
  resolveAudioVolume,
} from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

export type SfxName =
  | "cardPlay"
  | "trickCollect"
  | "deal"
  | "dealPacket"
  | "declaration"
  | "capot"
  | "matchWin"
  | "matchLose"
  | "popup"
  | "clockTick";

const AUDIO_BASE = `${import.meta.env.BASE_URL}audio`;

function sfxSet(stem: string, count: number): string[] {
  return Array.from({ length: count }, (_, i) => `${AUDIO_BASE}/sfx/${stem}-${i + 1}.mp3`);
}

export const SFX_URLS: Readonly<Record<SfxName, readonly string[]>> = {
  cardPlay: sfxSet("card-slide", 8),
  trickCollect: sfxSet("card-shove", 4),
  deal: [`${AUDIO_BASE}/sfx/card-shuffle.mp3`],
  dealPacket: sfxSet("card-place", 4),
  declaration: sfxSet("chips-stack", 6),
  capot: [`${AUDIO_BASE}/sfx/jingle-capot.mp3`],
  matchWin: [`${AUDIO_BASE}/sfx/jingle-win.mp3`],
  matchLose: [`${AUDIO_BASE}/sfx/jingle-lose.mp3`],
  popup: sfxSet("popup", 2),
  clockTick: [`${AUDIO_BASE}/sfx/clock-tick.mp3`],
};

export const MUSIC_TRACKS: readonly string[] = [
  `${AUDIO_BASE}/music/lucky-break.mp3`,
  `${AUDIO_BASE}/music/a-good-bass-for-gambling.mp3`,
  `${AUDIO_BASE}/music/bass-meant-jazz.mp3`,
];

const SFX_GAIN_AT_DEFAULT = 0.6;
const MUSIC_GAIN_AT_DEFAULT = 0.18;
const MUSIC_FADE_IN_S = 1.5;
const MUSIC_FADE_OUT_S = 0.4;
const MUSIC_VOLUME_RAMP_S = 0.1;
const DEDUPE_KEYS_REMEMBERED = 256;
const SUSPEND_DELAY_MS = MUSIC_FADE_OUT_S * 1000 + 100;

const GESTURE_EVENTS = ["pointerdown", "pointerup", "touchend", "keydown"] as const;

type AudioContextCtor = typeof AudioContext;

/** A bus gain for a 0-100 volume: linear, and exact at the default of 70. */
function gainFor(gainAtDefault: number, volume: number | null | undefined): number {
  return gainAtDefault * (resolveAudioVolume(volume) / DEFAULT_AUDIO_VOLUME);
}

let ctx: AudioContext | null = null;
let sfxGain: GainNode | null = null;
let musicGain: GainNode | null = null;

const buffers = new Map<string, AudioBuffer>();
const loading = new Map<string, Promise<void>>();
const playedKeys = new Set<string>();

let musicEl: HTMLAudioElement | null = null;
let musicWanted = false;
let needsFreshTrack = true;
let trackIndex = 0;
let stopTimer: ReturnType<typeof setTimeout> | null = null;
let consecutiveTrackErrors = 0;
let musicLevel = MUSIC_GAIN_AT_DEFAULT;
let musicFadedIn = false;
let fadeInEndsAt = 0;

let sessions = 0;
let suspendTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Returns the unprefixed `AudioContext` constructor, or null where it is missing.
 * Prefixed-only engines decode only via callbacks, so they take the silent no-op path.
 */
function audioContextCtor(): AudioContextCtor | null {
  if (typeof window === "undefined") return null;
  return (window as Window & { AudioContext?: AudioContextCtor }).AudioContext ?? null;
}

/**
 * Creates the context with its sfx and music gain buses on first need; before a gesture it starts suspended.
 * Music goes through a gain node too because iOS ignores `HTMLMediaElement.volume`.
 */
function ensureContext(): AudioContext | null {
  if (ctx) return ctx;
  const Ctor = audioContextCtor();
  if (!Ctor) return null;
  try {
    const context = new Ctor();
    const sfx = context.createGain();
    sfx.gain.value = SFX_GAIN_AT_DEFAULT;
    sfx.connect(context.destination);
    const music = context.createGain();
    music.gain.value = 0;
    music.connect(context.destination);
    ctx = context;
    sfxGain = sfx;
    musicGain = music;
  } catch {
    return null;
  }
  return ctx;
}

function resumeContext(context: AudioContext): void {
  if (context.state === "running") return;
  try {
    void context.resume().catch(() => {});
  } catch {
    return;
  }
}

/** Fetches and decodes one asset into the cache, sharing in-flight requests; failures are swallowed for a later retry. */
function loadBuffer(context: AudioContext, url: string): Promise<void> {
  if (buffers.has(url)) return Promise.resolve();
  const inflight = loading.get(url);
  if (inflight) return inflight;
  const request = fetch(url)
    .then((res) => {
      if (!res.ok) throw new Error(`audio ${res.status}`);
      return res.arrayBuffer();
    })
    .then((data) => context.decodeAudioData(data))
    .then((buffer) => {
      buffers.set(url, buffer);
    })
    .catch(() => {})
    .finally(() => {
      loading.delete(url);
    });
  loading.set(url, request);
  return request;
}

/** Fetch and decode every sound effect so the first card plays without a gap. */
export function preloadSfx(): Promise<void> {
  const context = ensureContext();
  if (!context) return Promise.resolve();
  const urls = Object.values(SFX_URLS).flat();
  return Promise.all(urls.map((url) => loadBuffer(context, url))).then(() => undefined);
}

/**
 * Plays a random variant of a sound effect at the player's volume, only inside a session with sound on.
 * A `dedupeKey` makes the call idempotent per name, and is consumed even while sound is off.
 */
export function playSfx(name: SfxName, options: { dedupeKey?: string } = {}): void {
  if (options.dedupeKey !== undefined) {
    const key = `${name}:${options.dedupeKey}`;
    if (playedKeys.has(key)) return;
    playedKeys.add(key);
    if (playedKeys.size > DEDUPE_KEYS_REMEMBERED) {
      const oldest = playedKeys.values().next().value;
      if (oldest !== undefined) playedKeys.delete(oldest);
    }
  }
  if (sessions === 0) return;
  const user = useAuthStore.getState().user;
  if (!resolveAudioEnabled(user?.soundEnabled)) return;
  const volume = resolveAudioVolume(user?.soundVolume);
  if (volume === 0) return;

  const context = ensureContext();
  if (!context || !sfxGain) return;
  const queuedSoundsWouldBurstOnResume = context.state !== "running";
  if (queuedSoundsWouldBurstOnResume) return;

  const urls = SFX_URLS[name];
  const ready = urls.filter((url) => buffers.has(url));
  if (ready.length === 0) {
    for (const url of urls) void loadBuffer(context, url);
    return;
  }
  const url = ready[Math.floor(Math.random() * ready.length)];
  const buffer = url === undefined ? undefined : buffers.get(url);
  if (!buffer) return;
  sfxGain.gain.value = gainFor(SFX_GAIN_AT_DEFAULT, volume);
  try {
    const source = context.createBufferSource();
    source.buffer = buffer;
    source.connect(sfxGain);
    source.start();
  } catch {
    return;
  }
}

function rampMusic(target: number, seconds: number): void {
  if (!ctx || !musicGain) return;
  const now = ctx.currentTime;
  const gain = musicGain.gain;
  gain.cancelScheduledValues(now);
  gain.setValueAtTime(gain.value, now);
  gain.linearRampToValueAtTime(target, now + seconds);
}

function fadeMusicIn(): void {
  if (!ctx) return;
  musicFadedIn = true;
  fadeInEndsAt = ctx.currentTime + MUSIC_FADE_IN_S;
  rampMusic(musicLevel, MUSIC_FADE_IN_S);
}

function fadeMusicOut(): void {
  musicFadedIn = false;
  rampMusic(0, MUSIC_FADE_OUT_S);
}

/**
 * Sets the music volume (0-100), gliding to it live only while the music is faded in.
 * Otherwise the level is recorded for the next fade-in and a fade-out in progress continues.
 */
export function setMusicVolume(volume: number | null | undefined): void {
  const level = gainFor(MUSIC_GAIN_AT_DEFAULT, volume);
  if (level === musicLevel) return;
  musicLevel = level;
  if (!ctx || !musicWanted || !musicFadedIn) return;
  rampMusic(level, Math.max(MUSIC_VOLUME_RAMP_S, fadeInEndsAt - ctx.currentTime));
}

function trackUrl(index: number): string {
  return MUSIC_TRACKS[index % MUSIC_TRACKS.length] ?? "";
}

function advanceTrack(): void {
  if (!musicEl) return;
  trackIndex = (trackIndex + 1) % MUSIC_TRACKS.length;
  musicEl.src = trackUrl(trackIndex);
  if (musicWanted) void musicEl.play().catch(() => {});
}

function handleTrackEnded(): void {
  consecutiveTrackErrors = 0;
  advanceTrack();
}

function handleTrackPlaying(): void {
  consecutiveTrackErrors = 0;
}

/**
 * Skips a track that failed to load or broke mid-stream, which fires `error` and never `ended`.
 * Gives up once every track has failed in a row.
 */
function handleTrackError(): void {
  consecutiveTrackErrors += 1;
  if (consecutiveTrackErrors >= MUSIC_TRACKS.length) return;
  advanceTrack();
}

/** Creates the one music element for the engine's lifetime, since an element can feed a media source only once. */
function ensureMusicElement(context: AudioContext): HTMLAudioElement | null {
  if (musicEl) return musicEl;
  if (!musicGain) return null;
  try {
    const el = new Audio();
    el.preload = "auto";
    const source = context.createMediaElementSource(el);
    source.connect(musicGain);
    el.addEventListener("ended", handleTrackEnded);
    el.addEventListener("playing", handleTrackPlaying);
    el.addEventListener("error", handleTrackError);
    musicEl = el;
  } catch {
    return null;
  }
  return musicEl;
}

/**
 * Start the element (if needed) and fade the music in once it is actually
 * playing. Refused autoplay (no gesture yet) is swallowed; the gesture
 * listeners call back in here.
 */
function beginPlayback(): void {
  const context = ctx;
  const el = musicEl;
  if (!context || !el || !musicWanted) return;
  resumeContext(context);
  if (!el.paused) {
    fadeMusicIn();
    return;
  }
  let playing: Promise<void> | undefined;
  try {
    playing = el.play();
  } catch {
    return;
  }
  void Promise.resolve(playing)
    .then(() => {
      if (!musicWanted) {
        el.pause();
        return;
      }
      fadeMusicIn();
    })
    .catch(() => {});
}

/**
 * Start (or keep) the playlist. After a completed stop a new random track is
 * picked; during a pending fade-out the stop is cancelled and the current track
 * fades back up, so a StrictMode stop/start pair never interrupts the music.
 */
export function startMusic(): void {
  musicWanted = true;
  if (stopTimer !== null) {
    clearTimeout(stopTimer);
    stopTimer = null;
  }
  const context = ensureContext();
  if (!context) return;
  const el = ensureMusicElement(context);
  if (!el) return;
  if (needsFreshTrack) {
    needsFreshTrack = false;
    consecutiveTrackErrors = 0;
    trackIndex = Math.floor(Math.random() * MUSIC_TRACKS.length);
    el.src = trackUrl(trackIndex);
    musicFadedIn = false;
    if (musicGain) {
      musicGain.gain.cancelScheduledValues(context.currentTime);
      musicGain.gain.setValueAtTime(0, context.currentTime);
    }
  }
  beginPlayback();
}

/** Fade the music out and pause it. A `startMusic` before the fade ends wins. */
export function stopMusic(): void {
  musicWanted = false;
  if (!musicEl) return;
  fadeMusicOut();
  if (stopTimer !== null) clearTimeout(stopTimer);
  stopTimer = setTimeout(() => {
    stopTimer = null;
    if (musicWanted) return;
    musicEl?.pause();
    needsFreshTrack = true;
  }, MUSIC_FADE_OUT_S * 1000);
}

/**
 * Resumes an existing context (never creates one) and restarts music autoplay refused.
 * Touch grants activation on pointerup/touchend, so those are listened to as well as pointerdown.
 */
function handleGesture(): void {
  const context = ctx;
  if (!context) return;
  resumeContext(context);
  if (musicWanted && musicEl?.paused) beginPlayback();
}

/**
 * Opens a match-audio session, allowing sound effects and unlocking audio on the next gesture; returns the release.
 * After the last session closes, the context is suspended once the music has faded out.
 */
export function openAudioSession(): () => void {
  sessions += 1;
  if (sessions === 1) {
    if (suspendTimer !== null) {
      clearTimeout(suspendTimer);
      suspendTimer = null;
    }
    if (ctx) resumeContext(ctx);
    if (typeof window !== "undefined") {
      for (const type of GESTURE_EVENTS) {
        window.addEventListener(type, handleGesture, { capture: true, passive: true });
      }
    }
  }
  let released = false;
  return () => {
    if (released) return;
    released = true;
    sessions -= 1;
    if (sessions !== 0) return;
    if (typeof window !== "undefined") {
      for (const type of GESTURE_EVENTS) {
        window.removeEventListener(type, handleGesture, { capture: true });
      }
    }
    if (suspendTimer !== null) clearTimeout(suspendTimer);
    suspendTimer = setTimeout(() => {
      suspendTimer = null;
      if (sessions > 0 || !ctx || ctx.state !== "running") return;
      try {
        void ctx.suspend().catch(() => {});
      } catch {
        return;
      }
    }, SUSPEND_DELAY_MS);
  };
}

/** Test seam: drop every piece of engine state, including the context. */
export function __resetAudioEngineForTests(): void {
  if (stopTimer !== null) clearTimeout(stopTimer);
  if (suspendTimer !== null) clearTimeout(suspendTimer);
  if (typeof window !== "undefined") {
    for (const type of GESTURE_EVENTS) {
      window.removeEventListener(type, handleGesture, { capture: true });
    }
  }
  ctx = null;
  sfxGain = null;
  musicGain = null;
  buffers.clear();
  loading.clear();
  playedKeys.clear();
  musicEl = null;
  musicWanted = false;
  needsFreshTrack = true;
  trackIndex = 0;
  stopTimer = null;
  consecutiveTrackErrors = 0;
  musicLevel = MUSIC_GAIN_AT_DEFAULT;
  musicFadedIn = false;
  fadeInEndsAt = 0;
  sessions = 0;
  suspendTimer = null;
}
