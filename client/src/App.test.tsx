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

vi.mock("@/shared/api/axiosClient", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/shared/api/axiosClient")>()),
  refreshAccessToken: vi.fn(() => Promise.reject(new Error("no session"))),
}));

vi.mock("@/shared/api/rooms", () => ({
  addBot: vi.fn(),
  removeBot: vi.fn(),
  createRoom: vi.fn(),
}));

vi.mock("@/shared/providers/WebSocketContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/shared/providers/WebSocketContext")>()),
  useWsSendMessage: () => vi.fn(),
  useWsConnectionState: () => "connected" as const,
}));

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
    rankDivision: 1,
    spIntoDivision: 0,
    spForNextDivision: 50,
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

  it("serves /leaderboard from the real router, inside ProtectedRoute and AppLayout", async () => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ id: 7, username: "kiro" }),
      isLoading: false,
    });
    window.history.replaceState(null, "", "/leaderboard");

    render(<App />);

    expect(await screen.findByTestId("leaderboard-page")).toBeInTheDocument();
    expect(screen.getByTestId("app-nav")).toBeInTheDocument();
    expect(screen.getByTestId("nav-leaderboard")).toHaveAttribute("aria-current", "page");
  });

  it("redirects an unauthenticated visitor away from /leaderboard", async () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    window.history.replaceState(null, "", "/leaderboard");

    render(<App />);

    expect(await screen.findByTestId("landing-page")).toBeInTheDocument();
    expect(screen.queryByTestId("leaderboard-page")).not.toBeInTheDocument();
  });

  it("mounts the global sonner Toaster so toasts are visible app-wide", async () => {
    render(<App />);

    expect(await screen.findByRole("region", { name: /notifications/i })).toBeInTheDocument();
  });
});
