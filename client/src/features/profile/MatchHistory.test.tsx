import "@/shared/i18n/i18n";

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  MatchesListResponse,
  MatchFilter,
  MatchHandView,
  MatchListItem,
} from "@/shared/api/matches";
import { QueryWrapper } from "@/test-utils";

import { MatchHistory } from "./MatchHistory";

const mockGetUserMatches = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getUserMatches: (...args: unknown[]) => mockGetUserMatches(...args),
}));

const NONZERO_COUNTS: Record<MatchFilter, number> = { all: 99, win: 50, loss: 40, abandoned: 9 };

function renderMatchHistory(
  userId: number | undefined = 1,
  counts: Record<MatchFilter, number> = NONZERO_COUNTS,
) {
  return render(
    <QueryWrapper>
      <BrowserRouter>
        <MatchHistory userId={userId} counts={counts} />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

function makeHand(overrides: Partial<MatchHandView> = {}): MatchHandView {
  return {
    handNumber: 1,
    teamACardPoints: 60,
    teamBCardPoints: 102,
    teamADeclPoints: 20,
    teamBDeclPoints: 0,
    lastTrickTeam: 1,
    lastTrickBonus: 10,
    capot: false,
    capotTeam: undefined,
    capotBonus: 0,
    failedContract: false,
    contractingTeam: 0,
    teamAHandTotal: 80,
    teamBHandTotal: 112,
    ...overrides,
  };
}

function makeMatch(overrides: Partial<MatchListItem> = {}): MatchListItem {
  return {
    id: 1,
    hasBots: false,
    variant: "bitola",
    matchMode: "1001",
    startedAt: "2026-04-10T12:00:00Z",
    completedAt: "2026-04-10T12:25:00Z",
    status: "completed",
    winnerTeam: 0,
    teamAScore: 1010,
    teamBScore: 640,
    abandonedBy: undefined,
    viewerSeat: 0,
    outcome: "win",
    players: [
      { seat: 0, userId: 10, username: "viewer", isBot: false },
      { seat: 1, userId: 11, username: "opp1", isBot: false },
      { seat: 2, userId: 12, username: "mate", isBot: false },
      { seat: 3, userId: 13, username: "opp2", isBot: false },
    ],
    hands: [
      makeHand({ handNumber: 1 }),
      makeHand({ handNumber: 2, capot: true, capotTeam: 0, capotBonus: 100 }),
    ],
    ...overrides,
  };
}

function makeResponse(
  items: MatchListItem[],
  total: number,
  offset = 0,
  limit = 20,
): MatchesListResponse {
  return { items, total, offset, limit };
}

describe("MatchHistory", () => {
  beforeEach(() => {
    mockGetUserMatches.mockReset();
  });

  it("renders loading skeleton on first mount", () => {
    mockGetUserMatches.mockReturnValue(new Promise(() => {}));
    renderMatchHistory();
    expect(screen.getByTestId("match-history-loading")).toBeInTheDocument();
  });

  it("renders empty state with lobby link when the user has no games", async () => {
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([], 0));
    renderMatchHistory(1, { all: 0, win: 0, loss: 0, abandoned: 0 });
    const empty = await screen.findByTestId("match-history-empty");
    expect(empty).toBeInTheDocument();
    const cta = screen.getByTestId("match-history-empty-cta");
    expect(cta).toHaveAttribute("href", "/lobby");
  });

  it("renders the filtered-empty state (not onboarding) when a filter matches nothing", async () => {
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([], 0));
    // counts.all > 0 → the user has games, just none for this view.
    renderMatchHistory(1, { all: 12, win: 0, loss: 0, abandoned: 0 });
    expect(await screen.findByTestId("match-history-empty-filtered")).toBeInTheDocument();
    expect(screen.queryByTestId("match-history-empty")).not.toBeInTheDocument();
  });

  it("re-queries with outcome + sort params when filter and sort change", async () => {
    mockGetUserMatches.mockResolvedValue(makeResponse([makeMatch()], 1));
    renderMatchHistory();
    await screen.findByTestId("match-history-row");

    // Initial fetch uses outcome "all", sort "new".
    expect(mockGetUserMatches).toHaveBeenLastCalledWith(1, 20, 0, { outcome: "all", sort: "new" });

    fireEvent.click(screen.getByTestId("match-history-filter-win"));
    await waitFor(() => {
      expect(mockGetUserMatches).toHaveBeenLastCalledWith(1, 20, 0, {
        outcome: "win",
        sort: "new",
      });
    });

    fireEvent.click(screen.getByTestId("match-history-sort"));
    await waitFor(() => {
      expect(mockGetUserMatches).toHaveBeenLastCalledWith(1, 20, 0, {
        outcome: "win",
        sort: "old",
      });
    });
  });

  it("shows the bots marker and localized bot names on bot-inclusive rows", async () => {
    mockGetUserMatches.mockResolvedValueOnce(
      makeResponse(
        [
          makeMatch({
            hasBots: true,
            players: [
              { seat: 0, userId: 10, username: "viewer", isBot: false },
              { seat: 1, userId: 0, username: "", isBot: true },
              { seat: 2, userId: 12, username: "mate", isBot: false },
              { seat: 3, userId: 0, username: "", isBot: true },
            ],
          }),
        ],
        1,
      ),
    );
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    expect(within(row).getByTestId("match-history-bots-marker")).toBeInTheDocument();
    // Opponent seats 1 and 3 are bots — seat-derived localized names, with
    // the bot glyph (not a name initial) in the chip avatars.
    expect(within(row).getByText("Bot 2")).toBeInTheDocument();
    expect(within(row).getByText("Bot 4")).toBeInTheDocument();
    expect(within(row).getAllByTestId("avatar-icon")).toHaveLength(2);
  });

  it("hides the bots marker on human-only rows", async () => {
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([makeMatch()], 1));
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    expect(within(row).queryByTestId("match-history-bots-marker")).not.toBeInTheDocument();
  });

  it("renders rows with teammate, opponents, outcome and viewer-first score", async () => {
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([makeMatch()], 1));
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    expect(within(row).getByText("mate")).toBeInTheDocument();
    expect(within(row).getByText(/opp1/)).toBeInTheDocument();
    expect(within(row).getByText(/opp2/)).toBeInTheDocument();
    // Viewer is on teamA → "Us" first (1010), "Them" second (640)
    expect(within(row).getByText("1010")).toBeInTheDocument();
    expect(within(row).getByText("640")).toBeInTheDocument();
    expect(within(row).getByTestId("match-history-outcome")).toHaveAttribute("data-outcome", "win");
  });

  it("flips score order when viewer was on teamB (viewerSeat=1)", async () => {
    mockGetUserMatches.mockResolvedValueOnce(
      makeResponse([makeMatch({ viewerSeat: 1, teamAScore: 200, teamBScore: 800 })], 1),
    );
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    // Viewer on teamB → Us=800 (teamB score) appears first as data-team="teamB"
    const scoreSpans = within(row)
      .getAllByText(/^(800|200)$/)
      .map((el) => ({ text: el.textContent, team: el.getAttribute("data-team") }));
    // First score span should be the viewer's team
    expect(scoreSpans[0]).toEqual({ text: "800", team: "teamB" });
    expect(scoreSpans[1]).toEqual({ text: "200", team: "teamA" });
  });

  it("shows all three outcome variants", async () => {
    const matches = [
      makeMatch({ id: 1, outcome: "win" }),
      makeMatch({ id: 2, outcome: "loss" }),
      makeMatch({ id: 3, outcome: "abandoned" }),
    ];
    mockGetUserMatches.mockResolvedValueOnce(makeResponse(matches, 3));
    renderMatchHistory();
    await waitFor(() => {
      expect(screen.getAllByTestId("match-history-row")).toHaveLength(3);
    });
    const outcomes = screen.getAllByTestId("match-history-outcome");
    expect(outcomes.map((el) => el.getAttribute("data-outcome"))).toEqual([
      "win",
      "loss",
      "abandoned",
    ]);
  });

  it("toggles detail view on row click with correct aria state", async () => {
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([makeMatch()], 1));
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    const toggle = within(row).getByRole("button");

    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("match-history-detail")).not.toBeInTheDocument();

    fireEvent.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const detail = screen.getByTestId("match-history-detail");
    expect(detail).toBeInTheDocument();
    expect(within(detail).getAllByTestId("match-history-hand-row")).toHaveLength(2);

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("match-history-detail")).not.toBeInTheDocument();
  });

  it("renders Capot badge and failed-contract pill — viewer was on the contracting team → 'Us'", async () => {
    const match = makeMatch({
      hands: [
        makeHand({
          handNumber: 1,
          failedContract: true,
          contractingTeam: 0,
          teamACardPoints: 0,
          teamAHandTotal: 0,
          teamBCardPoints: 162,
          teamBHandTotal: 182,
        }),
        makeHand({
          handNumber: 2,
          capot: true,
          capotTeam: 1,
          capotBonus: 100,
          teamBCardPoints: 162,
          teamBHandTotal: 312,
          teamACardPoints: 0,
          teamAHandTotal: 0,
        }),
      ],
    });
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([match], 1));
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    fireEvent.click(within(row).getByRole("button"));

    expect(screen.getByTestId("match-history-hand-capot")).toBeInTheDocument();
    // Viewer is at seat 0 (teamA, index 0); contractingTeam is 0 → "us" side failed.
    const failed = screen.getByTestId("match-history-hand-failed");
    expect(failed).toHaveTextContent(/us/);
    expect(failed).not.toHaveTextContent(/them/);
  });

  it("renders failed-contract pill as 'Them' when viewer was NOT on the contracting team", async () => {
    const match = makeMatch({
      viewerSeat: 1, // teamB
      hands: [
        makeHand({
          handNumber: 1,
          failedContract: true,
          contractingTeam: 0, // teamA contracted; viewer is teamB → Them
          teamACardPoints: 0,
          teamAHandTotal: 0,
          teamBCardPoints: 162,
          teamBHandTotal: 182,
        }),
      ],
    });
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([match], 1));
    renderMatchHistory();
    const row = await screen.findByTestId("match-history-row");
    fireEvent.click(within(row).getByRole("button"));

    // Viewer is teamB; teamA contracted and failed → "them" side failed.
    const failed = screen.getByTestId("match-history-hand-failed");
    expect(failed).toHaveTextContent(/them/);
  });

  it("shows Load more when total > loaded and appends rows on click", async () => {
    const firstPage = makeResponse([makeMatch({ id: 1 })], 2, 0, 20);
    const secondPage = makeResponse([makeMatch({ id: 2 })], 2, 1, 20);

    mockGetUserMatches.mockResolvedValueOnce(firstPage).mockResolvedValueOnce(secondPage);

    renderMatchHistory();

    await waitFor(() => {
      expect(screen.getAllByTestId("match-history-row")).toHaveLength(1);
    });

    const loadMore = await screen.findByTestId("match-history-load-more");
    fireEvent.click(loadMore);

    await waitFor(() => {
      expect(screen.getAllByTestId("match-history-row")).toHaveLength(2);
    });

    expect(screen.queryByTestId("match-history-load-more")).not.toBeInTheDocument();
  });

  it("hides Load more when all rows are loaded", async () => {
    mockGetUserMatches.mockResolvedValueOnce(makeResponse([makeMatch()], 1));
    renderMatchHistory();
    await screen.findByTestId("match-history-row");
    expect(screen.queryByTestId("match-history-load-more")).not.toBeInTheDocument();
  });

  it("shows Load more button when there are more matches to fetch", async () => {
    mockGetUserMatches.mockResolvedValueOnce(
      makeResponse([makeMatch({ id: 1 }), makeMatch({ id: 2 })], 10, 0, 20),
    );
    renderMatchHistory();
    await screen.findAllByTestId("match-history-row");
    expect(screen.getByTestId("match-history-load-more")).toBeInTheDocument();
  });

  it("does not fetch when userId is undefined", () => {
    render(
      <QueryWrapper>
        <BrowserRouter>
          <MatchHistory userId={undefined} counts={NONZERO_COUNTS} />
        </BrowserRouter>
      </QueryWrapper>,
    );
    expect(mockGetUserMatches).not.toHaveBeenCalled();
  });
});
