import "@/shared/i18n/i18n";

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetAudioPreferenceRequestSequence } from "@/shared/lib/audioPreference";
import { resetCardDeckRequestSequence } from "@/shared/lib/cardDeckPreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { SettingsDialog } from "./SettingsDialog";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

describe("SettingsDialog card deck section", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({ cardDeckPreference: "croatian" });
    resetCardDeckRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, cardDeckPreference: "french" }),
      isLoading: false,
    });
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("en");
  });

  it("marks the player's current deck as the selected radio", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    expect(screen.getByTestId("settings-deck-option-french")).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByTestId("settings-deck-option-croatian")).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("writes the deck optimistically and PATCHes only the deck field", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-deck-option-croatian"));

    // The store write is what re-skins the table — it must not wait on the PATCH.
    expect(useAuthStore.getState().user?.cardDeckPreference).toBe("croatian");
    // Deck-only body: resending the language would make a deck change able to
    // clobber a language chosen in another tab.
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { cardDeckPreference: "croatian" });
  });

  it("reverts the store deck when the PATCH fails", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-deck-option-croatian"));

    // Silent revert, matching LanguageSelector: the rendered deck follows the
    // store, so the cards revert with it and nothing interrupts play.
    await waitFor(() => {
      expect(useAuthStore.getState().user?.cardDeckPreference).toBe("french");
    });
    expect(screen.getByTestId("settings-deck-option-french")).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("does not PATCH when the already-selected deck is clicked again", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-deck-option-french"));

    expect(mockUpdatePreferences).not.toHaveBeenCalled();
  });

  it("keeps the language section alongside the deck section", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    expect(screen.getByTestId("settings-language-option-en")).toBeInTheDocument();
    expect(screen.getByTestId("settings-language-option-hr")).toBeInTheDocument();
  });

  it("keeps the language and deck rows in radiogroups named by their headings", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    // `radiogroup` is now SettingSection's default rather than hardcoded, so
    // pin that the two exclusive pickers still get it.
    const language = screen.getByTestId("settings-language-option-en").parentElement;
    const deck = screen.getByTestId("settings-deck-option-french").parentElement;
    expect(language).toHaveAttribute("role", "radiogroup");
    expect(deck).toHaveAttribute("role", "radiogroup");
    expect(screen.getByRole("radiogroup", { name: "Language" })).toBe(language);
    expect(screen.getByRole("radiogroup", { name: "Card deck" })).toBe(deck);
  });

  it("persists a language choice and marks the row selected", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open onOpenChange={() => {}} />);

    expect(screen.getByTestId("settings-language-option-en")).toHaveAttribute(
      "aria-checked",
      "true",
    );

    await user.click(screen.getByTestId("settings-language-option-hr"));

    // Language-only body: the shared SettingRow must not have quietly rewired
    // the language path onto the deck field, and a language change must not
    // resend (and therefore be able to clobber) the deck.
    await waitFor(() => {
      expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { languagePreference: "hr" });
    });
    expect(useAuthStore.getState().user?.languagePreference).toBe("hr");
    expect(useAuthStore.getState().user?.cardDeckPreference).toBe("french");

    await waitFor(() => {
      expect(screen.getByTestId("settings-language-option-hr")).toHaveAttribute(
        "aria-checked",
        "true",
      );
    });
    expect(screen.getByTestId("settings-language-option-en")).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("reverts the store language when its PATCH fails", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-language-option-sr"));

    await waitFor(() => {
      expect(useAuthStore.getState().user?.languagePreference).toBe("en");
    });
  });

  it("ignores a superseded deck failure — the last toggle wins", async () => {
    const user = userEvent.setup();
    // First request fails LATE, second succeeds. Without the sequence guard the
    // stale rollback would reinstate "french" over the player's real choice.
    let releaseFirst: () => void = () => {};
    mockUpdatePreferences
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            releaseFirst = () => reject(new Error("late failure"));
          }),
      )
      .mockResolvedValueOnce({ cardDeckPreference: "french" });

    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-deck-option-croatian"));
    await user.click(screen.getByTestId("settings-deck-option-french"));
    expect(useAuthStore.getState().user?.cardDeckPreference).toBe("french");

    releaseFirst();
    await waitFor(() => {
      expect(mockUpdatePreferences).toHaveBeenCalledTimes(2);
    });

    // Still the second toggle's value, not the first request's rollback target.
    expect(useAuthStore.getState().user?.cardDeckPreference).toBe("french");
  });
});

describe("SettingsDialog sound section", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({});
    resetAudioPreferenceRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, soundEnabled: true, musicEnabled: true }),
      isLoading: false,
    });
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("en");
  });

  it("renders both audio switches, on by default, in a non-radio group", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    const sound = screen.getByTestId("settings-sound-toggle");
    const music = screen.getByTestId("settings-music-toggle");
    expect(sound).toHaveAttribute("role", "switch");
    expect(sound).toHaveAttribute("aria-checked", "true");
    expect(music).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("switch", { name: "Sound effects" })).toBe(sound);
    expect(screen.getByRole("switch", { name: "Music" })).toBe(music);
    // Independent switches: their group must not claim radio semantics.
    expect(sound.parentElement).toHaveAttribute("role", "group");
    expect(screen.getByRole("group", { name: "Sound" })).toBe(sound.parentElement);
  });

  it("reads a missing switch as ON", () => {
    useAuthStore.setState({
      user: { ...makeUser({ id: 7 }), musicEnabled: undefined as never },
    });
    render(<SettingsDialog open onOpenChange={() => {}} />);

    expect(screen.getByTestId("settings-music-toggle")).toHaveAttribute("aria-checked", "true");
  });

  it("turns sound effects off optimistically and PATCHes only that field", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-sound-toggle"));

    expect(useAuthStore.getState().user?.soundEnabled).toBe(false);
    expect(useAuthStore.getState().user?.musicEnabled).toBe(true);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundEnabled: false });
    expect(screen.getByTestId("settings-sound-toggle")).toHaveAttribute("aria-checked", "false");
  });

  it("toggles music independently of sound, both ways", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-music-toggle"));
    expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { musicEnabled: false });
    expect(useAuthStore.getState().user?.soundEnabled).toBe(true);

    await user.click(screen.getByTestId("settings-music-toggle"));
    expect(mockUpdatePreferences).toHaveBeenLastCalledWith(7, { musicEnabled: true });
    expect(screen.getByTestId("settings-music-toggle")).toHaveAttribute("aria-checked", "true");
  });

  it("reverts the switch silently when its PATCH fails", async () => {
    const user = userEvent.setup();
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-sound-toggle"));

    await waitFor(() => {
      expect(screen.getByTestId("settings-sound-toggle")).toHaveAttribute("aria-checked", "true");
    });
    expect(useAuthStore.getState().user?.soundEnabled).toBe(true);
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

describe("SettingsDialog volume sliders", () => {
  beforeEach(() => {
    mockUpdatePreferences.mockReset();
    mockUpdatePreferences.mockResolvedValue({});
    resetAudioPreferenceRequestSequence();
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, soundVolume: 70, musicVolume: 30 }),
      isLoading: false,
    });
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("en");
  });

  function slider(testId: string): HTMLInputElement {
    return within(screen.getByTestId(testId)).getByRole("slider") as HTMLInputElement;
  }

  it("puts a named slider under each switch, showing the stored level", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    const sound = screen.getByRole("slider", { name: "Sound effects volume" });
    const music = screen.getByRole("slider", { name: "Music volume" });
    expect(sound).toBe(slider("settings-sound-volume"));
    expect(music).toBe(slider("settings-music-volume"));
    expect(sound).toHaveAttribute("aria-valuenow", "70");
    expect(music).toHaveAttribute("aria-valuenow", "30");
    expect(music).toHaveAttribute("aria-valuetext", "30%");
    expect(screen.getByTestId("settings-sound-volume")).toHaveTextContent("70%");
    expect(screen.getByTestId("settings-music-volume")).toHaveTextContent("30%");

    // Each slider follows its own switch in the Sound group.
    const group = screen.getByRole("group", { name: "Sound" });
    const order = Array.from(group.children).map((el) => el.getAttribute("data-testid"));
    expect(order).toEqual([
      "settings-sound-toggle",
      "settings-sound-volume",
      "settings-music-toggle",
      "settings-music-volume",
    ]);
  });

  it("reads a missing volume as 70", () => {
    useAuthStore.setState({
      user: { ...makeUser({ id: 7 }), soundVolume: undefined as never },
    });
    render(<SettingsDialog open onOpenChange={() => {}} />);

    expect(slider("settings-sound-volume")).toHaveAttribute("aria-valuenow", "70");
    expect(screen.getByTestId("settings-sound-volume")).toHaveTextContent("70%");
  });

  it("persists a committed level as ONE PATCH of that field only", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    fireEvent.change(slider("settings-music-volume"), { target: { value: "55" } });

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicVolume: 55 });
    expect(useAuthStore.getState().user?.musicVolume).toBe(55);
    expect(useAuthStore.getState().user?.soundVolume).toBe(70);
    expect(screen.getByTestId("settings-music-volume")).toHaveTextContent("55%");
  });

  it("follows a pointer drag live and saves ONE PATCH on release", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);
    const drag = pointerDrag("settings-music-volume");

    drag.down(40);
    drag.move(25);

    expect(slider("settings-music-volume")).toHaveAttribute("aria-valuenow", "25");
    expect(screen.getByTestId("settings-music-volume")).toHaveTextContent("25%");
    expect(mockUpdatePreferences).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user?.musicVolume).toBe(30);

    drag.up(25);

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { musicVolume: 25 });
    expect(useAuthStore.getState().user?.musicVolume).toBe(25);
    expect(screen.getByTestId("settings-music-volume")).toHaveTextContent("25%");
  });

  it("steps with the keyboard", () => {
    render(<SettingsDialog open onOpenChange={() => {}} />);

    fireEvent.keyDown(slider("settings-sound-volume"), { key: "ArrowLeft" });

    expect(mockUpdatePreferences).toHaveBeenCalledTimes(1);
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { soundVolume: 69 });
    expect(slider("settings-sound-volume")).toHaveAttribute("aria-valuenow", "69");
  });

  it("disables a slider while its switch is off, keeping the value for when it returns", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open onOpenChange={() => {}} />);

    await user.click(screen.getByTestId("settings-music-toggle"));

    expect(slider("settings-music-volume")).toBeDisabled();
    expect(slider("settings-music-volume")).toHaveAttribute("aria-valuenow", "30");
    expect(screen.getByTestId("settings-music-volume")).toHaveTextContent("30%");
    expect(slider("settings-sound-volume")).not.toBeDisabled();

    await user.click(screen.getByTestId("settings-music-toggle"));

    expect(slider("settings-music-volume")).not.toBeDisabled();
    expect(slider("settings-music-volume")).toHaveAttribute("aria-valuenow", "30");
    // The switch PATCHes carried the switch alone, never the volume.
    for (const [, body] of mockUpdatePreferences.mock.calls) {
      expect(body).not.toHaveProperty("musicVolume");
    }
  });

  it("reverts the slider silently when its PATCH fails", async () => {
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));
    render(<SettingsDialog open onOpenChange={() => {}} />);

    fireEvent.change(slider("settings-music-volume"), { target: { value: "80" } });

    await waitFor(() => {
      expect(slider("settings-music-volume")).toHaveAttribute("aria-valuenow", "30");
    });
    expect(useAuthStore.getState().user?.musicVolume).toBe(30);
    expect(screen.getByTestId("settings-music-volume")).toHaveTextContent("30%");
  });
});
