import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "@/App";
import { LoginPage } from "@/features/auth/LoginPage";
import { LeaderboardPage } from "@/features/leaderboard/LeaderboardPage";
import { LobbyPage } from "@/features/lobby/LobbyPage";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

vi.mock("@/shared/api/auth", () => ({
  login: vi.fn(),
  logout: vi.fn(),
}));

// useAuthInit restores through the coordinated singleton (never /auth/refresh
// directly). Reject it so <App /> settles to the logged-out landing page
// instead of leaning on a real request failing in jsdom.
vi.mock("@/shared/api/axiosClient", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/shared/api/axiosClient")>()),
  refreshAccessToken: vi.fn(() => Promise.reject(new Error("no session"))),
}));

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  createRoom: vi.fn(),
}));

// Spread the original so the real AppLayout tree still finds WebSocketContext
// itself (the provider's context object), not just the two hooks. Mounting <App />
// on an authenticated route pulls that in for real.
vi.mock("@/shared/providers/WebSocketContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/shared/providers/WebSocketContext")>()),
  useWsSendMessage: () => vi.fn(),
  useWsConnectionState: () => "connected" as const,
}));

// AppLayout mounts DailyRewardGate and the season/leaderboard reads below it, so
// an authenticated route pulls these in for real.
vi.mock("@/shared/api/wallet", () => ({
  claimDailyLogin: vi.fn().mockResolvedValue({
    granted: false,
    amount: 0,
    streakDay: 1,
    newBalance: 5000,
    loginStreakDays: 1,
  }),
}));

vi.mock("@/shared/api/profile", () => ({
  updatePreferences: vi.fn().mockResolvedValue({ languagePreference: "en" }),
}));

vi.mock("@/shared/api/season", () => ({
  getCurrentSeason: vi.fn().mockResolvedValue({
    seasonName: "2026 Q3",
    endsAt: "2026-10-01T00:00:00Z",
    sp: 0,
    rankTier: "iron",
    spIntoTier: 0,
    spForNextTier: 500,
    gamesPlayed: 0,
    gamesCompleted: 0,
  }),
  getSeasonLeaderboard: vi.fn().mockResolvedValue({
    items: [{ position: 1, userId: 1, username: "ada", sp: 900, tier: "bronze", gamesPlayed: 4 }],
    total: 1,
    limit: 25,
    offset: 0,
    viewer: null,
  }),
  // Story 13.3: the season picker's list and the profile archive.
  getSeasons: vi.fn().mockResolvedValue({ items: [] }),
  getSeasonArchive: vi.fn().mockResolvedValue({ items: [] }),
}));

describe("App routing", () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders login page at /login", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/login"]}>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByTestId("login-title")).toHaveTextContent("Log in");
  });

  it("renders lobby page at /lobby", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/lobby"]}>
          <Routes>
            <Route path="/lobby" element={<LobbyPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByTestId("quick-play-card")).toBeInTheDocument();
    expect(screen.getByTestId("create-room-card")).toBeInTheDocument();
  });

  // Story 13.2: the /leaderboard route. Rendered through the same isolated
  // MemoryRouter the other route cases use — the page's own query is left
  // unresolved, so this asserts the route MOUNTS, which is what App owns.
  it("renders the leaderboard page at /leaderboard", () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/leaderboard"]}>
          <Routes>
            <Route path="/leaderboard" element={<LeaderboardPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByTestId("leaderboard-page")).toBeInTheDocument();
  });

  // P18: EVERY other test in this repo declares its own <Route> for
  // /leaderboard (this file did, TopBar.test.tsx stubs a target, AppLayout's
  // harness has its own tree), so deleting the real route from App.tsx left the
  // whole suite green while the nav tab silently redirected to /lobby via the
  // "*" catch-all.
  //
  // This one renders the REAL <App /> router at /leaderboard with a token in the
  // store, pinning three things together that no other test covers: the path is
  // registered, it sits inside ProtectedRoute (an authed user gets through), and
  // it sits inside AppLayout (the TopBar renders above it).
  it("serves /leaderboard from the real router, inside ProtectedRoute and AppLayout", async () => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, username: "kiro" }),
      isLoading: false,
    });
    window.history.replaceState(null, "", "/leaderboard");

    render(<App />);

    // The page itself...
    expect(await screen.findByTestId("leaderboard-page")).toBeInTheDocument();
    // ...under AppLayout's TopBar, with the tab marked active.
    expect(screen.getByTestId("app-nav")).toBeInTheDocument();
    expect(screen.getByTestId("nav-leaderboard")).toHaveAttribute("aria-current", "page");
  });

  // The other half of the gate: without a token the same URL must NOT render the
  // page. Proves the route is inside ProtectedRoute rather than beside it.
  it("redirects an unauthenticated visitor away from /leaderboard", async () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    window.history.replaceState(null, "", "/leaderboard");

    render(<App />);

    // ProtectedRoute sends an unauthenticated visitor to the public landing.
    expect(await screen.findByTestId("landing-page")).toBeInTheDocument();
    expect(screen.queryByTestId("leaderboard-page")).not.toBeInTheDocument();
  });

  // Regression guard: the sonner <Toaster> host must be mounted at the app
  // root. Without it, every toast.*() call (join/settlement feedback, auth
  // errors) is silently invisible even though the calls succeed — a bug that
  // unit tests miss because they mock `toast`. The Toaster renders an
  // aria-label="Notifications …" region regardless of route or auth state.
  it("mounts the global sonner Toaster so toasts are visible app-wide", async () => {
    render(<App />);

    expect(await screen.findByRole("region", { name: /notifications/i })).toBeInTheDocument();
  });
});
