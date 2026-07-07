import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CareerResponse } from "@/shared/api/career";
import type { ProfileResponse } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import { QueryWrapper } from "@/test-utils";

import { ProfilePage } from "./ProfilePage";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

const mockGetProfile = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  getProfile: (...args: unknown[]) => mockGetProfile(...args),
  updatePreferences: vi.fn().mockResolvedValue({ languagePreference: "en" }),
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
    languagePreference: "en",
    createdAt: "2026-01-15T10:00:00Z",
    totalGamesPlayed: 0,
    wins: 0,
    losses: 0,
    abandoned: 0,
    totalXp: 0,
    level: 0,
    xpIntoLevel: 0,
    xpForNextLevel: 50,
    ...overrides,
  };
}

function careerFixture(overrides: Partial<CareerResponse> = {}): CareerResponse {
  return {
    capots: 0,
    avgMatchSeconds: 0,
    streak: { kind: "none", length: 0 },
    topPartners: [],
    topRivals: [],
    ...overrides,
  };
}

describe("ProfilePage", () => {
  beforeEach(() => {
    mockGetProfile.mockReset();
    mockGetCareer.mockReset();
    mockGetUserMatches.mockReset();
    mockGetIdentities.mockReset();
    mockGetCareer.mockResolvedValue(careerFixture());
    // Default: MatchHistory renders the empty state so existing tests need no per-case setup.
    mockGetUserMatches.mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
    // Default: LinkedAccounts panel resolves to "no linked accounts".
    mockGetIdentities.mockResolvedValue({ hasPassword: true, identities: [] });
    useAuthStore.setState({
      token: "test-token",
      user: {
        id: 1,
        username: "testuser",
        email: "test@example.com",
        languagePreference: "en",
        walletBalance: 5000,
        loginStreakDays: 1,
        totalXp: 0,
        level: 0,
        createdAt: "2026-01-15T10:00:00Z",
      },
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
    // Denominator is totalGamesPlayed (abandoned included): 7 / 10 = 70%.
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

    // 4 / 10 = 40% — NOT 4 / (4+3) = 57%.
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

    // Error branch replaces the tile grid — numeric tiles must not render.
    expect(screen.queryByTestId("profile-stat-games-played")).not.toBeInTheDocument();
  });

  it("renders em-dash for the win-rate ring when zero games played", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-win-rate-ring")).toBeInTheDocument();
    });

    const ring = screen.getByTestId("profile-win-rate-ring");
    expect(ring).toHaveAttribute("data-rate", "");
    expect(ring.textContent).toContain("—");
    expect(ring.textContent).not.toContain("NaN");
    expect(ring.textContent).not.toContain("0%");
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
        topPartners: [{ userId: 2, username: "partner_a", played: 4, wins: 3 }],
        topRivals: [{ userId: 3, username: "rival_x", wins: 2, losses: 1 }],
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
    // Win streak callout shows.
    expect(screen.getByTestId("profile-streak")).toHaveAttribute("data-streak-kind", "win");
  });
});
