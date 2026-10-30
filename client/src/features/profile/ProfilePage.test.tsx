import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { CareerResponse } from "@/shared/api/career";
import type { ProfileResponse } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";
import { makeUser, mockViewportWidth, QueryWrapper } from "@/test-utils";

import { ProfilePage } from "./ProfilePage";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

const mockGetProfile = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  getProfile: (...args: unknown[]) => mockGetProfile(...args),
  updatePreferences: vi.fn().mockResolvedValue({ languagePreference: "en" }),
  updateUsername: vi.fn().mockResolvedValue({ username: "testuser", usernameChangedAt: null }),
}));

const mockGetCareer = vi.fn();
vi.mock("@/shared/api/career", () => ({
  getCareer: (...args: unknown[]) => mockGetCareer(...args),
}));

const mockGetUserMatches = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getUserMatches: (...args: unknown[]) => mockGetUserMatches(...args),
}));

const mockGetIdentities = vi.fn();
vi.mock("@/shared/api/identities", () => ({
  getIdentities: (...args: unknown[]) => mockGetIdentities(...args),
  linkIdentity: vi.fn(),
  unlinkIdentity: vi.fn(),
}));

const mockGetSeasonArchive = vi.fn();
const mockGetCurrentSeason = vi.fn();
vi.mock("@/shared/api/season", () => ({
  getSeasonArchive: (...args: unknown[]) => mockGetSeasonArchive(...args),
  getCurrentSeason: (...args: unknown[]) => mockGetCurrentSeason(...args),
}));

const currentSeason: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: "2099-10-01T00:00:00Z",
  sp: 680,
  rankTier: "gold",
  rankDivision: 2,
  spIntoDivision: 13,
  spForNextDivision: 67,
  gamesPlayed: 31,
  gamesWon: 15,
  gamesCompleted: 29,
};

const RANK = {
  seasonName: "2026 Q3",
  tier: "gold",
  division: 2,
  sp: 680,
  position: 3,
  players: 40,
  gamesPlayed: 31,
  gamesWon: 15,
};

function renderProfilePage() {
  return render(
    <QueryWrapper>
      <BrowserRouter>
        <ProfilePage />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

function profileFixture(overrides: Partial<ProfileResponse> = {}): ProfileResponse {
  return {
    id: 1,
    username: "testuser",
    avatarUrl: null,
    avatarLargeUrl: null,
    languagePreference: "en",
    cardDeckPreference: "french",
    soundEnabled: true,
    musicEnabled: true,
    soundVolume: 70,
    musicVolume: 70,
    allowNonFriendInvites: true,
    createdAt: "2026-01-15T10:00:00Z",
    totalGamesPlayed: 0,
    wins: 0,
    losses: 0,
    abandoned: 0,
    totalXp: 0,
    level: 0,
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

function careerFixture(overrides: Partial<CareerResponse> = {}): CareerResponse {
  return {
    capots: 0,
    avgMatchSeconds: 0,
    careerPoints: 0,
    streak: { kind: "none", length: 0 },
    topPartners: [],
    topRivals: [],
    ...overrides,
  };
}

let restoreViewport = () => {};

describe("ProfilePage", () => {
  afterEach(() => {
    restoreViewport();
  });

  beforeEach(() => {
    restoreViewport = mockViewportWidth(1280);
    mockGetProfile.mockReset();
    mockGetCareer.mockReset();
    mockGetUserMatches.mockReset();
    mockGetIdentities.mockReset();
    mockGetSeasonArchive.mockReset();
    mockGetSeasonArchive.mockResolvedValue({ items: [] });
    mockGetCurrentSeason.mockReset();
    mockGetCurrentSeason.mockResolvedValue(currentSeason);
    mockGetCareer.mockResolvedValue(careerFixture());
    mockGetUserMatches.mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [] });
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({
        loginStreakDays: 1,
        createdAt: "2026-01-15T10:00:00Z",
      }),
      isLoading: false,
    });
  });

  it("renders loading state initially", () => {
    mockGetProfile.mockReturnValue(new Promise(() => {}));
    renderProfilePage();

    expect(screen.getByTestId("profile-loading")).toBeInTheDocument();
  });

  it("renders username after profile loads", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-username")).toHaveTextContent("testuser");
    });
  });

  it("renders member since date", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-member-since")).toBeInTheDocument();
    });

    expect(screen.getByTestId("profile-member-since").textContent).toContain("2026");
  });

  it("renders match-history section + four stat tiles", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("match-history")).toBeInTheDocument();
    });

    expect(screen.getByTestId("profile-stats")).toBeInTheDocument();
    expect(screen.getByTestId("profile-stat-games-played")).toHaveAttribute("data-value", "0");
    expect(screen.getByTestId("profile-stat-wins")).toHaveAttribute("data-value", "0");
    expect(screen.getByTestId("profile-stat-losses")).toHaveAttribute("data-value", "0");
    expect(screen.getByTestId("profile-stat-abandoned")).toHaveAttribute("data-value", "0");
  });

  it("handles profile fetch error gracefully — falls back to auth store username", async () => {
    mockGetProfile.mockRejectedValueOnce(new Error("Network error"));

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-page")).toBeInTheDocument();
    });

    expect(screen.getByTestId("profile-username")).toHaveTextContent("testuser");
  });

  it("draws the profile's large avatar in the hero", async () => {
    const large = "https://assets.test/avatars/me/256.webp";
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({
        avatarUrl: "https://assets.test/avatars/me/128.webp",
        avatarLargeUrl: large,
      }),
    );

    renderProfilePage();

    const hero = await screen.findByTestId("profile-identity-hero");
    await waitFor(() => expect(hero.querySelector("img")).toHaveAttribute("src", large));
  });

  it("falls back to the auth store's avatar while there is no profile", async () => {
    const small = "https://assets.test/avatars/me/128.webp";
    useAuthStore.setState({ user: makeUser({ avatarUrl: small }) });
    mockGetProfile.mockRejectedValueOnce(new Error("Network error"));

    renderProfilePage();

    const hero = await screen.findByTestId("profile-identity-hero");
    expect(hero.querySelector("img")).toHaveAttribute("src", small);
  });

  it("renders real stats + win-rate ring when profile has played games", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 10, wins: 7, losses: 2, abandoned: 1 }),
    );

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-stat-games-played")).toHaveAttribute("data-value", "10");
    });

    expect(screen.getByTestId("profile-stat-wins")).toHaveAttribute("data-value", "7");
    expect(screen.getByTestId("profile-stat-losses")).toHaveAttribute("data-value", "2");
    expect(screen.getByTestId("profile-stat-abandoned")).toHaveAttribute("data-value", "1");
    const ring = screen.getByTestId("profile-win-rate-ring");
    expect(ring).toHaveAttribute("data-rate", "70");
    expect(ring.textContent).toContain("70%");
  });

  it("counts abandoned games in the win-rate denominator", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 10, wins: 4, losses: 3, abandoned: 3 }),
    );

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-stat-games-played")).toHaveAttribute("data-value", "10");
    });

    const ring = screen.getByTestId("profile-win-rate-ring");
    expect(ring).toHaveAttribute("data-rate", "40");
    expect(ring.textContent).toContain("40%");
  });

  it("renders stats error state when profile query fails", async () => {
    mockGetProfile.mockRejectedValueOnce(new Error("500 Internal Server Error"));

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-stats-error")).toBeInTheDocument();
    });

    expect(screen.queryByTestId("profile-stat-games-played")).not.toBeInTheDocument();
  });

  it("renders a dash placeholder for the win-rate ring when zero games played", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-win-rate-ring")).toBeInTheDocument();
    });

    const ring = screen.getByTestId("profile-win-rate-ring");
    expect(ring).toHaveAttribute("data-rate", "");
    expect(ring.textContent).toContain("–");
    expect(ring.textContent).not.toContain("—");
    expect(ring.textContent).not.toContain("NaN");
    expect(ring.textContent).not.toContain("0%");
  });

  it("hydrates the auth store's honor from the profile response", async () => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ honorScore: 90, honorTier: "trusted", isNewPlayer: false }),
      isLoading: false,
    });
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({
        honorScore: 44,
        honorTier: "problematic",
        isNewPlayer: false,
        honorCompletedTotal: 0,
        honorAbandonedTotal: 1,
      }),
    );

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-honor", "44");
    });
    await waitFor(() => {
      expect(useAuthStore.getState().user?.honorScore).toBe(44);
    });
    expect(useAuthStore.getState().user?.honorTier).toBe("problematic");
  });

  it("hydrates a zero honor score rather than treating it as absent", async () => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ honorScore: 96, honorTier: "exemplary", isNewPlayer: false }),
      isLoading: false,
    });
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ honorScore: 0, honorTier: "problematic", isNewPlayer: false }),
    );

    renderProfilePage();

    await waitFor(() => {
      expect(useAuthStore.getState().user?.honorScore).toBe(0);
    });
  });

  it("leaves the auth store alone when the profile honor fields are malformed", async () => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ honorScore: 90, honorTier: "trusted", isNewPlayer: false }),
      isLoading: false,
    });
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({
        honorScore: undefined as unknown as number,
        honorTier: "" as unknown as string,
      }),
    );

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-username")).toBeInTheDocument();
    });
    expect(useAuthStore.getState().user?.honorScore).toBe(90);
    expect(useAuthStore.getState().user?.honorTier).toBe("trusted");
  });

  it("renders the career sidebar (partners, rivals, milestones) when career loads", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 5, wins: 3, losses: 2 }),
    );
    mockGetCareer.mockResolvedValueOnce(
      careerFixture({
        capots: 2,
        avgMatchSeconds: 1500,
        streak: { kind: "win", length: 3 },
        topPartners: [{ userId: 2, username: "partner_a", avatarUrl: null, played: 4, wins: 3 }],
        topRivals: [{ userId: 3, username: "rival_x", avatarUrl: null, wins: 2, losses: 1 }],
      }),
    );

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-partners")).toBeInTheDocument();
    });

    expect(screen.getByTestId("profile-rivals")).toBeInTheDocument();
    expect(screen.getByTestId("profile-milestones")).toBeInTheDocument();
    expect(screen.getByTestId("profile-partners")).toHaveTextContent("partner_a");
    expect(screen.getByTestId("profile-rivals")).toHaveTextContent("rival_x");
    expect(screen.getByTestId("profile-streak")).toHaveAttribute("data-streak-kind", "win");
  });

  it("links partner and rival names to their public profiles", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 5, wins: 3, losses: 2 }),
    );
    mockGetCareer.mockResolvedValueOnce(
      careerFixture({
        topPartners: [
          { userId: 2, username: "partner_a", avatarUrl: null, played: 4, wins: 3 },
          { userId: 5, username: "partner_b", avatarUrl: null, played: 2, wins: 1 },
        ],
        topRivals: [{ userId: 3, username: "rival_x", avatarUrl: null, wins: 2, losses: 1 }],
      }),
    );

    renderProfilePage();

    const partners = await screen.findByTestId("profile-partners");
    expect(
      within(partners).getByRole("link", { name: "View partner_a's profile" }),
    ).toHaveAttribute("href", "/players/2");
    expect(
      within(partners).getByRole("link", { name: "View partner_b's profile" }),
    ).toHaveAttribute("href", "/players/5");
    expect(
      within(screen.getByTestId("profile-rivals")).getByRole("link", {
        name: "View rival_x's profile",
      }),
    ).toHaveAttribute("href", "/players/3");
  });

  it("links to Settings and keeps every preference off the profile", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());
    renderProfilePage();

    expect(await screen.findByTestId("profile-settings-link")).toHaveAttribute("href", "/settings");
    for (const id of [
      "profile-card-deck",
      "profile-audio",
      "profile-invites",
      "profile-linked-accounts",
    ]) {
      expect(screen.queryByTestId(id), id).not.toBeInTheDocument();
    }
    expect(screen.queryByTestId("profile-avatar-edit")).not.toBeInTheDocument();
  });

  it("keeps the second-person streak copy and the Play link on the viewer's own profile", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 5, wins: 3, losses: 2 }),
    );
    mockGetCareer.mockResolvedValueOnce(careerFixture({ streak: { kind: "loss", length: 3 } }));

    renderProfilePage();

    const callout = await screen.findByTestId("profile-streak");
    expect(callout).toHaveTextContent(/Shake it off/);
    expect(screen.getByTestId("profile-streak-play")).toBeInTheDocument();
  });

  it("shows this season's rank, the climb to the next rank, place and record", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture({ seasonRank: RANK }));
    renderProfilePage();

    const card = await screen.findByTestId("leaderboard-standing");
    expect(card).toHaveTextContent("Gold 2");
    await waitFor(() => expect(card).toHaveTextContent("54 SP to Gold 3"));
    expect(within(card).getByTestId("leaderboard-standing-place")).toHaveTextContent("#3");
    expect(within(card).getByTestId("leaderboard-standing-place")).toHaveTextContent("of 40");
    expect(within(card).getByTestId("leaderboard-standing-record")).toHaveTextContent("15–16");
    expect(screen.getByTestId("profile-season")).toHaveTextContent("2026 Q3");
  });

  it("invites a first match when you have not played this season", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture({ seasonRank: null }));
    renderProfilePage();

    const card = await screen.findByTestId("leaderboard-standing-empty");
    expect(within(card).getByRole("link", { name: "Play" })).toHaveAttribute("href", "/lobby");
  });

  it("lists past seasons in the sidebar when there is season history", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture({ seasonRank: RANK }));
    mockGetSeasonArchive.mockResolvedValue({
      items: [
        {
          seasonId: 5,
          seasonName: "2026 Q2",
          sp: 450,
          tier: "silver",
          division: null,
          gamesPlayed: 14,
          gamesWon: 7,
          startedAt: "2026-04-01T00:00:00Z",
          endsAt: "2026-07-01T00:00:00Z",
        },
      ],
    });

    renderProfilePage();

    const list = await screen.findByTestId("prior-season-archive");
    expect(list.querySelectorAll('[data-testid="season-archive-row"]')).toHaveLength(1);
    expect(mockGetSeasonArchive).toHaveBeenCalledWith(1);
  });

  it("leaves the past-seasons panel out without season history", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture({ seasonRank: null }));
    mockGetSeasonArchive.mockResolvedValue({ items: [] });

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-username")).toHaveTextContent("testuser");
    });
    await waitFor(() => expect(mockGetSeasonArchive).toHaveBeenCalled());
    expect(screen.queryByTestId("profile-season-section")).not.toBeInTheDocument();
  });

  it("splits a phone profile into Overview, Matches and Seasons tabs", async () => {
    restoreViewport();
    restoreViewport = () => {};
    const user = userEvent.setup();
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 4, wins: 2, losses: 2 }),
    );
    renderProfilePage();

    expect(await screen.findByTestId("profile-stats")).toBeInTheDocument();
    expect(screen.queryByTestId("match-history")).not.toBeInTheDocument();

    await user.click(
      within(screen.getByTestId("profile-tabs")).getByRole("radio", { name: "Matches" }),
    );
    expect(await screen.findByTestId("match-history")).toBeInTheDocument();
    expect(screen.queryByTestId("profile-stats")).not.toBeInTheDocument();

    await user.click(
      within(screen.getByTestId("profile-tabs")).getByRole("radio", { name: "Seasons" }),
    );
    expect(await screen.findByTestId("profile-seasons-empty")).toHaveTextContent(
      "No finished seasons yet.",
    );
  });
});
