import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { ProfileResponse } from "@/shared/api/profile";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser, mockViewportWidth, QueryWrapper } from "@/test-utils";

import { SettingsPage } from "./SettingsPage";

const mockForgotPassword = vi.fn();
const mockLogout = vi.fn();
vi.mock("@/shared/api/auth", () => ({
  forgotPassword: (...args: unknown[]) => mockForgotPassword(...args),
  logout: (...args: unknown[]) => mockLogout(...args),
}));

const mockGetProfile = vi.fn();
const mockUpdatePreferences = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  getProfile: (...args: unknown[]) => mockGetProfile(...args),
  updatePreferences: (...args: unknown[]) => mockUpdatePreferences(...args),
  updateUsername: vi.fn(),
  uploadAvatar: vi.fn(),
  removeAvatar: vi.fn(),
}));

vi.mock("@/shared/api/identities", () => ({
  getIdentities: vi.fn().mockResolvedValue({ hasPassword: true, identities: [] }),
  linkIdentity: vi.fn(),
  unlinkIdentity: vi.fn(),
}));

let restoreViewport = () => {};

function profile(overrides: Partial<ProfileResponse> = {}): ProfileResponse {
  return {
    id: 7,
    username: "kiro",
    avatarUrl: null,
    avatarLargeUrl: null,
    usernameChangedAt: null,
    languagePreference: "en",
    cardDeckPreference: "french",
    soundEnabled: true,
    musicEnabled: false,
    soundVolume: 70,
    musicVolume: 40,
    allowNonFriendInvites: true,
    createdAt: "2026-01-15T10:00:00Z",
    totalGamesPlayed: 0,
    wins: 0,
    losses: 0,
    abandoned: 0,
    totalXp: 0,
    level: 1,
    xpIntoLevel: 0,
    xpForNextLevel: 50,
    honorScore: 80,
    honorTier: "fair",
    honorCompletedTotal: 0,
    honorAbandonedTotal: 0,
    isNewPlayer: true,
    honorTrendDelta: 0,
    honorTrendDirection: "flat",
    seasonRank: null,
    ...overrides,
  };
}

function renderPage() {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={["/settings"]}>
        <Routes>
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/" element={<p data-testid="landing">landing</p>} />
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

beforeEach(() => {
  mockForgotPassword.mockReset().mockResolvedValue(undefined);
  mockLogout.mockReset();
  mockGetProfile.mockReset().mockResolvedValue(profile());
  mockUpdatePreferences.mockReset().mockResolvedValue({});
  useAuthStore.setState({
    token: "t",
    user: makeUser({ id: 7, username: "kiro", email: "kiro@example.com" }),
    isLoading: false,
  });
});

afterEach(async () => {
  restoreViewport();
  restoreViewport = () => {};
  await i18n.changeLanguage("en");
});

describe("SettingsPage", () => {
  it("shows every group on a phone, then Log out", () => {
    renderPage();
    for (const id of [
      "settings-account",
      "settings-sign-in",
      "settings-game",
      "profile-audio",
      "profile-invites",
    ]) {
      expect(screen.getByTestId(id), id).toBeInTheDocument();
    }
    expect(screen.getByTestId("settings-logout")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-nav")).not.toBeInTheDocument();
  });

  it("puts the picture, name and email together in the account block", async () => {
    renderPage();
    const account = screen.getByTestId("settings-account");
    expect(within(account).getByTestId("profile-username")).toHaveTextContent("kiro");
    expect(within(account).getByTestId("settings-email")).toHaveTextContent("kiro@example.com");
    expect(within(account).getByTestId("profile-avatar-edit")).toBeInTheDocument();
    await waitFor(() => expect(mockGetProfile).toHaveBeenCalledWith(7));
  });

  it("emails a password link to the signed-in address and says where it went", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByTestId("settings-password-send"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-password")).toHaveTextContent(
        "Link sent to kiro@example.com.",
      ),
    );
    expect(mockForgotPassword).toHaveBeenCalledWith({ email: "kiro@example.com" });
    expect(screen.queryByTestId("settings-password-send")).not.toBeInTheDocument();
  });

  it("switches the language and saves it as the player's preference", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(
      within(screen.getByTestId("settings-language")).getByRole("radio", { name: "MK" }),
    );

    await waitFor(() => expect(i18n.language).toBe("mk"));
    expect(mockUpdatePreferences).toHaveBeenCalledWith(7, { languagePreference: "mk" });
  });

  it("logs out and lands on the landing page", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByTestId("settings-logout"));

    expect(await screen.findByTestId("landing")).toBeInTheDocument();
    expect(useAuthStore.getState().user).toBeNull();
  });

  it("lists the sections beside the groups on a desktop and marks the one picked", async () => {
    restoreViewport = mockViewportWidth(1280);
    Element.prototype.scrollIntoView = vi.fn();
    const user = userEvent.setup();
    renderPage();

    const nav = screen.getByTestId("settings-nav");
    expect(within(nav).getAllByRole("button")).toHaveLength(6);
    await user.click(screen.getByTestId("settings-nav-sound"));

    expect(screen.getByTestId("settings-nav-sound")).toHaveAttribute("aria-current", "true");
    expect(screen.getByTestId("settings-nav-account")).not.toHaveAttribute("aria-current");
  });

  it("splits the groups into two columns on a tablet", () => {
    restoreViewport = mockViewportWidth(820);
    renderPage();
    const account = screen.getByTestId("settings-account");
    const game = screen.getByTestId("settings-game");
    expect(account.parentElement).not.toBe(game.parentElement);
  });
});
