import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import {
  DEFAULT_AUDIO_VOLUME,
  persistAudioPreferences,
  resetAudioPreferenceRequestSequence,
  resolveAudioEnabled,
  resolveAudioVolume,
} from "./audioPreference";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

function deferred() {
  let reject: (e: Error) => void = () => {};
  let resolve: (v: unknown) => void = () => {};
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function audioState() {
  const user = useAuthStore.getState().user;
  return { sound: user?.soundEnabled, music: user?.musicEnabled };
}

function volumeState() {
  const user = useAuthStore.getState().user;
  return { sound: user?.soundVolume, music: user?.musicVolume };
}

describe("resolveAudioEnabled", () => {
  it.each([
    [true, true],
    [false, false],
    [undefined, true],
    [null, true],
  ])("resolves %o to %o — a missing switch is ON", (value, expected) => {
    expect(resolveAudioEnabled(value)).toBe(expected);
  });
});

describe("resolveAudioVolume", () => {
  it("defaults to 70 — the pre-volume loudness", () => {
    expect(DEFAULT_AUDIO_VOLUME).toBe(70);
  });

  it.each([
    [0, 0],
    [30, 30],
    [70, 70],
    [100, 100],
    [undefined, 70],
    [null, 70],
    [Number.NaN, 70],
    [Number.POSITIVE_INFINITY, 70],
    [-1, 0],
    [101, 100],
    [50.4, 50],
    [50.5, 51],
  ])("resolves %o to %o — missing means the default, never silence", (value, expected) => {
    expect(resolveAudioVolume(value)).toBe(expected);
  });
});

describe("persistAudioPreferences", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({});
    resetAudioPreferenceRequestSequence();
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 7, soundEnabled: true, musicEnabled: true }),
      isLoading: false,
    });
  });

  it("writes the store before the request resolves", () => {
    const pending = deferred();
    mockUpdatePreferences.mockReturnValueOnce(pending.promise);

    void persistAudioPreferences({ soundEnabled: false });

    expect(audioState()).toEqual({ sound: false, music: true });
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundEnabled: false });
  });

  it("PATCHes only the fields that actually change", async () => {
    useAuthStore.setState({ user: makeUser({ id: 7, soundEnabled: true, musicEnabled: false }) });

    await persistAudioPreferences({ soundEnabled: false, musicEnabled: false });

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundEnabled: false });
    expect(audioState()).toEqual({ sound: false, music: false });
  });

  it("sends both fields in ONE request when both change", async () => {
    await persistAudioPreferences({ soundEnabled: false, musicEnabled: false });

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, {
      soundEnabled: false,
      musicEnabled: false,
    });
  });

  it("sends nothing when no field changes", async () => {
    await persistAudioPreferences({ soundEnabled: true, musicEnabled: true });
    await persistAudioPreferences({});

    expect(mockUpdatePreferences).not.toHaveBeenCalled();
  });

  it("treats a missing stored field as ON when deciding what changed", async () => {
    useAuthStore.setState({
      user: { ...makeUser({ id: 7 }), soundEnabled: undefined as never },
    });

    await persistAudioPreferences({ soundEnabled: true });
    expect(mockUpdatePreferences).not.toHaveBeenCalled();

    await persistAudioPreferences({ soundEnabled: false });
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundEnabled: false });
  });

  it("reverts only the failed request's own fields", async () => {
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    useAuthStore.setState({ user: makeUser({ id: 7, soundEnabled: true, musicEnabled: false }) });

    await persistAudioPreferences({ musicEnabled: true });

    expect(audioState()).toEqual({ sound: true, music: false });
  });

  it("leaves a field alone when a later write of it superseded the failure", async () => {
    // off (fails late) → on → off. The first write's rollback target is ON,
    // which would silently un-mute the player's actual last choice.
    const first = deferred();
    mockUpdatePreferences.mockReturnValueOnce(first.promise);

    const firstWrite = persistAudioPreferences({ soundEnabled: false });
    await persistAudioPreferences({ soundEnabled: true });
    await persistAudioPreferences({ soundEnabled: false });
    expect(audioState().sound).toBe(false);

    first.reject(new Error("late failure"));
    await firstWrite;

    expect(audioState()).toEqual({ sound: false, music: true });
  });

  it("still reverts a field that no later write touched", async () => {
    // Mute both (fails late), then flip sound on (succeeds): sound is owned by
    // the second write, but music was only ever written by the first, so its
    // failure must still roll music back.
    const mute = deferred();
    mockUpdatePreferences.mockReturnValueOnce(mute.promise).mockResolvedValueOnce({});

    const muteWrite = persistAudioPreferences({ soundEnabled: false, musicEnabled: false });
    await persistAudioPreferences({ soundEnabled: true });

    mute.reject(new Error("late failure"));
    await muteWrite;

    expect(audioState()).toEqual({ sound: true, music: true });
    expect(mockUpdatePreferences).toHaveBeenNthCalledWith(2, 7, { soundEnabled: true });
  });

  it("does not revert onto a different account", async () => {
    const pending = deferred();
    mockUpdatePreferences.mockReturnValueOnce(pending.promise);

    const write = persistAudioPreferences({ soundEnabled: false });
    useAuthStore.setState({ user: makeUser({ id: 8, soundEnabled: false }) });
    pending.reject(new Error("offline"));
    await write;

    expect(useAuthStore.getState().user).toMatchObject({ id: 8, soundEnabled: false });
  });

  it("is a no-op when signed out", async () => {
    useAuthStore.setState({ user: null });

    await persistAudioPreferences({ soundEnabled: false });

    expect(mockUpdatePreferences).not.toHaveBeenCalled();
  });

  describe("volumes", () => {
    beforeEach(() => {
      useAuthStore.setState({
        user: makeUser({ id: 7, soundVolume: 70, musicVolume: 70 }),
      });
    });

    it("writes a volume to the store before the request resolves", () => {
      const pending = deferred();
      mockUpdatePreferences.mockReturnValueOnce(pending.promise);

      void persistAudioPreferences({ musicVolume: 30 });

      expect(volumeState()).toEqual({ sound: 70, music: 30 });
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicVolume: 30 });
    });

    it("sends 0 — silent is a real level, not an absent field", async () => {
      await persistAudioPreferences({ soundVolume: 0 });

      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundVolume: 0 });
      expect(volumeState()).toEqual({ sound: 0, music: 70 });
    });

    it("clamps and rounds before comparing and sending", async () => {
      await persistAudioPreferences({ soundVolume: 130, musicVolume: 29.6 });

      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundVolume: 100, musicVolume: 30 });
      expect(volumeState()).toEqual({ sound: 100, music: 30 });
    });

    it("sends nothing when the level does not change", async () => {
      await persistAudioPreferences({ soundVolume: 70, musicVolume: 70.2 });

      expect(mockUpdatePreferences).not.toHaveBeenCalled();
    });

    it("treats a missing stored volume as 70 when deciding what changed", async () => {
      useAuthStore.setState({
        user: { ...makeUser({ id: 7 }), musicVolume: undefined as never },
      });

      await persistAudioPreferences({ musicVolume: 70 });
      expect(mockUpdatePreferences).not.toHaveBeenCalled();

      await persistAudioPreferences({ musicVolume: 40 });
      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicVolume: 40 });
    });

    it("reverts a failed volume write to the level it replaced", async () => {
      mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));

      await persistAudioPreferences({ musicVolume: 30 });

      expect(volumeState()).toEqual({ sound: 70, music: 70 });
    });

    it("leaves a volume alone when a later write of it superseded the failure", async () => {
      const first = deferred();
      mockUpdatePreferences.mockReturnValueOnce(first.promise);

      const firstWrite = persistAudioPreferences({ musicVolume: 30 });
      await persistAudioPreferences({ musicVolume: 55 });

      first.reject(new Error("late failure"));
      await firstWrite;

      expect(volumeState().music).toBe(55);
    });

    it("keeps switches and volumes independent under latest-wins", async () => {
      // A mute that fails late must roll back the switches only; the volume
      // change made after it has its own field and its own outcome.
      const mute = deferred();
      mockUpdatePreferences.mockReturnValueOnce(mute.promise).mockResolvedValueOnce({});

      const muteWrite = persistAudioPreferences({ soundEnabled: false, musicEnabled: false });
      await persistAudioPreferences({ soundVolume: 20 });

      mute.reject(new Error("late failure"));
      await muteWrite;

      expect(audioState()).toEqual({ sound: true, music: true });
      expect(volumeState()).toEqual({ sound: 20, music: 70 });
    });

    it("never touches the volumes when only the switches move", async () => {
      useAuthStore.setState({ user: makeUser({ id: 7, soundVolume: 30, musicVolume: 80 }) });

      await persistAudioPreferences({ soundEnabled: false, musicEnabled: false });
      await persistAudioPreferences({ soundEnabled: true, musicEnabled: true });

      expect(volumeState()).toEqual({ sound: 30, music: 80 });
      for (const [, body] of mockUpdatePreferences.mock.calls) {
        expect(body).not.toHaveProperty("soundVolume");
        expect(body).not.toHaveProperty("musicVolume");
      }
    });
  });
});
