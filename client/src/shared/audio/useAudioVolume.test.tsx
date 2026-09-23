import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetAudioPreferenceRequestSequence } from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { COMMIT_COALESCE_MS, useAudioVolume } from "./useAudioVolume";

const engine = vi.hoisted(() => ({
  playSfx: vi.fn(),
  setMusicVolume: vi.fn(),
}));

vi.mock("./audioEngine", () => ({
  playSfx: engine.playSfx,
  setMusicVolume: engine.setMusicVolume,
}));

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

function storedVolumes() {
  const user = useAuthStore.getState().user;
  return { sound: user?.soundVolume, music: user?.musicVolume };
}

describe("useAudioVolume", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdatePreferences.mockResolvedValue({});
    resetAudioPreferenceRequestSequence();
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 7, soundVolume: 70, musicVolume: 70 }),
      isLoading: false,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows the stored volume, and 70 when it is missing", () => {
    useAuthStore.setState({
      user: { ...makeUser({ id: 7, soundVolume: 30 }), musicVolume: undefined as never },
    });

    expect(renderHook(() => useAudioVolume("soundVolume")).result.current.value).toBe(30);
    expect(renderHook(() => useAudioVolume("musicVolume")).result.current.value).toBe(70);
  });

  describe("music", () => {
    it("drives the engine live while dragging, without touching the store or the server", () => {
      const { result } = renderHook(() => useAudioVolume("musicVolume"));

      act(() => result.current.onValueChange(50));
      act(() => result.current.onValueChange(30));

      expect(result.current.value).toBe(30);
      expect(engine.setMusicVolume).toHaveBeenLastCalledWith(30);
      expect(storedVolumes().music).toBe(70);
      expect(mockUpdatePreferences).not.toHaveBeenCalled();
      expect(engine.playSfx).not.toHaveBeenCalled();
    });

    it("persists ONE PATCH on release, optimistically", () => {
      const { result } = renderHook(() => useAudioVolume("musicVolume"));

      act(() => result.current.onValueChange(40));
      act(() => result.current.onValueChange(30));
      act(() => result.current.onValueCommitted(30));

      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicVolume: 30 });
      expect(storedVolumes()).toEqual({ sound: 70, music: 30 });
      expect(result.current.value).toBe(30);
      expect(engine.playSfx).not.toHaveBeenCalled();
    });

    it("falls back to the previous level when the PATCH fails", async () => {
      mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
      const { result } = renderHook(() => useAudioVolume("musicVolume"));

      act(() => result.current.onValueChange(30));
      act(() => result.current.onValueCommitted(30));

      await waitFor(() => expect(storedVolumes().music).toBe(70));
      expect(result.current.value).toBe(70);
    });

    it("hands the music back to the stored level when unmounted mid-drag", () => {
      const { result, unmount } = renderHook(() => useAudioVolume("musicVolume"));

      act(() => result.current.onValueChange(20));
      engine.setMusicVolume.mockClear();
      unmount();

      expect(engine.setMusicVolume).toHaveBeenCalledTimes(1);
      expect(engine.setMusicVolume).toHaveBeenCalledWith(70);
      expect(mockUpdatePreferences).not.toHaveBeenCalled();
    });

    it("leaves the engine alone on an unmount after the drag was committed", () => {
      const { result, unmount } = renderHook(() => useAudioVolume("musicVolume"));

      act(() => result.current.onValueChange(20));
      act(() => result.current.onValueCommitted(20));
      engine.setMusicVolume.mockClear();
      unmount();

      expect(engine.setMusicVolume).not.toHaveBeenCalled();
    });
  });

  describe("sound effects", () => {
    it("stays quiet while dragging — the level only matters on release", () => {
      const { result } = renderHook(() => useAudioVolume("soundVolume"));

      act(() => result.current.onValueChange(40));

      expect(result.current.value).toBe(40);
      expect(engine.playSfx).not.toHaveBeenCalled();
      expect(engine.setMusicVolume).not.toHaveBeenCalled();
      expect(mockUpdatePreferences).not.toHaveBeenCalled();
    });

    it("persists on release and previews one card sound at the NEW level", () => {
      let levelAtPreview: number | undefined;
      engine.playSfx.mockImplementation(() => {
        levelAtPreview = useAuthStore.getState().user?.soundVolume;
      });
      const { result } = renderHook(() => useAudioVolume("soundVolume"));

      act(() => result.current.onValueChange(40));
      act(() => result.current.onValueCommitted(40));

      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundVolume: 40 });
      expect(engine.playSfx).toHaveBeenCalledTimes(1);
      expect(engine.playSfx).toHaveBeenCalledWith("cardPlay");
      // The engine reads the volume at play time, so the store must already
      // hold the new level when the preview fires.
      expect(levelAtPreview).toBe(40);
      expect(engine.setMusicVolume).not.toHaveBeenCalled();
    });
  });

  describe("stale commits", () => {
    it("ignores a commit that no value change preceded", () => {
      const { result } = renderHook(() => useAudioVolume("soundVolume"));

      // Base UI commits its last changed value when nothing moved — a thumb
      // click without dragging, or End at the maximum.
      act(() => result.current.onValueCommitted(30));

      expect(mockUpdatePreferences).not.toHaveBeenCalled();
      expect(engine.playSfx).not.toHaveBeenCalled();
      expect(storedVolumes().sound).toBe(70);
    });

    it("ignores a repeat commit after a gesture was already saved", async () => {
      vi.useFakeTimers();
      mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
      const { result } = renderHook(() => useAudioVolume("soundVolume"));

      act(() => result.current.onValueChange(40));
      act(() => result.current.onValueCommitted(40));
      // The failed save reverts the store to 70 ...
      await act(async () => {});
      expect(storedVolumes().sound).toBe(70);
      act(() => vi.advanceTimersByTime(COMMIT_COALESCE_MS));

      // ... and a later no-move commit reporting the stale 40 must not re-save
      // it (or replay the preview) — neither at once nor after the quiet period.
      act(() => result.current.onValueCommitted(40));
      act(() => vi.advanceTimersByTime(COMMIT_COALESCE_MS));

      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(engine.playSfx).toHaveBeenCalledTimes(1);
      expect(storedVolumes().sound).toBe(70);
    });
  });

  describe("held arrow key (commit coalescing)", () => {
    /** One auto-repeat: Base UI fires change then commit on every keydown. */
    function step(control: { current: ReturnType<typeof useAudioVolume> }, value: number) {
      act(() => control.current.onValueChange(value));
      act(() => control.current.onValueCommitted(value));
    }

    it("saves the first step at once and the last one when the key goes quiet", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useAudioVolume("soundVolume"));

      step(result, 71);
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { soundVolume: 71 });
      expect(engine.playSfx).toHaveBeenCalledTimes(1);

      for (const value of [72, 73, 74, 75]) {
        act(() => vi.advanceTimersByTime(30));
        step(result, value);
      }
      // Held back: the slider shows the latest step, nothing more is sent.
      expect(result.current.value).toBe(75);
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(engine.playSfx).toHaveBeenCalledTimes(1);

      act(() => vi.advanceTimersByTime(COMMIT_COALESCE_MS - 1));
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);

      act(() => vi.advanceTimersByTime(1));
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(2);
      expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { soundVolume: 75 });
      expect(engine.playSfx).toHaveBeenCalledTimes(2);
      expect(storedVolumes().sound).toBe(75);
      expect(result.current.value).toBe(75);
    });

    it("keeps held music live and saves it once when the key goes quiet", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useAudioVolume("musicVolume"));

      step(result, 69);
      for (const value of [68, 67, 66]) {
        act(() => vi.advanceTimersByTime(30));
        step(result, value);
        expect(engine.setMusicVolume).toHaveBeenLastCalledWith(value);
      }
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
      expect(storedVolumes().music).toBe(69);

      act(() => vi.advanceTimersByTime(COMMIT_COALESCE_MS));

      expect(mockUpdatePreferences).toHaveBeenCalledTimes(2);
      expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { musicVolume: 66 });
      expect(storedVolumes().music).toBe(66);
      expect(engine.playSfx).not.toHaveBeenCalled();
    });

    it("saves an isolated commit at once again after the quiet period", () => {
      vi.useFakeTimers();
      const { result } = renderHook(() => useAudioVolume("soundVolume"));

      step(result, 60);
      act(() => vi.advanceTimersByTime(COMMIT_COALESCE_MS));
      step(result, 50);

      expect(mockUpdatePreferences).toHaveBeenCalledTimes(2);
      expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { soundVolume: 50 });
      expect(engine.playSfx).toHaveBeenCalledTimes(2);
    });

    it("saves a held step on unmount, without a preview", () => {
      vi.useFakeTimers();
      const { result, unmount } = renderHook(() => useAudioVolume("soundVolume"));

      step(result, 71);
      act(() => vi.advanceTimersByTime(30));
      step(result, 72);
      unmount();

      expect(mockUpdatePreferences).toHaveBeenCalledTimes(2);
      expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { soundVolume: 72 });
      expect(engine.playSfx).toHaveBeenCalledTimes(1);

      act(() => vi.advanceTimersByTime(COMMIT_COALESCE_MS * 2));
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(2);
    });
  });
});
