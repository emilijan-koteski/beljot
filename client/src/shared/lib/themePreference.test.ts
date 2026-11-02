import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import {
  applyTheme,
  BRASS_MODE_CLASS,
  persistTheme,
  resetThemeRequestSequence,
  THEME_STORAGE_KEY,
} from "./themePreference";

const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
}));

beforeEach(() => {
  resetThemeRequestSequence();
  mockUpdatePreferences.mockReset().mockResolvedValue({});
  useAuthStore.setState({ token: "t", user: makeUser({ id: 3 }), isLoading: false });
});

afterEach(() => {
  document.documentElement.classList.remove(BRASS_MODE_CLASS);
  window.localStorage.clear();
});

describe("applyTheme", () => {
  it("marks <html> and remembers brass mode for the next load", () => {
    applyTheme("brass");

    expect(document.documentElement).toHaveClass(BRASS_MODE_CLASS);
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("brass");
  });

  it("clears both the class and the stored mirror for parchment", () => {
    applyTheme("brass");
    applyTheme("parchment");

    expect(document.documentElement).not.toHaveClass(BRASS_MODE_CLASS);
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();
  });
});

describe("persistTheme", () => {
  it("writes the theme to the store before the server answers", async () => {
    const pending = persistTheme("brass", "parchment");

    expect(useAuthStore.getState().user?.themePreference).toBe("brass");
    await pending;
    expect(mockUpdatePreferences).toHaveBeenCalledWith(3, { themePreference: "brass" });
  });

  it("restores the previous theme when the save fails", async () => {
    mockUpdatePreferences.mockRejectedValueOnce(new Error("offline"));

    await persistTheme("brass", "parchment");

    expect(useAuthStore.getState().user?.themePreference).toBe("parchment");
  });

  it("keeps a later toggle when an earlier save fails after it", async () => {
    let failFirst: (reason: Error) => void = () => {};
    mockUpdatePreferences.mockImplementationOnce(
      () => new Promise((_, reject) => (failFirst = reject)),
    );

    const first = persistTheme("brass", "parchment");
    await persistTheme("parchment", "brass");
    failFirst(new Error("late"));
    await first;

    expect(useAuthStore.getState().user?.themePreference).toBe("parchment");
  });

  it("does nothing while signed out", async () => {
    useAuthStore.setState({ user: null });

    await persistTheme("brass", "parchment");

    expect(mockUpdatePreferences).not.toHaveBeenCalled();
  });
});
