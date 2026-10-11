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

const mockGetCurrentSeason = vi.fn();
vi.mock("@/shared/api/season", () => ({
  getCurrentSeason: (...args: unknown[]) => mockGetCurrentSeason(...args),
}));

const seasonFixture: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: "2099-10-01T00:00:00Z",
  sp: 680,
  rankTier: "gold",
  rankDivision: 2,
  spIntoDivision: 13,
  spForNextDivision: 67,
  gamesPlayed: 31,
  gamesCompleted: 29,
};

beforeEach(() => {
  mockGetCurrentSeason.mockReset();
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

    await act(async () => {
      await router.navigate(-1);
    });
    expect(router.state.location.pathname).toBe("/lobby");
  });

  it("pops back to the recorded lobby root when the Play link is clicked", async () => {
    const router = renderAtWithRouter("/profile");

    window.history.replaceState({ idx: 1 }, "");
    sessionStorage.setItem("beljot:lobby-idx", "0");

    await act(async () => {
      fireEvent.click(screen.getByTestId("nav-play"));
    });
    expect(router.state.location.pathname).toBe("/lobby");

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

    await act(async () => {
      await router.navigate(-1);
    });
    expect(router.state.location.pathname).toBe("/lobby");
  });

  it("keeps native behavior for modified clicks on the Play link", () => {
    renderAtWithRouter("/profile");

    const passedThrough = fireEvent.click(screen.getByTestId("nav-play"), { ctrlKey: true });
    expect(passedThrough).toBe(true);
  });
});

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

  it("renders the tier badge and the rank once the standing resolves", async () => {
    renderWithRouter();

    const chip = await screen.findByTestId("header-rank");
    expect(chip).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("header-rank-badge")).toHaveAttribute("data-tier", "gold");
    expect(screen.getByTestId("header-rank-tier").textContent).toBe("Gold 2");
  });

  it("names a single tier without a division", async () => {
    mockGetCurrentSeason.mockResolvedValue({
      ...seasonFixture,
      sp: 1250,
      rankTier: "master",
      rankDivision: null,
    });
    renderWithRouter();

    await screen.findByTestId("header-rank");
    expect(screen.getByTestId("header-rank-tier").textContent).toBe("Master");
  });

  it("shows no SP figure at all", async () => {
    renderWithRouter();

    const chip = await screen.findByTestId("header-rank");
    expect(chip.textContent).not.toContain("680");
    expect(chip.textContent).not.toContain("SP");
  });

  it("drops the tier name below sm, leaving the badge alone on phones", async () => {
    renderWithRouter();

    await screen.findByTestId("header-rank");
    const name = screen.getByTestId("header-rank-tier");
    expect(name.className).toContain("hidden");
    expect(name.className).toContain("sm:inline");
    expect(screen.getByTestId("header-rank-badge").className).not.toContain("hidden");
  });

  it("names the rank for assistive tech at every width", async () => {
    renderWithRouter();

    const chip = await screen.findByTestId("header-rank");
    const label = chip.querySelector(".sr-only");
    expect(label?.textContent).toBe("Rank: Gold 2");
  });

  it("falls back to the SP bucket for an unrecognised tier token", async () => {
    mockGetCurrentSeason.mockResolvedValue({ ...seasonFixture, rankTier: "mythic", sp: 700 });
    renderWithRouter();

    expect(await screen.findByTestId("header-rank")).toHaveAttribute("data-tier", "gold");
  });

  it("localizes the rank", async () => {
    await i18n.changeLanguage("mk");
    renderWithRouter();

    await screen.findByTestId("header-rank");
    expect(screen.getByTestId("header-rank-tier").textContent).toBe("Злато 2");
  });

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

describe("TopBar support entry", () => {
  beforeEach(() => {
    setAuthUser();
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("offers support from the desktop user menu, above logout", async () => {
    const user = userEvent.setup();
    renderWithRouter();

    await user.click(screen.getByTestId("nav-user"));
    await waitFor(() => {
      expect(screen.getByTestId("nav-support")).toBeInTheDocument();
    });

    const item = screen.getByTestId("nav-support");
    expect(item).toHaveTextContent("Support Beljot.online");
    expect(item.compareDocumentPosition(screen.getByTestId("nav-logout"))).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it("opens the explainer dialog from the desktop user menu", async () => {
    const user = userEvent.setup();
    renderWithRouter();

    await user.click(screen.getByTestId("nav-user"));
    await waitFor(() => {
      expect(screen.getByTestId("nav-support")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("nav-support"));

    expect(await screen.findByTestId("support-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("support-cta")).toHaveAttribute(
      "href",
      "https://buymeacoffee.com/emilijan",
    );
  });

  it("offers support from the mobile menu too", async () => {
    const user = userEvent.setup();
    renderWithRouter();

    await user.click(screen.getByTestId("nav-menu"));
    await waitFor(() => {
      expect(screen.getByTestId("nav-menu-support")).toBeInTheDocument();
    });
    expect(screen.getByTestId("nav-menu-support")).toHaveTextContent("Support Beljot.online");

    await user.click(screen.getByTestId("nav-menu-support"));
    expect(await screen.findByTestId("support-dialog")).toBeInTheDocument();
  });

  it("adds no support entry to a bar that renders no menus", () => {
    render(
      withQuery(
        <MemoryRouter initialEntries={["/login"]}>
          <TopBar showFullBrand />
        </MemoryRouter>,
      ),
    );

    expect(screen.queryByTestId("nav-menu")).not.toBeInTheDocument();
    expect(screen.queryByTestId("nav-support")).not.toBeInTheDocument();
    expect(screen.queryByTestId("nav-menu-support")).not.toBeInTheDocument();
  });
});

describe("TopBar nav pill avatar", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("shows the initial without an avatar", () => {
    setAuthUser({ avatarUrl: null });
    renderWithRouter();
    const disc = screen.getByTestId("nav-user-avatar");
    expect(disc.querySelector("img")).toBeNull();
    expect(disc).toHaveTextContent("K");
  });

  it("shows the 128 image lazily and falls back to the initial if it fails", () => {
    const url = "https://assets.test/avatars/k/128.webp";
    setAuthUser({ avatarUrl: url });
    renderWithRouter();
    const disc = screen.getByTestId("nav-user-avatar");
    const img = disc.querySelector("img");
    expect(img).toHaveAttribute("src", url);
    expect(img).toHaveAttribute("loading", "lazy");
    expect(img).toHaveAttribute("width", "26");

    fireEvent.error(img!);
    expect(disc.querySelector("img")).toBeNull();
    expect(disc).toHaveTextContent("K");
  });
});
