import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { useGuestThemeStore } from "@/shared/stores/themeStore";

import { GoogleSignInButton } from "./GoogleSignInButton";

const mockRenderGoogleButton = vi.fn();
vi.mock("@/shared/lib/googleIdentity", () => ({
  getGoogleClientId: () => "test-client-id",
  renderGoogleButton: (...args: unknown[]) => mockRenderGoogleButton(...args),
}));

beforeEach(() => {
  mockRenderGoogleButton.mockReset().mockResolvedValue(true);
  useAuthStore.setState({ token: null, user: null, isLoading: false });
});

describe("GoogleSignInButton", () => {
  it("asks Google for the outline button in parchment", async () => {
    useGuestThemeStore.setState({ theme: "parchment" });
    render(<GoogleSignInButton onCredential={vi.fn()} />);

    await waitFor(() =>
      expect(mockRenderGoogleButton).toHaveBeenCalledWith(
        expect.objectContaining({ theme: "outline" }),
      ),
    );
  });

  it("switches to Google's dark button when brass mode turns on", async () => {
    useGuestThemeStore.setState({ theme: "parchment" });
    render(<GoogleSignInButton onCredential={vi.fn()} />);

    act(() => useGuestThemeStore.setState({ theme: "brass" }));

    await waitFor(() =>
      expect(mockRenderGoogleButton).toHaveBeenLastCalledWith(
        expect.objectContaining({ theme: "filled_black" }),
      ),
    );
  });

  it("keeps the slot on a light colour scheme so no box shows behind the iframe", () => {
    useGuestThemeStore.setState({ theme: "brass" });
    render(<GoogleSignInButton onCredential={vi.fn()} />);

    expect(screen.getByTestId("google-signin-button")).toHaveStyle({ colorScheme: "light" });
  });
});
