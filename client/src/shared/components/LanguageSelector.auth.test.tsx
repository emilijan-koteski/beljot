import "@/shared/i18n/i18n";

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { i18n, LANGUAGE_STORAGE_KEY } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";

import { LanguageSelector } from "./LanguageSelector";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

function renderAuthMode() {
  return render(<LanguageSelector testIdPrefix="auth-language" />);
}

describe("LanguageSelector (auth-prefixed, no server persistence)", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    window.localStorage.clear();
    await i18n.changeLanguage("en");
    window.localStorage.clear();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("en");
  });

  it("renders the current language in the trigger", () => {
    renderAuthMode();
    expect(screen.getByTestId("auth-language-selector")).toHaveTextContent("EN");
  });

  it("renders all four language options in order en/hr/sr/mk", async () => {
    const user = userEvent.setup();
    renderAuthMode();

    await user.click(screen.getByTestId("auth-language-selector"));

    await waitFor(() => {
      expect(screen.getByTestId("auth-language-option-en")).toBeInTheDocument();
    });
    const items = screen.getAllByTestId(/^auth-language-option-/);
    expect(items.map((el) => el.getAttribute("data-testid"))).toEqual([
      "auth-language-option-en",
      "auth-language-option-hr",
      "auth-language-option-sr",
      "auth-language-option-mk",
    ]);
  });

  it("changes i18n.language on selection without calling the preferences API", async () => {
    const user = userEvent.setup();
    renderAuthMode();

    await user.click(screen.getByTestId("auth-language-selector"));
    await waitFor(() => {
      expect(screen.getByTestId("auth-language-option-mk")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("auth-language-option-mk"));

    await waitFor(() => {
      expect(i18n.language).toBe("mk");
    });
    expect(mockUpdatePreferences).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user).toBeNull();
  });

  it("persists the picked language to localStorage via the i18n listener", async () => {
    const user = userEvent.setup();
    renderAuthMode();

    await user.click(screen.getByTestId("auth-language-selector"));
    await waitFor(() => {
      expect(screen.getByTestId("auth-language-option-hr")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("auth-language-option-hr"));

    await waitFor(() => {
      expect(window.localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("hr");
    });
  });
});
