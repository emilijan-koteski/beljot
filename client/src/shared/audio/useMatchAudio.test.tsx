import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { useMatchAudio } from "./useMatchAudio";

const engine = vi.hoisted(() => ({
  release: vi.fn(),
  openAudioSession: vi.fn(),
  preloadSfx: vi.fn(() => Promise.resolve()),
  startMusic: vi.fn(),
  stopMusic: vi.fn(),
  setMusicVolume: vi.fn(),
}));

vi.mock("./audioEngine", () => ({
  openAudioSession: engine.openAudioSession,
  preloadSfx: engine.preloadSfx,
  startMusic: engine.startMusic,
  stopMusic: engine.stopMusic,
  setMusicVolume: engine.setMusicVolume,
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

function setMusic(musicEnabled: boolean | undefined) {
  useAuthStore.setState({
    token: "t",
    user: { ...makeUser(), musicEnabled: musicEnabled as boolean },
    isLoading: false,
  });
}

function setSound(soundEnabled: boolean | undefined) {
  const user = useAuthStore.getState().user ?? makeUser();
  useAuthStore.setState({ user: { ...user, soundEnabled: soundEnabled as boolean } });
}

function setMusicLevel(musicVolume: number | undefined) {
  const user = useAuthStore.getState().user ?? makeUser();
  useAuthStore.setState({ user: { ...user, musicVolume: musicVolume as number } });
}

describe("useMatchAudio", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    engine.openAudioSession.mockReturnValue(engine.release);
    setMusic(true);
  });

  it("opens an audio session and preloads the effects on mount", () => {
    renderHook(() => useMatchAudio());

    expect(engine.openAudioSession).toHaveBeenCalledTimes(1);
    expect(engine.preloadSfx).toHaveBeenCalledTimes(1);
  });

  it("does not preload (and so create a context) while sound is off", () => {
    setSound(false);
    renderHook(() => useMatchAudio());

    expect(engine.openAudioSession).toHaveBeenCalledTimes(1);
    expect(engine.preloadSfx).not.toHaveBeenCalled();
  });

  it("preloads the moment sound is switched on mid-match", () => {
    setSound(false);
    renderHook(() => useMatchAudio());

    act(() => setSound(true));
    expect(engine.preloadSfx).toHaveBeenCalledTimes(1);

    act(() => setSound(false));
    expect(engine.preloadSfx).toHaveBeenCalledTimes(1);
  });

  it("treats a missing sound switch as ON and preloads", () => {
    setSound(undefined);
    renderHook(() => useMatchAudio());

    expect(engine.preloadSfx).toHaveBeenCalledTimes(1);
  });

  it("starts the music on entry when music is on (the default)", () => {
    setMusic(undefined);
    renderHook(() => useMatchAudio());

    expect(engine.startMusic).toHaveBeenCalledTimes(1);
    expect(engine.stopMusic).not.toHaveBeenCalled();
  });

  it("never starts the music when it is off", () => {
    setMusic(false);
    renderHook(() => useMatchAudio());

    expect(engine.startMusic).not.toHaveBeenCalled();
  });

  it("follows a mid-match toggle immediately", () => {
    renderHook(() => useMatchAudio());
    expect(engine.startMusic).toHaveBeenCalledTimes(1);

    act(() => setMusic(false));
    expect(engine.stopMusic).toHaveBeenCalledTimes(1);

    act(() => setMusic(true));
    expect(engine.startMusic).toHaveBeenCalledTimes(2);
  });

  it("keeps the music playing when only sound effects are switched off", () => {
    renderHook(() => useMatchAudio());
    expect(engine.startMusic).toHaveBeenCalledTimes(1);

    act(() => {
      const user = useAuthStore.getState().user!;
      useAuthStore.getState().setUser({ ...user, soundEnabled: false });
    });

    expect(engine.stopMusic).not.toHaveBeenCalled();
    expect(engine.startMusic).toHaveBeenCalledTimes(1);
  });

  it("stops the music and closes the session when the page unmounts", () => {
    const { unmount } = renderHook(() => useMatchAudio());

    unmount();

    expect(engine.stopMusic).toHaveBeenCalledTimes(1);
    expect(engine.release).toHaveBeenCalledTimes(1);
  });

  it("puts the stored music volume in place before the music starts", () => {
    setMusicLevel(30);
    renderHook(() => useMatchAudio());

    expect(engine.setMusicVolume).toHaveBeenCalledWith(30);
    expect(engine.setMusicVolume.mock.invocationCallOrder[0]).toBeLessThan(
      engine.startMusic.mock.invocationCallOrder[0]!,
    );
  });

  it("treats a missing music volume as the default of 70", () => {
    setMusicLevel(undefined);
    renderHook(() => useMatchAudio());

    expect(engine.setMusicVolume).toHaveBeenCalledWith(70);
  });

  it("follows the stored music volume — a commit or a revert — without restarting the music", () => {
    renderHook(() => useMatchAudio());
    engine.setMusicVolume.mockClear();

    act(() => setMusicLevel(30));
    expect(engine.setMusicVolume).toHaveBeenLastCalledWith(30);

    act(() => setMusicLevel(70));
    expect(engine.setMusicVolume).toHaveBeenLastCalledWith(70);

    expect(engine.startMusic).toHaveBeenCalledTimes(1);
    expect(engine.stopMusic).not.toHaveBeenCalled();
  });

  it("never starts the music at volume 0, even with the switch on", () => {
    setMusicLevel(0);
    renderHook(() => useMatchAudio());

    expect(engine.startMusic).not.toHaveBeenCalled();
  });

  it("stops the music at volume 0 and starts it again above 0", () => {
    renderHook(() => useMatchAudio());
    expect(engine.startMusic).toHaveBeenCalledTimes(1);

    act(() => setMusicLevel(0));
    expect(engine.stopMusic).toHaveBeenCalledTimes(1);

    act(() => setMusicLevel(30));
    expect(engine.startMusic).toHaveBeenCalledTimes(2);

    // Between two non-zero levels the playlist just carries on.
    act(() => setMusicLevel(50));
    expect(engine.startMusic).toHaveBeenCalledTimes(2);
    expect(engine.stopMusic).toHaveBeenCalledTimes(1);
  });
});
