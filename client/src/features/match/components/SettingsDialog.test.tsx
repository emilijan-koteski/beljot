import "@/shared/i18n/i18n";

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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
