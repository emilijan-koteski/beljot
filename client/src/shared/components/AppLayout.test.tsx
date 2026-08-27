import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { AppLayout } from "./AppLayout";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

vi.mock("@/shared/api/profile", () => ({
  updatePreferences: vi.fn().mockResolvedValue({ languagePreference: "en" }),
}));

// The mounted DailyRewardGate fires claimDailyLogin on bootstrap — stub it so
// these layout tests don't reach the network. granted:false → no dialog opens.
vi.mock("@/shared/api/wallet", () => ({
  claimDailyLogin: vi.fn().mockResolvedValue({
    granted: false,
    amount: 0,
    streakDay: 1,
    newBalance: 5000,
    loginStreakDays: 1,
  }),
}));

// AppLayout hosts the always-mounted RoomInviteModal (Story 11.5), which uses
// react-query mutations — so the layout now needs a QueryClient, exactly as it
// has one in production (App wraps RouterProvider in QueryProvider).
function renderWithRouter(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route element={<AppLayout />}>
            <Route path="/lobby" element={<div data-testid="lobby-content">Lobby</div>} />
            <Route
              path="/leaderboard"
              element={<div data-testid="leaderboard-content">Leaderboard</div>}
            />
            <Route path="/profile" element={<div data-testid="profile-content">Profile</div>} />
            <Route path="/rules" element={<div data-testid="rules-content">Rules</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AppLayout", () => {
  beforeEach(() => {
    useAuthStore.setState({
      token: "test-token",
      user: makeUser({ loginStreakDays: 1 }),
      isLoading: false,
    });
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders nav bar with all tabs", () => {
    renderWithRouter("/lobby");

    expect(screen.getByTestId("app-nav")).toBeInTheDocument();
    expect(screen.getByTestId("app-name")).toHaveTextContent("Beljot");
    expect(screen.getByTestId("nav-play")).toHaveTextContent("Play");
    // Story 13.2 added a FOURTH tab. Its test-id derives from the labelKey
    // (`nav.leaderboard` -> `nav-leaderboard`), like every other one.
    expect(screen.getByTestId("nav-leaderboard")).toHaveTextContent("Leaderboard");
    expect(screen.getByTestId("nav-profile")).toHaveTextContent("Profile");
    expect(screen.getByTestId("nav-rules")).toHaveTextContent("Rules");
  });

  it("shows the Beljot logo before the wordmark", () => {
    renderWithRouter("/lobby");

    const logo = screen.getByTestId("app-logo");
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute("src", "/beljot-logo.svg");
  });

  it("highlights active tab for /lobby", () => {
    renderWithRouter("/lobby");

    const playTab = screen.getByTestId("nav-play");
    expect(playTab).toHaveAttribute("aria-current", "page");
  });

  it("highlights active tab for /profile", () => {
    renderWithRouter("/profile");

    const profileTab = screen.getByTestId("nav-profile");
    expect(profileTab).toHaveAttribute("aria-current", "page");
  });

  // AC2: the new tab gets the SAME active styling as the others — it is one more
  // entry in the same data-driven navItems list, not a special case.
  it("highlights active tab for /leaderboard", () => {
    renderWithRouter("/leaderboard");

    expect(screen.getByTestId("nav-leaderboard")).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("nav-play")).not.toHaveAttribute("aria-current");
  });

  it("renders outlet content", () => {
    renderWithRouter("/lobby");

    expect(screen.getByTestId("lobby-content")).toBeInTheDocument();
  });

  it("renders language selector", () => {
    renderWithRouter("/lobby");

    expect(screen.getByTestId("language-selector")).toBeInTheDocument();
  });

  it("displays current user's username and avatar", () => {
    renderWithRouter("/lobby");

    const userButton = screen.getByTestId("nav-user");
    expect(userButton).toBeInTheDocument();
    expect(userButton).toHaveTextContent("T");
    expect(userButton).toHaveTextContent("testuser");
  });

  it("shows dropdown with logout option and logs out on click", async () => {
    const user = userEvent.setup();
    renderWithRouter("/lobby");

    await user.click(screen.getByTestId("nav-user"));

    await waitFor(() => {
      expect(screen.getByTestId("nav-logout")).toBeInTheDocument();
    });
    expect(screen.getByTestId("nav-logout")).toHaveTextContent("Log out");

    await user.click(screen.getByTestId("nav-logout"));

    await waitFor(() => {
      const state = useAuthStore.getState();
      expect(state.user).toBeNull();
      expect(state.token).toBeNull();
    });
  });

  it("does not display user widget when no user is logged in", () => {
    useAuthStore.setState({ user: null, token: null });
    renderWithRouter("/lobby");

    expect(screen.queryByTestId("nav-user")).not.toBeInTheDocument();
  });
});
