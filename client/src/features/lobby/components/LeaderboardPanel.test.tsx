import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LeaderboardPanel } from "@/features/lobby/components/LeaderboardPanel";
import { getSeasonLeaderboard } from "@/shared/api/season";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import type { LeaderboardResponse } from "@/shared/types/apiTypes";
import { makeUser } from "@/test-utils";

vi.mock("@/shared/api/season", () => ({
  getSeasonLeaderboard: vi.fn(),
}));

const mockGet = vi.mocked(getSeasonLeaderboard);

function page(overrides: Partial<LeaderboardResponse> = {}): LeaderboardResponse {
  return {
    items: [
      { position: 1, userId: 1, username: "ada", sp: 9000, tier: "diamond", gamesPlayed: 40 },
      { position: 2, userId: 2, username: "kiro", sp: 4000, tier: "gold", gamesPlayed: 31 },
      { position: 3, userId: 3, username: "mira", sp: 100, tier: "iron", gamesPlayed: 2 },
    ],
    total: 3,
    limit: 10,
    offset: 0,
    viewer: { position: 2, userId: 2, sp: 4000, tier: "gold", gamesPlayed: 31 },
    ...overrides,
  };
}

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
  return { qc, ...render(<LeaderboardPanel />, { wrapper }) };
}

describe("LeaderboardPanel", () => {
  beforeEach(() => {
    mockGet.mockReset();
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 2, username: "kiro" }),
      isLoading: false,
    });
  });

  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  it("shows a loading line while the query is in flight", () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    renderPanel();
    const loading = screen.getByTestId("leaderboard-panel-loading");
    expect(loading).toBeInTheDocument();
    // Announced rather than a silent visual change.
    expect(loading).toHaveAttribute("role", "status");
    expect(loading).toHaveAttribute("aria-busy", "true");
  });

  it("renders the top rows with position, username and SP", async () => {
    mockGet.mockResolvedValue(page());
    renderPanel();

    const rows = await screen.findAllByTestId("leaderboard-row");
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveAttribute("data-user-id", "1");
    expect(rows[0]).toHaveTextContent("ada");
    expect(rows[0]).toHaveTextContent((9000).toLocaleString());
    expect(rows[0]!.querySelector('[data-testid="leaderboard-position"]')).toHaveTextContent("1");
  });

  it("requests exactly ten rows from offset zero", async () => {
    mockGet.mockResolvedValue(page());
    renderPanel();

    await screen.findAllByTestId("leaderboard-row");
    expect(mockGet).toHaveBeenCalledWith(10, 0);
  });

  it("marks the viewer's own row and leaves the others unmarked", async () => {
    mockGet.mockResolvedValue(page());
    renderPanel();

    const rows = await screen.findAllByTestId("leaderboard-row");
    // The server's viewer block names user 2.
    expect(rows[1]).toHaveAttribute("data-self", "true");
    expect(rows[0]).not.toHaveAttribute("data-self");
    expect(rows[2]).not.toHaveAttribute("data-self");
  });

  // P2: ONE RULE FOR "IS THIS ME", shared with LeaderboardPage. This panel used
  // to mark self from the auth store's id, a second contract on the same
  // question — so a player whose row exists but who has NO server standing (0 SP,
  // or a stale token on a deleted account) saw a highlighted row here and an
  // unmarked one on the full page, one click away.
  it("marks nothing when the server sends no viewer, even if the auth id is in the list", async () => {
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 2, username: "kiro" }),
      isLoading: false,
    });
    // User 2 IS in items, but the server reports no standing for them.
    mockGet.mockResolvedValue(page({ viewer: null }));
    renderPanel();

    const rows = await screen.findAllByTestId("leaderboard-row");
    expect(rows.some((r) => r.getAttribute("data-user-id") === "2")).toBe(true);
    for (const row of rows) {
      expect(row).not.toHaveAttribute("data-self");
    }
    expect(screen.queryByTestId("leaderboard-you")).not.toBeInTheDocument();
  });

  // And the converse: the server's viewer decides even when it disagrees with the
  // auth store, which is what makes it the single source rather than a hint.
  it("follows the server's viewer id over the auth store's", async () => {
    useAuthStore.setState({
      token: "t",
      user: makeUser({ id: 1, username: "ada" }),
      isLoading: false,
    });
    mockGet.mockResolvedValue(page()); // viewer says userId 2
    renderPanel();

    const rows = await screen.findAllByTestId("leaderboard-row");
    expect(rows[1]).toHaveAttribute("data-self", "true");
    expect(rows[0]).not.toHaveAttribute("data-self");
  });

  it("renders each row's tier badge with the server's tier", async () => {
    mockGet.mockResolvedValue(page());
    renderPanel();

    const badges = await screen.findAllByTestId("leaderboard-tier-badge");
    expect(badges[0]).toHaveAttribute("data-tier", "diamond");
    expect(badges[1]).toHaveAttribute("data-tier", "gold");
  });

  it("falls back to the SP bucket for a tier token this bundle does not know", async () => {
    mockGet.mockResolvedValue(
      page({
        items: [
          { position: 1, userId: 1, username: "ada", sp: 4000, tier: "mythic", gamesPlayed: 1 },
        ],
        total: 1,
        viewer: null,
      }),
    );
    renderPanel();

    const badge = await screen.findByTestId("leaderboard-tier-badge");
    // 4000 SP is Gold — the row still renders rather than showing a raw key.
    expect(badge).toHaveAttribute("data-tier", "gold");
  });

  it("links to the full leaderboard page", async () => {
    mockGet.mockResolvedValue(page());
    renderPanel();

    expect(screen.getByTestId("leaderboard-panel-view-all")).toHaveAttribute(
      "href",
      "/leaderboard",
    );
  });

  it("shows an empty state when no one has SP yet", async () => {
    mockGet.mockResolvedValue(page({ items: [], total: 0, viewer: null }));
    renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("leaderboard-panel-empty")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("leaderboard-row")).not.toBeInTheDocument();
  });

  it("shows an error state when the first request fails", async () => {
    mockGet.mockRejectedValue(new Error("boom"));
    renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("leaderboard-panel-error")).toBeInTheDocument();
    });
  });

  // P3: THE POLL. This widget refetches every 60s, so `isError` goes true on any
  // single transient failure — and checking it before `data` replaced a populated
  // top ten with one line of error text until the next tick.
  it("keeps the rows when a refetch fails, reporting the failure inline", async () => {
    mockGet.mockResolvedValueOnce(page());
    const { qc } = renderPanel();

    const rows = await screen.findAllByTestId("leaderboard-row");
    expect(rows).toHaveLength(3);

    // The poll fires and fails.
    mockGet.mockRejectedValue(new Error("transient"));
    await qc.refetchQueries();

    await waitFor(() => {
      expect(screen.getByTestId("leaderboard-panel-stale")).toBeInTheDocument();
    });
    // The rows the reader already had are still there...
    expect(screen.getAllByTestId("leaderboard-row")).toHaveLength(3);
    // ...and the destructive error branch did NOT take over.
    expect(screen.queryByTestId("leaderboard-panel-error")).not.toBeInTheDocument();
  });

  it("renders localized copy", async () => {
    await i18n.changeLanguage("mk");
    mockGet.mockResolvedValue(page({ items: [], total: 0, viewer: null }));
    renderPanel();

    await waitFor(() => {
      expect(screen.getByTestId("leaderboard-panel-empty").textContent).toBe(
        i18n.t("season.leaderboard.empty"),
      );
    });
    await i18n.changeLanguage("en");
  });
});
