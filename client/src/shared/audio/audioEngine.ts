import {
  DEFAULT_AUDIO_VOLUME,
  resolveAudioEnabled,
  resolveAudioVolume,
} from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * The in-match audio engine: card sound effects and the background playlist
 * behind ONE Web Audio graph.
 *
 *   sfx buffers ──► sfxGain ───┐
 *                              ├──► destination
 *   <audio> ──► mediaSource ──► musicGain ─┘
 *
 * Why Web Audio for the music too, rather than a bare `<audio>` element: iOS
 * treats `HTMLMediaElement.volume` as read-only (always 1), so the only way to
 * sit the music under the card sounds — and to fade it — on every platform is a
 * gain node. Routing both buses through one `AudioContext` also means a single
 * user gesture unlocks both.
 *
 * Assets are MP3 only: Safari / iOS before 18.4 cannot `decodeAudioData` Ogg.
 * Provenance and the transcode recipe live in `docs/audio.md`.
 *
 * Loudness: each bus's gain is its gain-at-default scaled linearly by the
 * player's 0-100 volume for that channel, so the default volume of 70 plays at
 * exactly the levels the game shipped with before volumes existed, and 0 is
 * silent. The on/off switches are separate and untouched by the volumes.
 *
 * Everything here is a silent no-op where `AudioContext` does not exist (jsdom,
 * very old browsers), and every playback failure is swallowed: sound is
 * feedback, never something a player should see an error for.
 */

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

/**
 * Recordings per sound. Where there are several, one is picked at random so
 * repeats don't sound canned; the one-off moments (a jingle, the shuffle, the
 * clock) have a single recording each.
 */
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

/** The rotating playlist. A random track starts; the rest follow in order. */
export const MUSIC_TRACKS: readonly string[] = [
  `${AUDIO_BASE}/music/lucky-break.mp3`,
  `${AUDIO_BASE}/music/a-good-bass-for-gambling.mp3`,
  `${AUDIO_BASE}/music/bass-meant-jazz.mp3`,
];

/** Bus gains at the default volume of 70 — the pre-volume shipping levels. */
const SFX_GAIN_AT_DEFAULT = 0.6;
const MUSIC_GAIN_AT_DEFAULT = 0.18;
const MUSIC_FADE_IN_S = 1.5;
const MUSIC_FADE_OUT_S = 0.4;
/** How fast a live music-volume change glides to its new level. */
const MUSIC_VOLUME_RAMP_S = 0.1;
/**
 * How many dedupe keys are remembered. A hand spends about 25–35: a shuffle,
 * 12 deal packets (8 + 4, or 12), 8 collects, a reveal or two — more with a
 * reshuffle or urgent ticks. So this holds the last seven or so hands, far
 * more than any remount can reach back for.
 */
const DEDUPE_MEMORY = 256;
/**
 * How long after the last session closes the context is suspended: past the
 * music fade-out, so the fade is heard to the end rather than cut off.
 */
const SUSPEND_DELAY_MS = MUSIC_FADE_OUT_S * 1000 + 100;

/**
 * Activation-triggering inputs. `pointerdown` alone is not enough: for touch
 * the browser grants activation on `pointerup` / `touchend`, not on the down.
 */
const GESTURE_EVENTS = ["pointerdown", "pointerup", "touchend", "keydown"] as const;

type AudioContextCtor = typeof AudioContext;

/** A bus gain for a 0-100 volume: linear, and exact at the default of 70. */
function gainFor(gainAtDefault: number, volume: number | null | undefined): number {
  // Ratio first, so the default volume multiplies by exactly 1.
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
/** Tracks that failed in a row; at the playlist length the engine gives up. */
let consecutiveTrackErrors = 0;
/** The music gain the player's volume asks for; every fade-in targets it. */
let musicLevel = MUSIC_GAIN_AT_DEFAULT;
/**
 * Whether the music has been faded up (or is fading up) toward `musicLevel`:
 * set when a fade-in is scheduled, cleared by a stop or a fresh track. Only
 * then does a volume change move the gain live — before it, the pending
 * fade-in picks the new level up; during a fade-out it must not undo the fade.
 */
let musicFadedIn = false;
/** Context time the current fade-in lands, so a volume change can keep it. */
let fadeInEndsAt = 0;

/** Open match-audio sessions; SFX only ever sound while one is open. */
let sessions = 0;
let suspendTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Unprefixed `AudioContext` only. Engines that ship just `webkitAudioContext`
 * support only the callback form of `decodeAudioData`, so no sound effect
 * could ever decode there — they take the silent no-op path instead.
 */
function audioContextCtor(): AudioContextCtor | null {
  if (typeof window === "undefined") return null;
  return (window as Window & { AudioContext?: AudioContextCtor }).AudioContext ?? null;
}

/**
 * Create the context on first need. Created before any gesture it starts
 * `suspended`; the gesture listeners resume it.
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
    // Some engines throw synchronously outside a gesture; the next one retries.
  }
}

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
    // A failed load is retried by the next preload or play; production answers
    // a missing asset with index.html, which lands here as a decode error.
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
 * Play one random variant of a sound effect, if the player has sound on, at
 * the player's sound-effects volume (both read at play time; volume 0 plays
 * nothing).
 *
 * `dedupeKey` makes a call idempotent: a second call with the same name and key
 * is dropped, so an effect that re-runs on remount (a reconnect mid-trick keeps
 * the resolved-trick snapshot alive) cannot sound the same moment twice. The key
 * is consumed even when sound is off, so switching sound on afterwards does not
 * replay an old moment either.
 */
export function playSfx(name: SfxName, options: { dedupeKey?: string } = {}): void {
  if (options.dedupeKey !== undefined) {
    const key = `${name}:${options.dedupeKey}`;
    if (playedKeys.has(key)) return;
    playedKeys.add(key);
    if (playedKeys.size > DEDUPE_MEMORY) {
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
  // A source started on a suspended context is held, then released the moment
  // the context resumes — every card played before the first click would burst
  // out at once. Before the unlock, sounds are simply dropped.
  if (context.state !== "running") return;

  const urls = SFX_URLS[name];
  const ready = urls.filter((url) => buffers.has(url));
  if (ready.length === 0) {
    for (const url of urls) void loadBuffer(context, url);
    return;
  }
  const url = ready[Math.floor(Math.random() * ready.length)];
  const buffer = url === undefined ? undefined : buffers.get(url);
  if (!buffer) return;
  // Effects are a fraction of a second long, so setting the bus level per play
  // is exact enough: at most a still-ringing tail shifts with it.
  sfxGain.gain.value = gainFor(SFX_GAIN_AT_DEFAULT, volume);
  try {
    const source = context.createBufferSource();
    source.buffer = buffer;
    source.connect(sfxGain);
    source.start();
  } catch {
    // A decoded buffer that cannot start is not worth surfacing.
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
 * Set the music volume (0-100; missing means the default). While the music is
 * faded in it glides to the new level in ~0.1 s — or, mid fade-in, keeps the
 * fade's own landing time — so a slider drag is heard live. Otherwise it only
 * records the level: the next fade-in targets it, and a fade-out in progress
 * carries on to silence.
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
 * A track that fails to load or breaks mid-stream fires `error`, never
 * `ended` — without this the playlist would stall on it for the rest of the
 * session. Skip to the next track, but stop once every track has failed in a
 * row, so an unreachable playlist cannot loop forever.
 */
function handleTrackError(): void {
  consecutiveTrackErrors += 1;
  if (consecutiveTrackErrors >= MUSIC_TRACKS.length) return;
  advanceTrack();
}

function ensureMusicElement(context: AudioContext): HTMLAudioElement | null {
  if (musicEl) return musicEl;
  if (!musicGain) return null;
  try {
    const el = new Audio();
    el.preload = "auto";
    // One element for the engine's lifetime: an element can be wired into a
    // media source exactly once.
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
 * Resume a context that exists; never create one. With sound and music both
 * off nothing has created a context, and a click must not start one either.
 */
function handleGesture(): void {
  const context = ctx;
  if (!context) return;
  resumeContext(context);
  if (musicWanted && musicEl?.paused) beginPlayback();
}

/**
 * Open a match-audio session: sound effects may play while at least one is
 * open, and the first pointer / key input resumes the context and starts any
 * music that autoplay refused. Returns the release function.
 *
 * Once the last session closes, the context is suspended after the music has
 * had time to fade out, so an idle SPA does not keep the audio device running;
 * the next session cancels a pending suspend and resumes the context.
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
        // A context that cannot suspend just stays idle.
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
