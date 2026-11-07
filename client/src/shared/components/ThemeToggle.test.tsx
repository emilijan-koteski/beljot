import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  BRASS_MODE_CLASS,
  THEME_PICKED_KEY,
  THEME_STORAGE_KEY,
} from "@/shared/lib/themePreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { useGuestThemeStore } from "@/shared/stores/themeStore";
import { makeUser } from "@/test-utils";

import { ThemeToggle } from "./ThemeToggle";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

beforeEach(() => {
  mockUpdatePreferences.mockReset().mockResolvedValue({});
  useGuestThemeStore.setState({ theme: "parchment" });
});

afterEach(() => {
  document.documentElement.classList.remove(BRASS_MODE_CLASS);
  window.localStorage.clear();
  window.sessionStorage.clear();
});

describe("ThemeToggle", () => {
  it("switches a signed-out visitor to brass mode and remembers the pick", async () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    const user = userEvent.setup();
    render(<ThemeToggle />);

    const toggle = screen.getByRole("button", { name: "Brass mode" });
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    await user.click(toggle);

    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(document.documentElement).toHaveClass(BRASS_MODE_CLASS);
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("brass");
    expect(window.sessionStorage.getItem(THEME_PICKED_KEY)).toBe("brass");
    expect(mockUpdatePreferences).not.toHaveBeenCalled();
  });

  it("saves a signed-in player's choice to the account", async () => {
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 5, themePreference: "brass" }),
      isLoading: false,
    });
    const user = userEvent.setup();
    render(<ThemeToggle />);

    await user.click(screen.getByRole("button", { name: "Brass mode" }));

    expect(useAuthStore.getState().user?.themePreference).toBe("parchment");
    expect(mockUpdatePreferences).toHaveBeenCalledWith(5, { themePreference: "parchment" });
    expect(window.sessionStorage.getItem(THEME_PICKED_KEY)).toBeNull();
  });
});
