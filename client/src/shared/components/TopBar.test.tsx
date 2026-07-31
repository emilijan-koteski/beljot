import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, MemoryRouter, Route, RouterProvider, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TopBar } from "@/shared/components/TopBar";
import { resetLobbyReturnGuardForTests } from "@/shared/hooks/useLobbyReturn";
import { useAuthStore } from "@/shared/stores/authStore";
import type { User } from "@/shared/types/apiTypes";
import { makeUser } from "@/test-utils";

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

function setAuthUser(overrides: Partial<User> = {}) {
  useAuthStore.setState({
    token: "test-token",
    // Routed through the shared makeUser fixture so the next additive User field
    // is a one-line change here too (code review 2026-07-29 — this file was the
    // last one still hand-building the literal). The honor defaults describe an
    // established player so the chip renders its numeric branch; New Player
    // suppression is exercised explicitly below.
    user: makeUser({
      username: "kiro",
      email: "kiro@example.com",
      honorScore: 90,
      honorTier: "trusted",
      isNewPlayer: false,
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

describe("TopBar honor chip (Story 9.7)", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("renders the score and tier from the store", () => {
    setAuthUser({ honorScore: 96, honorTier: "exemplary" });
    renderWithRouter();

    expect(screen.getByTestId("honor-score")).toHaveTextContent("96");
    expect(screen.getByTestId("honor-tier")).toHaveTextContent("Exemplary");
    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-tier", "exemplary");
  });

  it("renders a zero score as a real 0, not as a fallback", () => {
    // Go zero values serialize as real values — `honorScore || 80` would
    // silently promote a Problematic player to Fair.
    setAuthUser({ honorScore: 0, honorTier: "problematic" });
    renderWithRouter();

    expect(screen.getByTestId("honor-score")).toHaveTextContent("0");
    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-tier", "problematic");
  });

  it("suppresses the number for a New Player", () => {
    setAuthUser({ honorScore: 80, honorTier: "fair", isNewPlayer: true });
    renderWithRouter();

    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-new-player", "true");
    expect(screen.getByTestId("honor-new-player")).toHaveTextContent("New Player");
    expect(screen.queryByTestId("honor-score")).not.toBeInTheDocument();
  });

  it("falls back to the score's own band for an unknown tier token", () => {
    setAuthUser({ honorScore: 55, honorTier: "legendary" });
    renderWithRouter();

    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-tier", "unreliable");
  });

  it("live-updates when event:honor_updated writes a new score to the store", () => {
    setAuthUser({ honorScore: 90, honorTier: "trusted" });
    renderWithRouter();
    expect(screen.getByTestId("honor-score")).toHaveTextContent("90");

    // Exactly what the useWsDispatch honor handler does.
    act(() => {
      const current = useAuthStore.getState().user!;
      useAuthStore
        .getState()
        .setUser({ ...current, honorScore: 73, honorTier: "fair", isNewPlayer: false });
    });

    expect(screen.getByTestId("honor-score")).toHaveTextContent("73");
    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-tier", "fair");
  });

  it("renders no honor chip when signed out", () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    renderWithRouter();

    expect(screen.queryByTestId("honor-chip")).not.toBeInTheDocument();
  });

  // jsdom applies no Tailwind CSS, so the class list is the only thing a unit
  // test can assert here. It is worth asserting: the chip shipped as
  // `hidden ... sm:flex`, which made honor invisible on every phone while the
  // coin pill beside it had no breakpoint gate at all (code review 2026-07-29).
  it("stays visible at phone widths, like the coin pill beside it", () => {
    setAuthUser({ honorScore: 96, honorTier: "exemplary" });
    renderWithRouter();

    const chip = screen.getByTestId("honor-chip");
    expect(chip.className).not.toMatch(/\bhidden\b/);
    expect(chip.className).toMatch(/\bflex\b/);

    // The coin pill is the reference: same unconditional visibility.
    expect(screen.getByTestId("coin-balance").className).not.toMatch(/\bhidden\b/);
  });

  it("names the chip for assistive tech instead of announcing a bare number", () => {
    setAuthUser({ honorScore: 96, honorTier: "exemplary" });
    renderWithRouter();

    // Without the label, a screen reader reads "96 Exemplary" next to the coin
    // balance with no indication that it is an honor score.
    expect(screen.getByTestId("honor-chip")).toHaveTextContent("Honor");
  });

  it("renders the 80 prior rather than a blank danger chip when the score is absent", () => {
    // A bundle newer than the server: the refresh envelope has no honorScore, and
    // unlike the WS payload that HTTP path is not type-guarded. NaN used to fall
    // through every tier floor to "problematic".
    setAuthUser({ honorScore: undefined as unknown as number, honorTier: "" });
    renderWithRouter();

    expect(screen.getByTestId("honor-score")).toHaveTextContent("80");
    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-tier", "fair");
  });

  it("suppresses the score when isNewPlayer is absent rather than showing a confident one", () => {
    // Same version-skew input as above, but for the flag. `undefined` used to be
    // falsy and take the NUMERIC branch, so a server that had not shipped the
    // honor fields showed every account a confident 80/"Fair" — newcomers
    // included. Absent must mean suppressed (review pass 2).
    setAuthUser({ isNewPlayer: undefined as unknown as boolean });
    renderWithRouter();

    expect(screen.getByTestId("honor-chip")).toHaveAttribute("data-new-player", "true");
    expect(screen.getByTestId("honor-new-player")).toBeInTheDocument();
    expect(screen.queryByTestId("honor-score")).not.toBeInTheDocument();
  });

  // "New Player" / "Нов играч" is the widest content this chip can hold, and it
  // applies to every account's first five matches. Measured in the 2026-07-29 E2E
  // pass: while the words were visible from sm up they WRAPPED to a second line
  // in the 768..1023px band (chip 88x50px inside a 30px row of pills) and pushed
  // the whole nav into horizontal scroll. The label is sr-only below lg — the
  // shield alone carries the state — which also matches the scored branch, whose
  // tier word is sr-only at every width. Pinned because the class list is the
  // only thing preventing the regression, and jsdom cannot measure the wrap.
  it("keeps the New Player words sr-only below lg so the chip cannot wrap", () => {
    setAuthUser({ isNewPlayer: true });
    renderWithRouter();

    const label = screen.getByTestId("honor-new-player");
    expect(label).toHaveClass("sr-only");
    expect(label).toHaveClass("lg:not-sr-only");
    expect(label.className).not.toMatch(/\bsm:not-sr-only\b/);
    // Still announced at every width, so nothing is lost below lg.
    expect(label).toHaveTextContent("New Player");
    expect(screen.getByTestId("honor-chip")).toHaveClass("whitespace-nowrap");
  });

  it("reveals the tier word from lg up, and only from lg", () => {
    setAuthUser({ honorScore: 96, honorTier: "exemplary", isNewPlayer: false });
    renderWithRouter();

    // The redesign makes the tier VISIBLE on desktop — hiding it in a tooltip is
    // why a declining score used to look identical to a healthy one, which defeats
    // the point of having five tiers.
    //
    // But it takes the same lg gate as the "New Player" words above, and for the
    // same measured reason: at 640..1023px the row already carries nav links and
    // the username pill, and adding a word there made the chip wrap onto a second
    // line inside a 30px row. Below lg the shield's tone AND SHAPE carry the tier
    // (HonorShield varies the glyph), so nothing is lost visually and the sr-only
    // text keeps the accessible reading identical at every width.
    const tier = screen.getByTestId("honor-tier");
    expect(tier).toHaveClass("sr-only");
    expect(tier).toHaveClass("lg:not-sr-only");
    expect(tier.className).not.toMatch(/\bsm:not-sr-only\b/);
    expect(tier).toHaveTextContent("Exemplary");
    expect(screen.getByTestId("honor-chip")).toHaveClass("whitespace-nowrap");
  });

  it("makes the chip a button that opens the explainer", async () => {
    setAuthUser({ honorScore: 96, honorTier: "exemplary", isNewPlayer: false });
    renderWithRouter();

    // Nothing in the product said what honour measures; the chip is the surface
    // every signed-in player sees, so it is the entry point.
    const chip = screen.getByTestId("honor-chip");
    expect(chip.tagName).toBe("BUTTON");
    expect(screen.queryByTestId("honor-explainer")).not.toBeInTheDocument();

    await userEvent.click(chip);
    expect(await screen.findByTestId("honor-explainer")).toBeInTheDocument();
  });

  it("varies the shield glyph with the tier, so colour is never the only signal", () => {
    setAuthUser({ honorScore: 20, honorTier: "problematic", isNewPlayer: false });
    renderWithRouter();

    expect(screen.getByTestId("honor-shield")).toHaveAttribute("data-tier", "problematic");
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
