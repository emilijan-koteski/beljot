import "@/shared/i18n/i18n";

import { QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { createMemoryRouter, MemoryRouter, Route, RouterProvider, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TopBar } from "@/shared/components/TopBar";
import { resetLobbyReturnGuardForTests } from "@/shared/hooks/useLobbyReturn";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import type { CurrentSeasonResponse, User } from "@/shared/types/apiTypes";
import { createTestQueryClient, makeUser } from "@/test-utils";

vi.mock("@/shared/api/auth", () => ({
  logout: vi.fn(),
}));

// The header's rank chip reads GET /seasons/current, so the bar is a fetching
// component now and every render below needs a query client.
const mockGetCurrentSeason = vi.fn();
vi.mock("@/shared/api/season", () => ({
  getCurrentSeason: (...args: unknown[]) => mockGetCurrentSeason(...args),
}));

const seasonFixture: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: "2099-10-01T00:00:00Z",
  sp: 4000,
  rankTier: "gold",
  spIntoTier: 1000,
  spForNextTier: 2500,
  gamesPlayed: 31,
  gamesCompleted: 29,
};

beforeEach(() => {
  mockGetCurrentSeason.mockReset();
  // DEFAULT: never settles, so the chip renders nothing and the bar looks
  // exactly as it did before the rank joined it. That keeps every pre-existing
  // assertion in this file about its own subject, and it is also the honest
  // first frame — the chip has no skeleton by design.
  mockGetCurrentSeason.mockReturnValue(new Promise(() => {}));
});

/** A fresh query client per render() call — never shared across tests. */
function withQuery(ui: ReactNode) {
  return <QueryClientProvider client={createTestQueryClient()}>{ui}</QueryClientProvider>;
}

function renderWithRouter() {
  return render(
    withQuery(
      <MemoryRouter initialEntries={["/lobby"]}>
        <Routes>
          <Route path="/lobby" element={<TopBar showNav showUserMenu />} />
          <Route path="/" element={<div data-testid="landing-page">Landing</div>} />
        </Routes>
      </MemoryRouter>,
    ),
  );
}

function setAuthUser(overrides: Partial<User> = {}) {
  useAuthStore.setState({
    token: "test-token",
    // Routed through the shared makeUser fixture so the next additive User field
    // is a one-line change here too (code review 2026-07-29 — this file was the
    // last one still hand-building the literal).
    user: makeUser({
      username: "kiro",
      email: "kiro@example.com",
      ...overrides,
    }),
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

describe("TopBar honor (Story 9.7)", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("does not render an honor chip in the header", () => {
    // Honor is surfaced on the profile page (HonorHeroBand), not in the header —
    // the chip that used to sit beside the coin pill was removed.
    setAuthUser({ honorScore: 96, honorTier: "exemplary" });
    renderWithRouter();

    expect(screen.queryByTestId("honor-chip")).not.toBeInTheDocument();
    expect(screen.queryByTestId("honor-score")).not.toBeInTheDocument();
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
    render(withQuery(<RouterProvider router={router} />));
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

// Story 13.2 added a fourth tab. navItems feeds BOTH renderers — the >=md link
// row and the mobile dropdown — so a tab that appears in one and not the other
// means someone stopped mapping over the shared list.
describe("TopBar leaderboard tab", () => {
  beforeEach(() => {
    setAuthUser();
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the Leaderboard tab in the desktop nav row, linking to /leaderboard", () => {
    renderWithRouter();

    const tab = screen.getByTestId("nav-leaderboard");
    expect(tab).toHaveTextContent("Leaderboard");
    expect(tab).toHaveAttribute("href", "/leaderboard");
  });

  it("renders the Leaderboard entry in the mobile menu too", async () => {
    const user = userEvent.setup();
    renderWithRouter();

    // The mobile menu is its own dropdown (`nav-menu`), separate from the
    // desktop user pill (`nav-user`).
    await user.click(screen.getByTestId("nav-menu"));
    await waitFor(() => {
      expect(screen.getByTestId("nav-menu-leaderboard")).toBeInTheDocument();
    });
    expect(screen.getByTestId("nav-menu-leaderboard")).toHaveTextContent("Leaderboard");
  });

  it("navigates to /leaderboard when the tab is activated", async () => {
    const router = createMemoryRouter(
      [
        { path: "/lobby", element: <TopBar showNav showUserMenu /> },
        { path: "/leaderboard", element: <div data-testid="leaderboard-page">Leaderboard</div> },
      ],
      { initialEntries: ["/lobby"] },
    );
    render(withQuery(<RouterProvider router={router} />));

    await act(async () => {
      fireEvent.click(screen.getByTestId("nav-leaderboard"));
    });

    expect(router.state.location.pathname).toBe("/leaderboard");
  });
});

describe("TopBar seasonal rank chip", () => {
  beforeEach(() => {
    setAuthUser();
    mockGetCurrentSeason.mockResolvedValue(seasonFixture);
  });

  afterEach(async () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    await i18n.changeLanguage("en");
  });

  it("renders the tier badge and the tier name once the standing resolves", async () => {
    renderWithRouter();

    const chip = await screen.findByTestId("header-rank");
    expect(chip).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("header-rank-badge")).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("header-rank-tier").textContent).toBe(i18n.t("season.tier.gold"));
  });

  // THE POINT OF THE CHIP: identity, not arithmetic. The SP total, the band
  // decomposition and the countdown all live on the profile's RankBanner — a
  // number here would compete with the two the header already carries (level
  // and coin balance).
  it("shows no SP figure at all", async () => {
    renderWithRouter();

    const chip = await screen.findByTestId("header-rank");
    expect(chip.textContent).not.toContain("4,000");
    expect(chip.textContent).not.toContain("4000");
    expect(chip.textContent).not.toContain("SP");
  });

  it("drops the tier name below sm, leaving the badge alone on phones", async () => {
    renderWithRouter();

    await screen.findByTestId("header-rank");
    const name = screen.getByTestId("header-rank-tier");
    expect(name.className).toContain("hidden");
    expect(name.className).toContain("sm:inline");
    // The badge itself carries no responsive gate — it is the phone treatment.
    expect(screen.getByTestId("header-rank-badge").className).not.toContain("hidden");
  });

  // The badge is decorative and the name is hidden on phones, so without the
  // sr-only line the rank would be announced as nothing at all below sm.
  it("names the rank for assistive tech at every width", async () => {
    renderWithRouter();

    const chip = await screen.findByTestId("header-rank");
    const label = chip.querySelector(".sr-only");
    expect(label?.textContent).toBe(
      i18n.t("season.banner.rankAria", { tier: i18n.t("season.tier.gold") }),
    );
  });

  it("falls back to the SP bucket for an unrecognised tier token", async () => {
    // Version skew: a newer server sends a tier this bundle has never heard of.
    mockGetCurrentSeason.mockResolvedValue({ ...seasonFixture, rankTier: "mythic" });
    renderWithRouter();

    // 4000 SP is Gold.
    expect(await screen.findByTestId("header-rank")).toHaveAttribute("data-tier", "gold");
  });

  it("localizes the tier name", async () => {
    await i18n.changeLanguage("mk");
    renderWithRouter();

    await screen.findByTestId("header-rank");
    expect(screen.getByTestId("header-rank-tier").textContent).toBe(i18n.t("season.tier.gold"));
  });

  // AuthLayout mounts this same bar with nobody signed in, and the endpoint
  // behind the chip is auth-only — so the chip must not even be mounted there.
  it("renders no chip and makes no request when signed out", async () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });

    render(
      withQuery(
        <MemoryRouter initialEntries={["/login"]}>
          <TopBar showFullBrand />
        </MemoryRouter>,
      ),
    );

    await waitFor(() => expect(screen.getByTestId("app-brand")).toBeInTheDocument());
    expect(screen.queryByTestId("header-rank")).not.toBeInTheDocument();
    expect(mockGetCurrentSeason).not.toHaveBeenCalled();
  });
});
