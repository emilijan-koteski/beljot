import { existsSync } from "node:fs";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import {
  __resetAudioEngineForTests,
  MUSIC_TRACKS,
  openAudioSession,
  playSfx,
  preloadSfx,
  setMusicVolume,
  SFX_URLS,
  startMusic,
  stopMusic,
} from "./audioEngine";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

// --- Web Audio / media fakes -------------------------------------------------
// jsdom ships neither AudioContext nor a working HTMLMediaElement.play, so the
// engine is exercised against minimal recorders of what it asks for.

class FakeParam {
  value = 0;
  lastRampTarget: number | null = null;
  lastRampEnd: number | null = null;
  cancelScheduledValues = vi.fn();
  setValueAtTime = vi.fn((v: number) => {
    this.value = v;
  });
  linearRampToValueAtTime = vi.fn((v: number, endTime: number) => {
    this.lastRampTarget = v;
    this.lastRampEnd = endTime;
  });
}

class FakeGain {
  gain = new FakeParam();
  connect = vi.fn();
}

class FakeBufferSource {
  buffer: unknown = null;
  connect = vi.fn();
  start = vi.fn();
}

class FakeMediaSource {
  readonly element: unknown;
  constructor(element: unknown) {
    this.element = element;
  }
  connect = vi.fn();
}

class FakeAudioContext {
  static instances: FakeAudioContext[] = [];
  static initialState: AudioContextState = "running";
  state: AudioContextState = FakeAudioContext.initialState;
  currentTime = 0;
  destination = {};
  gains: FakeGain[] = [];
  sources: FakeBufferSource[] = [];
  mediaSources: FakeMediaSource[] = [];
  constructor() {
    FakeAudioContext.instances.push(this);
  }
  createGain() {
    const g = new FakeGain();
    this.gains.push(g);
    return g;
  }
  createBufferSource() {
    const s = new FakeBufferSource();
    this.sources.push(s);
    return s;
  }
  createMediaElementSource(el: unknown) {
    const m = new FakeMediaSource(el);
    this.mediaSources.push(m);
    return m;
  }
  decodeAudioData = vi.fn(async (data: ArrayBuffer & { url?: string }) => ({ url: data.url }));
  resume = vi.fn(async () => {
    this.state = "running";
  });
  suspend = vi.fn(async () => {
    this.state = "suspended";
  });
  /** [sfxGain, musicGain] — created in that order. */
  get sfxGain() {
    return this.gains[0];
  }
  get musicGain() {
    return this.gains[1];
  }
}

class FakeAudio {
  static instances: FakeAudio[] = [];
  static rejectPlay = false;
  src = "";
  preload = "";
  paused = true;
  private listeners = new Map<string, Array<() => void>>();
  constructor() {
    FakeAudio.instances.push(this);
  }
  play = vi.fn(() => {
    if (FakeAudio.rejectPlay) return Promise.reject(new Error("NotAllowedError"));
    this.paused = false;
    return Promise.resolve();
  });
  pause = vi.fn(() => {
    this.paused = true;
  });
  addEventListener(type: string, fn: () => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
  }
  emit(type: string) {
    for (const fn of this.listeners.get(type) ?? []) fn();
  }
}

const fetchMock = vi.fn(async (url: string) => ({
  ok: true,
  arrayBuffer: async () => Object.assign(new ArrayBuffer(8), { url }),
}));

function context(): FakeAudioContext {
  const c = FakeAudioContext.instances[0];
  if (!c) throw new Error("no AudioContext was created");
  return c;
}

function music(): FakeAudio {
  const a = FakeAudio.instances[0];
  if (!a) throw new Error("no music element was created");
  return a;
}

/** Let chained promise callbacks (fetch → decode, play → fade) settle. */
async function flush() {
  for (let i = 0; i < 5; i++) await Promise.resolve();
}

function setAudioPrefs(prefs: {
  soundEnabled?: boolean;
  musicEnabled?: boolean;
  soundVolume?: number;
}) {
  useAuthStore.setState({ user: { ...makeUser(), ...prefs } });
}

beforeEach(() => {
  __resetAudioEngineForTests();
  FakeAudioContext.instances = [];
  FakeAudioContext.initialState = "running";
  FakeAudio.instances = [];
  FakeAudio.rejectPlay = false;
  fetchMock.mockClear();
  vi.stubGlobal("AudioContext", FakeAudioContext);
  vi.stubGlobal("Audio", FakeAudio);
  vi.stubGlobal("fetch", fetchMock);
  useAuthStore.setState({ token: "t", user: makeUser(), isLoading: false });
});

afterEach(() => {
  __resetAudioEngineForTests();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("without Web Audio", () => {
  it("is a silent no-op end to end", async () => {
    vi.stubGlobal("AudioContext", undefined);
    const release = openAudioSession();

    await expect(preloadSfx()).resolves.toBeUndefined();
    expect(() => playSfx("cardPlay")).not.toThrow();
    expect(() => startMusic()).not.toThrow();
    expect(() => stopMusic()).not.toThrow();
    release();

    expect(fetchMock).not.toHaveBeenCalled();
    expect(FakeAudio.instances).toHaveLength(0);
  });
});

describe("prefixed-only Web Audio", () => {
  it("takes the no-op path — webkitAudioContext cannot decode with promises", async () => {
    vi.stubGlobal("AudioContext", undefined);
    vi.stubGlobal("webkitAudioContext", FakeAudioContext);
    openAudioSession();

    await preloadSfx();
    playSfx("cardPlay");
    startMusic();

    expect(FakeAudioContext.instances).toHaveLength(0);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("sound effects", () => {
  it("preloads every variant of every effect", async () => {
    await preloadSfx();

    const all = [...SFX_URLS.cardPlay, ...SFX_URLS.trickCollect];
    expect(all).toHaveLength(12);
    expect(fetchMock.mock.calls.map(([url]) => url).sort()).toEqual([...all].sort());
    expect(context().decodeAudioData).toHaveBeenCalledTimes(12);
    expect(all.every((url) => url.endsWith(".mp3"))).toBe(true);
  });

  it("plays one random card-slide variant through the sfx bus", async () => {
    openAudioSession();
    await preloadSfx();
    vi.spyOn(Math, "random").mockReturnValue(0.99);

    playSfx("cardPlay");

    expect(context().sources).toHaveLength(1);
    const source = context().sources[0]!;
    expect(source.start).toHaveBeenCalledTimes(1);
    expect(source.connect).toHaveBeenCalledWith(context().sfxGain);
    expect(source.buffer).toEqual({ url: SFX_URLS.cardPlay[7] });
    expect(context().sfxGain!.gain.value).toBeCloseTo(0.6);
  });

  it("draws the trick-collect sound from the card-shove set", async () => {
    openAudioSession();
    await preloadSfx();
    vi.spyOn(Math, "random").mockReturnValue(0);

    playSfx("trickCollect");

    expect(context().sources[0]!.buffer).toEqual({ url: SFX_URLS.trickCollect[0] });
    expect(SFX_URLS.trickCollect[0]).toMatch(/card-shove-1\.mp3$/);
  });

  it("reads the sound switch at play time", async () => {
    openAudioSession();
    await preloadSfx();

    setAudioPrefs({ soundEnabled: false });
    playSfx("cardPlay");
    expect(context().sources).toHaveLength(0);

    setAudioPrefs({ soundEnabled: true });
    playSfx("cardPlay");
    expect(context().sources).toHaveLength(1);
  });

  it("treats a missing sound switch as ON", async () => {
    openAudioSession();
    await preloadSfx();
    useAuthStore.setState({ user: { ...makeUser(), soundEnabled: undefined as never } });

    playSfx("cardPlay");

    expect(context().sources).toHaveLength(1);
  });

  it("is silent outside an audio session (no audio off the match page)", async () => {
    await preloadSfx();
    playSfx("cardPlay");
    expect(context().sources).toHaveLength(0);

    const release = openAudioSession();
    release();
    playSfx("cardPlay");
    expect(context().sources).toHaveLength(0);
  });

  it("drops sounds while the context is locked instead of bursting them on unlock", async () => {
    FakeAudioContext.initialState = "suspended";
    openAudioSession();
    await preloadSfx();

    playSfx("cardPlay");
    playSfx("cardPlay");
    window.dispatchEvent(new Event("pointerdown"));
    await flush();

    expect(context().state).toBe("running");
    expect(context().sources).toHaveLength(0);

    playSfx("cardPlay");
    expect(context().sources).toHaveLength(1);
  });

  it("plays a dedupe-keyed sound at most once per key", async () => {
    openAudioSession();
    await preloadSfx();

    playSfx("trickCollect", { dedupeKey: "1700000000000" });
    playSfx("trickCollect", { dedupeKey: "1700000000000" });
    expect(context().sources).toHaveLength(1);

    playSfx("trickCollect", { dedupeKey: "1700000000001" });
    expect(context().sources).toHaveLength(2);
  });

  it("consumes a dedupe key even while sound is off", async () => {
    openAudioSession();
    await preloadSfx();

    setAudioPrefs({ soundEnabled: false });
    playSfx("trickCollect", { dedupeKey: "42" });
    setAudioPrefs({ soundEnabled: true });
    playSfx("trickCollect", { dedupeKey: "42" });

    expect(context().sources).toHaveLength(0);
  });

  it("skips (and starts loading) when nothing has decoded yet", () => {
    openAudioSession();

    playSfx("cardPlay");

    expect(context().sources).toHaveLength(0);
    expect(fetchMock).toHaveBeenCalledTimes(SFX_URLS.cardPlay.length);
  });

  it("swallows a failed load and retries it on the next preload", async () => {
    fetchMock.mockImplementationOnce(async () => ({
      ok: false,
      arrayBuffer: async () => Object.assign(new ArrayBuffer(0), { url: "" }),
    }));

    await expect(preloadSfx()).resolves.toBeUndefined();
    expect(context().decodeAudioData).toHaveBeenCalledTimes(11);

    await preloadSfx();
    expect(context().decodeAudioData).toHaveBeenCalledTimes(12);
  });
});

describe("music", () => {
  it("routes a random first track through the music gain and fades it in", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.5);

    startMusic();

    expect(music().src).toBe(MUSIC_TRACKS[1]);
    expect(context().mediaSources).toHaveLength(1);
    expect(context().mediaSources[0]!.element).toBe(music());
    expect(context().mediaSources[0]!.connect).toHaveBeenCalledWith(context().musicGain);
    expect(music().play).toHaveBeenCalledTimes(1);

    await flush();
    expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(0.18);
    expect(context().musicGain!.gain.lastRampEnd).toBeCloseTo(1.5);
  });

  it("ships the three playlist tracks as MP3", () => {
    expect(MUSIC_TRACKS.map((url) => url.split("/").pop())).toEqual([
      "lucky-break.mp3",
      "a-good-bass-for-gambling.mp3",
      "bass-meant-jazz.mp3",
    ]);
  });

  it("swallows a refused autoplay and starts on the first gesture", async () => {
    FakeAudioContext.initialState = "suspended";
    FakeAudio.rejectPlay = true;
    const release = openAudioSession();

    startMusic();
    await flush();
    expect(context().musicGain!.gain.lastRampTarget).toBeNull();

    FakeAudio.rejectPlay = false;
    window.dispatchEvent(new Event("keydown"));
    await flush();

    expect(context().resume).toHaveBeenCalled();
    expect(music().play).toHaveBeenCalledTimes(2);
    expect(music().paused).toBe(false);
    expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(0.18);
    release();
  });

  it("advances through the playlist in order and wraps around", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0.99);
    startMusic();
    await flush();
    expect(music().src).toBe(MUSIC_TRACKS[2]);

    music().emit("ended");
    expect(music().src).toBe(MUSIC_TRACKS[0]);
    expect(music().play).toHaveBeenCalledTimes(2);

    music().emit("ended");
    expect(music().src).toBe(MUSIC_TRACKS[1]);
  });

  it("fades out, then pauses once the fade has run", async () => {
    vi.useFakeTimers();
    startMusic();
    await flush();

    stopMusic();
    expect(context().musicGain!.gain.lastRampTarget).toBe(0);
    expect(music().pause).not.toHaveBeenCalled();

    vi.advanceTimersByTime(400);
    expect(music().pause).toHaveBeenCalledTimes(1);
  });

  it("lets a start during the fade-out cancel the pending stop", async () => {
    vi.useFakeTimers();
    startMusic();
    await flush();
    const track = music().src;

    stopMusic();
    vi.advanceTimersByTime(200);
    startMusic();
    await flush();
    vi.advanceTimersByTime(1000);

    expect(music().pause).not.toHaveBeenCalled();
    expect(music().src).toBe(track);
    expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(0.18);
  });

  it("picks a fresh random track after a completed stop", async () => {
    vi.useFakeTimers();
    const random = vi.spyOn(Math, "random").mockReturnValue(0);
    startMusic();
    await flush();
    expect(music().src).toBe(MUSIC_TRACKS[0]);

    stopMusic();
    vi.advanceTimersByTime(400);
    random.mockReturnValue(0.99);
    startMusic();
    await flush();

    expect(FakeAudio.instances).toHaveLength(1);
    expect(context().mediaSources).toHaveLength(1);
    expect(music().src).toBe(MUSIC_TRACKS[2]);
    expect(music().paused).toBe(false);
  });

  it("pauses a play that resolves after the music was stopped", async () => {
    let resolvePlay: () => void = () => {};
    startMusic();
    music().play.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolvePlay = resolve;
        }),
    );
    // First play already resolved; force a fresh start path.
    await flush();
    music().paused = true;
    startMusic();
    stopMusic();
    resolvePlay();
    await flush();

    expect(music().pause).toHaveBeenCalled();
  });
});

describe("music errors", () => {
  it("skips a track that fails to load and plays the next", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    startMusic();
    await flush();
    expect(music().src).toBe(MUSIC_TRACKS[0]);

    music().emit("error");

    expect(music().src).toBe(MUSIC_TRACKS[1]);
    expect(music().play).toHaveBeenCalledTimes(2);
  });

  it("gives up once every track has failed in a row", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    startMusic();
    await flush();

    music().emit("error");
    music().emit("error");
    expect(music().src).toBe(MUSIC_TRACKS[2]);
    const plays = music().play.mock.calls.length;

    // Third consecutive failure: the whole playlist is unreachable — stop.
    music().emit("error");
    music().emit("error");
    expect(music().src).toBe(MUSIC_TRACKS[2]);
    expect(music().play).toHaveBeenCalledTimes(plays);
  });

  it("resets the failure count once a track actually plays", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    startMusic();
    await flush();

    music().emit("error");
    music().emit("error");
    music().emit("playing");
    music().emit("error");

    expect(music().src).toBe(MUSIC_TRACKS[0]);
  });

  it("resets the failure count when a track ends normally", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    startMusic();
    await flush();

    music().emit("error");
    music().emit("error");
    music().emit("ended");
    music().emit("error");

    expect(music().src).toBe(MUSIC_TRACKS[1]);
  });
});

describe("context suspend", () => {
  it("suspends the context after the last session closes, once the fade has run", async () => {
    vi.useFakeTimers();
    const release = openAudioSession();
    await preloadSfx();
    startMusic();
    await flush();

    stopMusic();
    release();
    vi.advanceTimersByTime(400);
    expect(context().suspend).not.toHaveBeenCalled();
    expect(music().pause).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(100);
    expect(context().suspend).toHaveBeenCalledTimes(1);
    await flush();
    expect(context().state).toBe("suspended");
  });

  it("cancels a pending suspend when a session reopens", async () => {
    vi.useFakeTimers();
    const release = openAudioSession();
    await preloadSfx();

    release();
    vi.advanceTimersByTime(200);
    openAudioSession();
    vi.advanceTimersByTime(1000);

    expect(context().suspend).not.toHaveBeenCalled();
    expect(context().state).toBe("running");
  });

  it("resumes a suspended context when the next session opens", async () => {
    vi.useFakeTimers();
    const release = openAudioSession();
    await preloadSfx();
    release();
    vi.advanceTimersByTime(500);
    await flush();
    expect(context().state).toBe("suspended");

    openAudioSession();
    await flush();

    expect(context().resume).toHaveBeenCalledTimes(1);
    expect(context().state).toBe("running");
    playSfx("cardPlay");
    expect(context().sources).toHaveLength(1);
  });

  it("does not suspend while another session is still open", async () => {
    vi.useFakeTimers();
    const releaseA = openAudioSession();
    openAudioSession();
    await preloadSfx();

    releaseA();
    vi.advanceTimersByTime(1000);

    expect(context().suspend).not.toHaveBeenCalled();
  });
});

describe("gesture unlock", () => {
  it("never creates a context — with sound and music off none should exist", () => {
    openAudioSession();

    window.dispatchEvent(new Event("pointerdown"));
    window.dispatchEvent(new Event("keydown"));

    expect(FakeAudioContext.instances).toHaveLength(0);
  });

  it("stops listening once the last session is released", async () => {
    FakeAudioContext.initialState = "suspended";
    const releaseA = openAudioSession();
    const releaseB = openAudioSession();
    await preloadSfx();

    releaseA();
    releaseA();
    window.dispatchEvent(new Event("pointerup"));
    await flush();
    expect(context().resume).toHaveBeenCalledTimes(1);

    context().state = "suspended";
    releaseB();
    window.dispatchEvent(new Event("pointerdown"));
    await flush();
    expect(context().resume).toHaveBeenCalledTimes(1);
  });
});

describe("volumes", () => {
  /** Gains the engine shipped with before volumes, i.e. at the default 70. */
  const SFX_AT_DEFAULT = 0.6;
  const MUSIC_AT_DEFAULT = 0.18;

  describe("sound effects", () => {
    it("plays at the pre-volume gain at the default of 70, stored or missing", async () => {
      openAudioSession();
      await preloadSfx();

      setAudioPrefs({ soundVolume: 70 });
      playSfx("cardPlay");
      expect(context().sfxGain!.gain.value).toBe(SFX_AT_DEFAULT);

      useAuthStore.setState({ user: { ...makeUser(), soundVolume: undefined as never } });
      playSfx("cardPlay");
      expect(context().sfxGain!.gain.value).toBe(SFX_AT_DEFAULT);
      expect(context().sources).toHaveLength(2);
    });

    it("scales the sfx bus linearly with the volume, read at play time", async () => {
      openAudioSession();
      await preloadSfx();

      setAudioPrefs({ soundVolume: 35 });
      playSfx("cardPlay");
      expect(context().sfxGain!.gain.value).toBeCloseTo(SFX_AT_DEFAULT / 2);

      setAudioPrefs({ soundVolume: 100 });
      playSfx("trickCollect");
      expect(context().sfxGain!.gain.value).toBeCloseTo((SFX_AT_DEFAULT * 100) / 70);
    });

    it("is silent at volume 0 with the switch on", async () => {
      openAudioSession();
      await preloadSfx();
      setAudioPrefs({ soundEnabled: true, soundVolume: 0 });

      playSfx("cardPlay");

      expect(context().sources).toHaveLength(0);
    });
  });

  describe("music", () => {
    it("fades in to the pre-volume gain at the default of 70", async () => {
      setMusicVolume(70);
      startMusic();
      await flush();

      expect(context().musicGain!.gain.lastRampTarget).toBe(MUSIC_AT_DEFAULT);
    });

    it("fades in to the level set before it started", async () => {
      setMusicVolume(35);
      startMusic();
      await flush();

      expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(MUSIC_AT_DEFAULT / 2);
      expect(context().musicGain!.gain.lastRampEnd).toBeCloseTo(1.5);
    });

    it("fades in to silence at volume 0", async () => {
      setMusicVolume(0);
      startMusic();
      await flush();

      expect(context().musicGain!.gain.lastRampTarget).toBe(0);
    });

    it("glides to a new level live once faded in", async () => {
      startMusic();
      await flush();
      context().currentTime = 5;

      setMusicVolume(35);

      expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(MUSIC_AT_DEFAULT / 2);
      expect(context().musicGain!.gain.lastRampEnd).toBeCloseTo(5.1);
    });

    it("keeps the fade-in's landing time when changed mid fade-in", async () => {
      startMusic();
      await flush();
      context().currentTime = 0.5;

      setMusicVolume(100);

      expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo((MUSIC_AT_DEFAULT * 100) / 70);
      expect(context().musicGain!.gain.lastRampEnd).toBeCloseTo(1.5);
    });

    it("does not undo a fade-out, and the next start targets the new level", async () => {
      vi.useFakeTimers();
      startMusic();
      await flush();

      stopMusic();
      const ramps = context().musicGain!.gain.linearRampToValueAtTime.mock.calls.length;
      setMusicVolume(35);
      expect(context().musicGain!.gain.linearRampToValueAtTime).toHaveBeenCalledTimes(ramps);
      expect(context().musicGain!.gain.lastRampTarget).toBe(0);

      startMusic();
      await flush();
      expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(MUSIC_AT_DEFAULT / 2);
    });

    it("waits for a refused autoplay, then fades in to the level set meanwhile", async () => {
      FakeAudioContext.initialState = "suspended";
      FakeAudio.rejectPlay = true;
      const release = openAudioSession();
      startMusic();
      await flush();

      setMusicVolume(35);
      expect(context().musicGain!.gain.lastRampTarget).toBeNull();

      FakeAudio.rejectPlay = false;
      window.dispatchEvent(new Event("pointerdown"));
      await flush();

      expect(context().musicGain!.gain.lastRampTarget).toBeCloseTo(MUSIC_AT_DEFAULT / 2);
      release();
    });

    it("does nothing for a level it already has", async () => {
      startMusic();
      await flush();
      const ramps = context().musicGain!.gain.linearRampToValueAtTime.mock.calls.length;

      setMusicVolume(70);
      setMusicVolume(undefined);

      expect(context().musicGain!.gain.linearRampToValueAtTime).toHaveBeenCalledTimes(ramps);
    });

    it("never creates a context on its own", () => {
      setMusicVolume(30);

      expect(FakeAudioContext.instances).toHaveLength(0);
    });
  });
});

describe("audio assets", () => {
  // A missing file fails silently at every other layer — the engine swallows
  // load errors and production answers with index.html — so this is the only
  // thing standing between a renamed asset and a silent table.
  const PUBLIC_DIR = join(process.cwd(), "public");
  const BASE = import.meta.env.BASE_URL;

  it.each([...SFX_URLS.cardPlay, ...SFX_URLS.trickCollect, ...MUSIC_TRACKS])("ships %s", (url) => {
    expect(url.startsWith(`${BASE}audio/`)).toBe(true);
    expect(existsSync(join(PUBLIC_DIR, url.slice(BASE.length)))).toBe(true);
  });
});
