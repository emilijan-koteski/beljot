import { renderHook } from "@testing-library/react";
import { act } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { BRASS_MODE_CLASS, THEME_STORAGE_KEY } from "@/shared/lib/themePreference";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { useThemeSync } from "./useThemeSync";

afterEach(() => {
  document.documentElement.classList.remove(BRASS_MODE_CLASS);
  window.localStorage.clear();
});

describe("useThemeSync", () => {
  it("leaves the first-paint class alone while the session is still restoring", () => {
    document.documentElement.classList.add(BRASS_MODE_CLASS);
    useAuthStore.setState({ token: null, user: null, isLoading: true });

    renderHook(() => useThemeSync());

    expect(document.documentElement).toHaveClass(BRASS_MODE_CLASS);
  });

  it("follows the signed-in player's theme as it changes", () => {
    useAuthStore.setState({
      token: "t",
      user: makeUser({ themePreference: "brass" }),
      isLoading: false,
    });
    renderHook(() => useThemeSync());
    expect(document.documentElement).toHaveClass(BRASS_MODE_CLASS);

    act(() => useAuthStore.setState({ user: makeUser({ themePreference: "parchment" }) }));

    expect(document.documentElement).not.toHaveClass(BRASS_MODE_CLASS);
  });

  it("drops brass mode and its stored mirror once the player signs out", () => {
    useAuthStore.setState({
      token: "t",
      user: makeUser({ themePreference: "brass" }),
      isLoading: false,
    });
    renderHook(() => useThemeSync());
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("brass");

    act(() => useAuthStore.setState({ token: null, user: null }));

    expect(document.documentElement).not.toHaveClass(BRASS_MODE_CLASS);
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();
  });
});
