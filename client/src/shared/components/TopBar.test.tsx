import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, MemoryRouter, Route, RouterProvider, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TopBar } from "@/shared/components/TopBar";
import { resetLobbyReturnGuardForTests } from "@/shared/hooks/useLobbyReturn";
import { useAuthStore } from "@/shared/stores/authStore";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

function renderWithRouter() {
  return render(
    <MemoryRouter initialEntries={["/lobby"]}>
      <Routes>
        <Route path="/lobby" element={<TopBar showNav showUserMenu />} />
        <Route path="/" element={<div data-testid="landing-page">Landing</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

function setAuthUser(overrides: Partial<import("@/shared/types/apiTypes").User> = {}) {
  useAuthStore.setState({
    token: "test-token",
    user: {
      id: 1,
      username: "kiro",
      email: "kiro@example.com",
      languagePreference: "en",
      walletBalance: 5000,
      loginStreakDays: 1,
      totalXp: 0,
      level: 0,
      createdAt: "2026-01-01T00:00:00Z",
      ...overrides,
    },
    isLoading: false,
  });
}

describe("TopBar logout", () => {
  beforeEach(() => {
    setAuthUser();
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("clears auth state and navigates to the landing page (/) on logout", async () => {
    const user = userEvent.setup();
    renderWithRouter();

    await user.click(screen.getByTestId("nav-user"));
    await waitFor(() => {
      expect(screen.getByTestId("nav-logout")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("nav-logout"));

    await waitFor(() => {
      expect(useAuthStore.getState().token).toBeNull();
      expect(screen.getByTestId("landing-page")).toBeInTheDocument();
    });
  });
});

describe("TopBar coin balance", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the wallet balance from the store, locale-formatted", () => {
    setAuthUser({ walletBalance: 12345 });
    renderWithRouter();

    const pill = screen.getByTestId("coin-balance");
    expect(pill).toHaveTextContent((12345).toLocaleString());
  });

  it("renders correctly at a zero balance", () => {
    setAuthUser({ walletBalance: 0 });
    renderWithRouter();

    expect(screen.getByTestId("coin-balance")).toHaveTextContent("0");
  });

  it("does not render the login streak in the header, even at a high streak", () => {
    // The streak is surfaced in the daily-reward dialog and the profile only —
    // never alongside the header coin balance.
    setAuthUser({ loginStreakDays: 7 });
    renderWithRouter();

    expect(screen.getByTestId("coin-balance")).toBeInTheDocument();
    expect(screen.queryByTestId("login-streak")).not.toBeInTheDocument();
  });
});

describe("TopBar XP level (Story 9.5)", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the level and XP bar from the store", () => {
    // Level 3, 600 XP: band threshold(3)=450 .. threshold(4)=800 (span 350),
    // 150 into the band → round(150/350*100) = 43%.
    setAuthUser({ level: 3, totalXp: 600 });
    renderWithRouter();

    expect(screen.getByTestId("xp-level")).toHaveTextContent("Lvl 3");
    const bar = screen.getByTestId("xp-bar");
    expect(bar).toBeInTheDocument();
    expect(bar).toHaveAttribute("aria-valuenow", "43");
  });

  it("renders Level 0 at an empty bar for a brand-new player", () => {
    setAuthUser({ level: 0, totalXp: 0 });
    renderWithRouter();

    expect(screen.getByTestId("xp-level")).toHaveTextContent("Lvl 0");
    expect(screen.getByTestId("xp-bar")).toHaveAttribute("aria-valuenow", "0");
  });
});

describe("TopBar history-stack shaping", () => {
  beforeEach(() => {
    setAuthUser();
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    sessionStorage.clear();
    resetLobbyReturnGuardForTests();
    window.history.replaceState(null, "");
  });

  // TopBar on every route; distinct landing pads so pathname assertions are
  // unambiguous. Stack starts as [/lobby, <initialPath>].
  function renderAtWithRouter(initialPath: string) {
    const router = createMemoryRouter([{ path: "*", element: <TopBar showNav showUserMenu /> }], {
      initialEntries: ["/lobby", initialPath],
      initialIndex: 1,
    });
    render(<RouterProvider router={router} />);
    return router;
  }

  it("replaces the current entry when navigating between non-lobby pages", async () => {
    const router = renderAtWithRouter("/profile");

    await act(async () => {
      fireEvent.click(screen.getByTestId("nav-rules"));
    });
    expect(router.state.location.pathname).toBe("/rules");

    // /profile was replaced — back skips it and lands on the lobby root.
    await act(async () => {
      await router.navigate(-1);
    });
    expect(router.state.location.pathname).toBe("/lobby");
  });

  it("pops back to the recorded lobby root when the Play link is clicked", async () => {
    const router = renderAtWithRouter("/profile");

    // Simulate the react-router history idx bookkeeping jsdom lacks: the
    // lobby root was recorded at idx 0 and the current entry sits at idx 1.
    window.history.replaceState({ idx: 1 }, "");
    sessionStorage.setItem("beljot:lobby-idx", "0");

    await act(async () => {
      fireEvent.click(screen.getByTestId("nav-play"));
    });
    expect(router.state.location.pathname).toBe("/lobby");

    // A pop keeps /profile in FORWARD history; a push or replace would not.
    await act(async () => {
      await router.navigate(1);
    });
    expect(router.state.location.pathname).toBe("/profile");
  });

  it("falls back to replacing with /lobby when no lobby root is recorded", async () => {
    const router = renderAtWithRouter("/profile");

    await act(async () => {
      fireEvent.click(screen.getByTestId("nav-play"));
    });
    expect(router.state.location.pathname).toBe("/lobby");

    // The /profile entry was replaced — back lands on the original root.
    await act(async () => {
      await router.navigate(-1);
    });
    expect(router.state.location.pathname).toBe("/lobby");
  });

  it("keeps native behavior for modified clicks on the Play link", () => {
    renderAtWithRouter("/profile");

    // fireEvent returns false when preventDefault was called — a ctrl+click
    // must pass through untouched for open-in-new-tab.
    const passedThrough = fireEvent.click(screen.getByTestId("nav-play"), { ctrlKey: true });
    expect(passedThrough).toBe(true);
  });
});
