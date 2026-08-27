import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CareerResponse } from "@/shared/api/career";
import type { ProfileResponse } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser, QueryWrapper } from "@/test-utils";

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

// SeasonSection is in the page tree, so the archive read must be mocked
// (default: nothing to show — the section stays out of the DOM), and the page
// now also mounts the RankBanner, which reads the viewer's current standing.
const mockGetSeasonArchive = vi.fn();
const mockGetCurrentSeason = vi.fn();
vi.mock("@/shared/api/season", () => ({
  getSeasonArchive: (...args: unknown[]) => mockGetSeasonArchive(...args),
  getCurrentSeason: (...args: unknown[]) => mockGetCurrentSeason(...args),
}));

const currentSeason = {
  seasonName: "2026 Q3",
  endsAt: "2099-10-01T00:00:00Z",
  sp: 4000,
  rankTier: "gold",
  spIntoTier: 1000,
  spForNextTier: 2500,
  gamesPlayed: 31,
  gamesCompleted: 29,
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
    languagePreference: "en",
    cardDeckPreference: "french",
    createdAt: "2026-01-15T10:00:00Z",
    totalGamesPlayed: 0,
    wins: 0,
    losses: 0,
    abandoned: 0,
    totalXp: 0,
    level: 0,
    xpIntoLevel: 0,
    xpForNextLevel: 50,
    // Honor (Story 9.7). Defaults describe a brand-new player: the Beta(4,1)
    // prior of 80 with no history, so isNewPlayer is true.
    honorScore: 80,
    honorTier: "fair",
    honorCompletedTotal: 0,
    honorAbandonedTotal: 0,
    isNewPlayer: true,
    honorTrendDelta: 0,
    honorTrendDirection: "flat",
    // Story 13.3: null is the wire default for a player who has not played
    // this season — the rank chip hides on it.
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

describe("ProfilePage", () => {
  beforeEach(() => {
    mockGetProfile.mockReset();
    mockGetCareer.mockReset();
    mockGetUserMatches.mockReset();
    mockGetIdentities.mockReset();
    mockGetSeasonArchive.mockReset();
    // Default: no season history → SeasonSection contributes nothing.
    mockGetSeasonArchive.mockResolvedValue({ items: [] });
    mockGetCurrentSeason.mockReset();
    // The endpoint answers a zero state rather than a 404 for a player who has
    // not played, so the banner has something to render in every test here.
    mockGetCurrentSeason.mockResolvedValue(currentSeason);
    mockGetCareer.mockResolvedValue(careerFixture());
    // Default: MatchHistory renders the empty state so existing tests need no per-case setup.
    mockGetUserMatches.mockResolvedValue({ items: [], total: 0, limit: 20, offset: 0 });
    // Default: LinkedAccounts panel resolves to "no linked accounts".
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

  // The placeholder is an EN dash, not an em dash: the value is a bare glyph
  // shared verbatim by all four locales, and em dashes are English-prose-only in
  // this project (mk/sr/hr must not contain one). Asserted here so the glyph
  // cannot drift back per-locale.
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

  // Regression test for the deferred-work item "Absent players never receive
  // their own event:honor_updated". The abandoning seat has no live socket when
  // the reconnect window expires, so it never gets the event and authStore keeps
  // the pre-abandonment values. Opening /profile used to render the fresh score
  // in HonorPanel beside the stale one in the header chip. The profile response
  // is authoritative (it recomputes from the stored weights), so it now hydrates
  // authStore and both surfaces agree.
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

  // A score of 0 is a real value and isNewPlayer false is a real value; both are
  // falsy, so a truthiness guard would silently skip the hydration.
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

  it("links partner and rival names to their public profiles", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ totalGamesPlayed: 5, wins: 3, losses: 2 }),
    );
    mockGetCareer.mockResolvedValueOnce(
      careerFixture({
        // Two partners so BOTH PartnerSpotlight link paths render: partner_a
        // is the featured block, partner_b takes the rest-list row.
        topPartners: [
          { userId: 2, username: "partner_a", played: 4, wins: 3 },
          { userId: 5, username: "partner_b", played: 2, wins: 1 },
        ],
        topRivals: [{ userId: 3, username: "rival_x", wins: 2, losses: 1 }],
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

  // The deck panel sits OUTSIDE the `career` gate: it renders off the auth
  // store, not career data, so a brand-new account with nothing to show in the
  // career sidebar must still be able to change its deck (Story 12.4).
  it("renders the card-deck picker with the store's deck selected, even with no career data", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture());
    mockGetCareer.mockRejectedValueOnce(new Error("no career yet"));
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ cardDeckPreference: "croatian" }),
      isLoading: false,
    });

    renderProfilePage();

    const panel = await screen.findByTestId("profile-card-deck");
    expect(panel).toBeInTheDocument();
    expect(screen.getByTestId("profile-deck-option-croatian")).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByTestId("profile-deck-option-french")).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  // The self page is the one place the second-person copy is correct — the
  // reader IS the subject here. Pins the subjectIsSelf default that the public
  // page overrides (Story 11.3 wording fix).
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

  // --- seasonal rank banner + prior-season archive ---

  // THE RANK BANNER IS THIS PAGE'S RANK SURFACE, and it is fed by
  // GET /seasons/current — not by the profile response's `seasonRank` block,
  // which carries no band decomposition and no countdown to draw a bar from.
  it("renders the rank banner from the current-season endpoint", async () => {
    renderProfilePage();

    const banner = await screen.findByTestId("rank-banner");
    expect(banner).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("rank-tier-name").textContent).toBe("Gold");
    // The progress the header chip deliberately omits: 1000 of a 2500 band.
    expect(screen.getByTestId("rank-progress")).toHaveAttribute("aria-valuenow", "40");
  });

  // ABOVE the streak callout, per the page's stated order: the season standing
  // is the longer-running fact and the streak is a note on current form.
  it("places the rank banner above the streak callout", async () => {
    mockGetCareer.mockResolvedValue(careerFixture({ streak: { kind: "win", length: 3 } }));

    renderProfilePage();

    const banner = await screen.findByTestId("rank-banner");
    const callout = await screen.findByTestId("profile-streak");
    expect(banner.compareDocumentPosition(callout)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  // ONE RANK READOUT PER PAGE. SeasonSection's own current-rank chip is
  // suppressed here (showCurrentRank={false}) precisely so the banner is not
  // shadowed by a smaller copy of the same standing a few sections down.
  it("renders the archive without the section's rank chip when the viewer has season history", async () => {
    mockGetProfile.mockResolvedValueOnce(
      profileFixture({ seasonRank: { seasonName: "2026 Q3", tier: "gold", sp: 4000 } }),
    );
    mockGetSeasonArchive.mockResolvedValue({
      items: [
        {
          seasonId: 5,
          seasonName: "2026 Q2",
          sp: 1800,
          tier: "silver",
          gamesPlayed: 14,
          startedAt: "2026-04-01T00:00:00Z",
          endsAt: "2026-07-01T00:00:00Z",
        },
      ],
    });

    renderProfilePage();

    const list = await screen.findByTestId("prior-season-archive");
    expect(list.querySelectorAll('[data-testid="season-archive-row"]')).toHaveLength(1);
    // The archive is the VIEWER's — keyed on their own id.
    expect(mockGetSeasonArchive).toHaveBeenCalledWith(1);
    // ...and the chip stays out even though the profile carries a seasonRank.
    expect(screen.queryByTestId("profile-season")).not.toBeInTheDocument();
  });

  // The hidden-when-empty contract, preserved from the pre-13.3 pages: with no
  // played seasons the whole SeasonSection leaves nothing in the DOM at all.
  // (The banner above is unaffected — every authed player has a current
  // standing, even if it is the Iron zero state.)
  it("renders no archive section at all without season history", async () => {
    mockGetProfile.mockResolvedValueOnce(profileFixture({ seasonRank: null }));
    mockGetSeasonArchive.mockResolvedValue({ items: [] });

    renderProfilePage();

    await waitFor(() => {
      expect(screen.getByTestId("profile-username")).toHaveTextContent("testuser");
    });
    await waitFor(() => expect(mockGetSeasonArchive).toHaveBeenCalled());
    expect(screen.queryByTestId("profile-season")).not.toBeInTheDocument();
    expect(screen.queryByTestId("prior-season-archive")).not.toBeInTheDocument();
  });
});
