import "@/shared/i18n/i18n";

import { render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import type { CareerResponse } from "@/shared/api/career";
import type { MatchesListResponse, MatchListItem } from "@/shared/api/matches";
import type { PublicProfileResponse } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser, QueryWrapper } from "@/test-utils";

import { PublicPlayerProfilePage } from "./PublicPlayerProfilePage";

const mockGetPublicProfile = vi.fn();
vi.mock("@/shared/api/profile", () => ({
  getProfile: vi.fn(),
  getPublicProfile: (...args: unknown[]) => mockGetPublicProfile(...args),
  updatePreferences: vi.fn(),
  updateUsername: vi.fn(),
}));

const mockGetCareer = vi.fn();
vi.mock("@/shared/api/career", () => ({
  getCareer: (...args: unknown[]) => mockGetCareer(...args),
}));

const mockGetUserMatches = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getUserMatches: (...args: unknown[]) => mockGetUserMatches(...args),
}));

function publicProfileFixture(
  overrides: Partial<PublicProfileResponse> = {},
): PublicProfileResponse {
  return {
    id: 2,
    username: "subjectplayer",
    createdAt: "2026-02-01T10:00:00Z",
    totalGamesPlayed: 12,
    wins: 7,
    losses: 4,
    abandoned: 1,
    totalXp: 600,
    level: 3,
    xpIntoLevel: 150,
    xpForNextLevel: 350,
    honorScore: 30,
    honorTier: "problematic",
    honorCompletedTotal: 11,
    honorAbandonedTotal: 1,
    isNewPlayer: false,
    honorTrendDelta: 0,
    honorTrendDirection: "flat",
    ...overrides,
  };
}

function careerFixture(overrides: Partial<CareerResponse> = {}): CareerResponse {
  return {
    capots: 2,
    avgMatchSeconds: 1500,
    careerPoints: 15840,
    streak: { kind: "win", length: 3 },
    topPartners: [],
    topRivals: [],
    ...overrides,
  };
}

function emptyMatches(): MatchesListResponse {
  return { items: [], total: 0, limit: 20, offset: 0 };
}

// A single completed match with the SUBJECT at seat 0 (viewer-relative to the
// subject, as the server computes it for a public request).
function subjectMatch(): MatchListItem {
  return {
    id: 1,
    variant: "bitola",
    matchMode: "1001",
    startedAt: "2026-02-01T12:00:00Z",
    completedAt: "2026-02-01T12:20:00Z",
    status: "completed",
    winnerTeam: 0,
    teamAScore: 1010,
    teamBScore: 640,
    hasBots: false,
    viewerSeat: 0,
    outcome: "win",
    endReason: "natural",
    players: [
      { seat: 0, userId: 2, username: "subjectplayer", isBot: false },
      { seat: 1, userId: 3, username: "opp1", isBot: false },
      { seat: 2, userId: 4, username: "mate", isBot: false },
      { seat: 3, userId: 5, username: "opp2", isBot: false },
    ],
    hands: [],
  };
}

function renderAt(id: string) {
  return render(
    <QueryWrapper>
      <MemoryRouter initialEntries={[`/players/${id}`]}>
        <Routes>
          <Route path="/players/:id" element={<PublicPlayerProfilePage />} />
        </Routes>
      </MemoryRouter>
    </QueryWrapper>,
  );
}

describe("PublicPlayerProfilePage (Story 11.3)", () => {
  beforeEach(() => {
    mockGetPublicProfile.mockReset();
    mockGetCareer.mockReset();
    mockGetUserMatches.mockReset();
    mockGetCareer.mockResolvedValue(careerFixture());
    mockGetUserMatches.mockResolvedValue(emptyMatches());
    // A viewer with a DISTINCT honor from the subject, so a hydration leak would
    // be obvious. This is the inverse of ProfilePage's honor-hydration test.
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 1, honorScore: 90, honorTier: "trusted", isNewPlayer: false }),
      isLoading: false,
    });
  });

  it("renders the subject's profile for a foreign id", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());

    renderAt("2");

    await waitFor(() => expect(screen.getByTestId("public-profile-page")).toBeInTheDocument());
    expect(mockGetPublicProfile).toHaveBeenCalledWith(2);
    expect(screen.getByTestId("profile-username")).toHaveTextContent("subjectplayer");
    // The honor band shows the SUBJECT's recomputed score, not the viewer's 90.
    expect(screen.getByTestId("profile-honor")).toHaveAttribute("data-honor", "30");
  });

  it("does not render the wallet pill, streak pill, or username edit pencil", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());

    const { container } = renderAt("2");

    await waitFor(() => expect(screen.getByTestId("public-profile-page")).toBeInTheDocument());
    expect(container.querySelector('[aria-label^="Coins"]')).toBeNull();
    expect(container.querySelector('[aria-label^="Day streak"]')).toBeNull();
    expect(screen.queryByTestId("profile-edit-username-button")).not.toBeInTheDocument();
  });

  it("never mounts the LinkedAccounts (SSO) surface", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());

    renderAt("2");

    await waitFor(() => expect(screen.getByTestId("public-profile-page")).toBeInTheDocument());
    expect(screen.queryByTestId("linked-accounts-loading")).not.toBeInTheDocument();
    expect(screen.queryByTestId("linked-accounts-error")).not.toBeInTheDocument();
    expect(screen.queryByTestId("linked-account-google")).not.toBeInTheDocument();
  });

  it("does NOT hydrate the viewer's auth-store honor from the subject's profile", async () => {
    mockGetPublicProfile.mockResolvedValue(
      publicProfileFixture({ honorScore: 30, honorTier: "problematic" }),
    );

    renderAt("2");

    await waitFor(() => expect(screen.getByTestId("public-profile-page")).toBeInTheDocument());
    // The viewer's own honor in the store is untouched — the self-only hydration
    // effect is structurally absent from this page (D5).
    expect(useAuthStore.getState().user?.honorScore).toBe(90);
    expect(useAuthStore.getState().user?.honorTier).toBe("trusted");
  });

  it("shows the New Player state for a subject under the match floor", async () => {
    mockGetPublicProfile.mockResolvedValue(
      publicProfileFixture({ honorScore: 86, honorTier: "trusted", isNewPlayer: true }),
    );

    renderAt("2");

    await waitFor(() => expect(screen.getByTestId("public-profile-page")).toBeInTheDocument());
    expect(screen.getByTestId("profile-honor-new")).toBeInTheDocument();
    expect(screen.queryByTestId("profile-honor-score")).not.toBeInTheDocument();
  });

  it("renders the career points milestone", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());
    mockGetCareer.mockResolvedValue(careerFixture({ careerPoints: 15840 }));

    renderAt("2");

    await waitFor(() => expect(screen.getByTestId("profile-milestones")).toBeInTheDocument());
    expect(mockGetCareer).toHaveBeenCalledWith(2);
    expect(screen.getByTestId("profile-milestones")).toHaveTextContent((15840).toLocaleString());
  });

  it("labels the subject's own seat with their username, never a YOU badge", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());
    mockGetUserMatches.mockResolvedValue({
      items: [subjectMatch()],
      total: 1,
      limit: 20,
      offset: 0,
    });

    renderAt("2");

    const history = await screen.findByTestId("match-history-list");
    expect(within(history).getAllByText("subjectplayer").length).toBeGreaterThan(0);
    // The subject is NOT the viewer, so no "You" marker anywhere in the row.
    expect(within(history).queryByText("You")).not.toBeInTheDocument();
  });

  it("renders no seasonal-rank section (Epic 13 unbuilt)", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());

    renderAt("2");

    await waitFor(() => expect(screen.getByTestId("public-profile-page")).toBeInTheDocument());
    // Pins the graceful-absence contract for when Epic 13 lands.
    expect(screen.queryByTestId("profile-season")).not.toBeInTheDocument();
    expect(screen.queryByTestId("prior-season-archive")).not.toBeInTheDocument();
  });

  it("renders a not-found state for an unknown/soft-deleted subject (404)", async () => {
    mockGetPublicProfile.mockRejectedValue(new FetchError(404, "USER_NOT_FOUND", "not found"));

    renderAt("999");

    await waitFor(() => expect(screen.getByTestId("public-profile-not-found")).toBeInTheDocument());
    expect(screen.queryByTestId("public-profile-page")).not.toBeInTheDocument();
  });

  it("renders a not-found state for a non-numeric id without querying the API", async () => {
    renderAt("abc");

    expect(screen.getByTestId("public-profile-not-found")).toBeInTheDocument();
    expect(mockGetPublicProfile).not.toHaveBeenCalled();
  });

  it("renders not-found for a non-canonical numeric id (1e2) without querying the API", async () => {
    // Number("1e2") === 100, but the strict guard rejects the non-canonical
    // spelling so it cannot alias to a real-but-different subject (player 100).
    renderAt("1e2");

    expect(screen.getByTestId("public-profile-not-found")).toBeInTheDocument();
    expect(mockGetPublicProfile).not.toHaveBeenCalled();
  });

  it("shows a subject-addressed empty match history with no lobby CTA when the subject never played", async () => {
    mockGetPublicProfile.mockResolvedValue(
      publicProfileFixture({ totalGamesPlayed: 0, wins: 0, losses: 0, abandoned: 0 }),
    );
    mockGetUserMatches.mockResolvedValue(emptyMatches());

    renderAt("2");

    const empty = await screen.findByTestId("match-history-empty");
    // Third-person copy about the subject, not the viewer's "Quick Play" onboarding.
    expect(empty).toHaveTextContent(/hasn't played any matches/i);
    // The self-only "go play" CTA must never render on someone else's profile.
    expect(screen.queryByTestId("match-history-empty-cta")).not.toBeInTheDocument();
  });

  // Every surface here is written for the SELF page by default, where the reader
  // is the subject. On a public profile the reader is a visitor, so second-person
  // copy told them to play 5 matches / to shake off a losing run that belongs to
  // someone else entirely.
  it("writes the honour newcomer hint about the subject, not at the viewer", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture({ isNewPlayer: true }));

    renderAt("2");

    const hint = await screen.findByTestId("profile-honor-new-hint");
    expect(hint).toHaveTextContent("Gets an honor score after 5 matches.");
    expect(hint).not.toHaveTextContent(/^Play 5 matches/);
  });

  it("writes the honour trend tooltip about the subject, not at the viewer", async () => {
    mockGetPublicProfile.mockResolvedValue(
      publicProfileFixture({ honorTrendDelta: 4, honorTrendDirection: "up" }),
    );

    renderAt("2");

    const trend = await screen.findByTestId("profile-honor-trend");
    expect(trend).toHaveAttribute(
      "title",
      "Their last 20 matches compared with the 20 before them.",
    );
  });

  it("states the subject's streak as a fact, with no Play nudge at the viewer", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());
    mockGetCareer.mockResolvedValue(careerFixture({ streak: { kind: "loss", length: 3 } }));

    renderAt("2");

    const callout = await screen.findByTestId("profile-streak");
    expect(callout).toHaveTextContent("The run breaks with their next win.");
    expect(callout).not.toHaveTextContent(/Shake it off/);
    expect(screen.queryByTestId("profile-streak-play")).not.toBeInTheDocument();
  });

  it("drops the 'play a few matches' prompt from the empty partner and rival panels", async () => {
    mockGetPublicProfile.mockResolvedValue(publicProfileFixture());
    mockGetCareer.mockResolvedValue(careerFixture({ topPartners: [], topRivals: [] }));

    renderAt("2");

    const partners = await screen.findByTestId("profile-partners");
    expect(partners).toHaveTextContent("No regular partners yet.");
    expect(partners).not.toHaveTextContent(/play a few matches/i);
    expect(screen.getByTestId("profile-rivals")).toHaveTextContent("No rivals yet.");
    expect(screen.getByTestId("profile-rivals")).not.toHaveTextContent(/play a few matches/i);
  });
});
