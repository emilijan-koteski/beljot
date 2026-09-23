import "@/shared/i18n/i18n";

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { resetAudioPreferenceRequestSequence } from "@/shared/lib/audioPreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { AudioPanel } from "./AudioPanel";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

/**
 * The profile-sidebar entry point for the two audio switches, so they are
 * settable outside a match. Same optimistic write + silent revert as the
 * in-match Settings dialog, sharing its latest-wins helper.
 */
describe("AudioPanel", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({});
    resetAudioPreferenceRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, soundEnabled: true, musicEnabled: false }),
      isLoading: false,
    });
  });

  it("shows each switch in the stored state, labelled by its row", () => {
    render(<AudioPanel />);

    expect(screen.getByTestId("profile-audio")).toHaveTextContent("Sound & music");
    const sound = screen.getByRole("switch", { name: "Sound effects" });
    const music = screen.getByRole("switch", { name: "Music" });
    expect(sound).toBe(screen.getByTestId("profile-sound-toggle"));
    expect(music).toBe(screen.getByTestId("profile-music-toggle"));
    expect(sound).toHaveAttribute("aria-checked", "true");
    expect(music).toHaveAttribute("aria-checked", "false");
  });

  it("reads missing switches as ON", () => {
    useAuthStore.setState({
      user: {
        ...makeUser({ id: 7 }),
        soundEnabled: undefined as never,
        musicEnabled: undefined as never,
      },
    });
    render(<AudioPanel />);

    expect(screen.getByTestId("profile-sound-toggle")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("profile-music-toggle")).toHaveAttribute("aria-checked", "true");
  });

  it("writes a switch optimistically and PATCHes only its own field", async () => {
    const user = userEvent.setup();
    render(<AudioPanel />);

    await user.click(screen.getByTestId("profile-music-toggle"));

    expect(useAuthStore.getState().user?.musicEnabled).toBe(true);
    expect(useAuthStore.getState().user?.soundEnabled).toBe(true);
    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicEnabled: true });
    expect(screen.getByTestId("profile-music-toggle")).toHaveAttribute("aria-checked", "true");
  });

  it("toggles from its row label too", async () => {
    const user = userEvent.setup();
    render(<AudioPanel />);

    await user.click(screen.getByText("Sound effects"));

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundEnabled: false });
    expect(useAuthStore.getState().user?.soundEnabled).toBe(false);
  });

  it("reverts the switch when the PATCH fails", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<AudioPanel />);

    await user.click(screen.getByTestId("profile-sound-toggle"));

    await waitFor(() => {
      expect(useAuthStore.getState().user?.soundEnabled).toBe(true);
    });
    expect(screen.getByTestId("profile-sound-toggle")).toHaveAttribute("aria-checked", "true");
  });
});

/**
 * A real pointer drag on a slider's control. jsdom has no layout and no pointer
 * capture, so the control gets a 100 px-wide box (1 px per volume point) and
 * no-op capture methods.
 */
function pointerDrag(testId: string) {
  const control = screen
    .getByTestId(testId)
    .querySelector<HTMLElement>('[data-slot="slider-control"]');
  if (!control) throw new Error(`no slider control in ${testId}`);
  control.getBoundingClientRect = () =>
    ({ left: 0, width: 100, right: 100, top: 0, bottom: 20, height: 20, x: 0, y: 0 }) as DOMRect;
  control.setPointerCapture = () => {};
  control.releasePointerCapture = () => {};
  control.hasPointerCapture = () => false;
  return {
    down: (clientX: number) => fireEvent.pointerDown(control, { button: 0, clientX }),
    move: (clientX: number) => fireEvent.pointerMove(document, { clientX, buttons: 1 }),
    up: (clientX: number) => fireEvent.pointerUp(document, { clientX }),
  };
}

describe("AudioPanel volume sliders", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({});
    resetAudioPreferenceRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        id: 7,
        soundEnabled: true,
        musicEnabled: false,
        soundVolume: 45,
        musicVolume: 80,
      }),
      isLoading: false,
    });
  });

  function slider(testId: string): HTMLInputElement {
    return within(screen.getByTestId(testId)).getByRole("slider") as HTMLInputElement;
  }

  it("puts a named slider under each switch, showing the stored level", () => {
    render(<AudioPanel />);

    const sound = screen.getByRole("slider", { name: "Sound effects volume" });
    const music = screen.getByRole("slider", { name: "Music volume" });
    expect(sound).toBe(slider("profile-sound-volume"));
    expect(music).toBe(slider("profile-music-volume"));
    expect(sound).toHaveAttribute("aria-valuenow", "45");
    expect(music).toHaveAttribute("aria-valuenow", "80");
    expect(screen.getByTestId("profile-sound-volume")).toHaveTextContent("45%");
    expect(screen.getByTestId("profile-music-volume")).toHaveTextContent("80%");
  });

  it("reads missing volumes as 70", () => {
    useAuthStore.setState({
      user: {
        ...makeUser({ id: 7 }),
        soundVolume: undefined as never,
        musicVolume: undefined as never,
      },
    });
    render(<AudioPanel />);

    expect(slider("profile-sound-volume")).toHaveAttribute("aria-valuenow", "70");
    expect(slider("profile-music-volume")).toHaveAttribute("aria-valuenow", "70");
  });

  it("disables the slider of a switched-off channel, value kept", () => {
    render(<AudioPanel />);

    expect(slider("profile-music-volume")).toBeDisabled();
    expect(slider("profile-music-volume")).toHaveAttribute("aria-valuenow", "80");
    expect(slider("profile-sound-volume")).not.toBeDisabled();
  });

  it("re-enables the slider on the kept value when its switch comes back on", async () => {
    const user = userEvent.setup();
    render(<AudioPanel />);

    await user.click(screen.getByTestId("profile-music-toggle"));

    expect(slider("profile-music-volume")).not.toBeDisabled();
    expect(slider("profile-music-volume")).toHaveAttribute("aria-valuenow", "80");
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicEnabled: true });
  });

  it("persists a committed level as ONE PATCH of that field only", () => {
    render(<AudioPanel />);

    fireEvent.change(slider("profile-sound-volume"), { target: { value: "20" } });

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundVolume: 20 });
    expect(useAuthStore.getState().user?.soundVolume).toBe(20);
    expect(screen.getByTestId("profile-sound-volume")).toHaveTextContent("20%");
  });

  it("keeps the slider outside the switch's label, so using it never toggles the switch", () => {
    render(<AudioPanel />);

    // Inside the <label>, a press on the track would reach the switch too.
    expect(screen.getByTestId("profile-sound-volume").closest("label")).toBeNull();
    expect(screen.getByTestId("profile-music-volume").closest("label")).toBeNull();

    fireEvent.keyDown(slider("profile-sound-volume"), { key: "ArrowRight" });

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundVolume: 46 });
    expect(screen.getByTestId("profile-sound-toggle")).toHaveAttribute("aria-checked", "true");
  });

  it("follows a pointer drag live and saves ONE PATCH on release", () => {
    useAuthStore.setState({
      user: makeUser({ id: 7, musicEnabled: true, soundVolume: 45, musicVolume: 80 }),
    });
    render(<AudioPanel />);
    const drag = pointerDrag("profile-music-volume");

    drag.down(40);
    drag.move(25);

    expect(slider("profile-music-volume")).toHaveAttribute("aria-valuenow", "25");
    expect(screen.getByTestId("profile-music-volume")).toHaveTextContent("25%");
    expect(mockUpdatePreferences).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user?.musicVolume).toBe(80);

    drag.up(25);

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicVolume: 25 });
    expect(useAuthStore.getState().user?.musicVolume).toBe(25);
    expect(screen.getByTestId("profile-music-volume")).toHaveTextContent("25%");
  });

  it("reverts the slider when the PATCH fails", async () => {
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<AudioPanel />);

    fireEvent.change(slider("profile-sound-volume"), { target: { value: "90" } });

    await waitFor(() => {
      expect(useAuthStore.getState().user?.soundVolume).toBe(45);
    });
    expect(slider("profile-sound-volume")).toHaveAttribute("aria-valuenow", "45");
  });
});
