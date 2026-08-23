import "@/shared/i18n/i18n";

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import type { MatchHandView, MatchListItem } from "@/shared/api/matches";

import { MatchStatsCard } from "./MatchStatsCard";

function makeHand(overrides: Partial<MatchHandView> = {}): MatchHandView {
  return {
    handNumber: 1,
    teamACardPoints: 90,
    teamBCardPoints: 72,
    teamADeclPoints: 20,
    teamBDeclPoints: 0,
    lastTrickTeam: 0,
    lastTrickBonus: 10,
    capot: false,
    capotTeam: undefined,
    capotBonus: 0,
    failedContract: false,
    contractingTeam: 0,
    teamAHandTotal: 120,
    teamBHandTotal: 72,
    ...overrides,
  };
}

function makeMatch(overrides: Partial<MatchListItem> = {}): MatchListItem {
  return {
    id: 7,
    variant: "bitola",
    matchMode: "1001",
    startedAt: "2026-08-01T12:00:00Z",
    completedAt: "2026-08-01T12:25:00Z",
    status: "completed",
    winnerTeam: 0,
    teamAScore: 1010,
    teamBScore: 640,
    hasBots: false,
    viewerSeat: 0,
    outcome: "win",
    endReason: "natural",
    players: [
      { seat: 0, userId: 10, username: "viewer", isBot: false },
      { seat: 1, userId: 11, username: "opp1", isBot: false },
      { seat: 2, userId: 12, username: "mate", isBot: false },
      { seat: 3, userId: 13, username: "opp2", isBot: false },
    ],
    hands: [makeHand({ handNumber: 1 }), makeHand({ handNumber: 2 })],
    ...overrides,
  };
}

function renderCard(props: Partial<Parameters<typeof MatchStatsCard>[0]> = {}) {
  const onToggle = props.onToggle ?? vi.fn();
  const result = render(
    <BrowserRouter>
      <ul>
        <MatchStatsCard
          match={props.match ?? makeMatch()}
          isOpen={props.isOpen ?? false}
          onToggle={onToggle}
          subjectIsSelf={props.subjectIsSelf}
          handsLayout={props.handsLayout}
          linkPlayers={props.linkPlayers}
          footer={props.footer}
        />
      </ul>
    </BrowserRouter>,
  );
  return { ...result, onToggle };
}

describe("MatchStatsCard", () => {
  it("renders the header with the viewer-relative score and outcome", () => {
    renderCard();
    const row = screen.getByTestId("match-history-row");
    expect(row).toHaveAttribute("data-match-id", "7");
    expect(screen.getByTestId("match-history-outcome")).toHaveAttribute("data-outcome", "win");
    // Viewer sits on team A, so their score leads.
    const scores = within(row).getAllByText(/^(1010|640)$/);
    expect(scores[0]).toHaveTextContent("1010");
  });

  it("derives the subject's seat chip from viewerSeat", () => {
    renderCard({ match: makeMatch({ viewerSeat: 1 }) });
    const chips = screen.getAllByTestId("match-seat-chip");
    // First chip is the subject's own — seat 1 here, not the seat-0 default.
    expect(chips[0]).toHaveTextContent("opp1");
  });

  it("hides the hand breakdown until it is open", async () => {
    const { onToggle } = renderCard({ isOpen: false });
    expect(screen.queryByTestId("match-history-detail")).toBeNull();
    await userEvent.click(screen.getByTestId("match-history-row-header"));
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("renders the hand breakdown when open", () => {
    renderCard({ isOpen: true });
    expect(screen.getByTestId("match-history-detail")).toBeInTheDocument();
    expect(screen.getAllByTestId("match-history-hand-row")).toHaveLength(2);
  });

  it("fires onToggle exactly once when the chevron is pressed", async () => {
    const { onToggle } = renderCard({ isOpen: true });
    await userEvent.click(screen.getByRole("button", { name: /hide hand details/i }));
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("forces the stacked hand layout with handsLayout='stacked'", () => {
    // jsdom reports the desktop viewport, so "auto" picks the wide table; the
    // stacked override is what keeps the card inside a narrow overlay panel.
    renderCard({ isOpen: true, handsLayout: "auto" });
    expect(screen.getByTestId("match-history-hands-grid").className).toContain("overflow-x-auto");

    renderCard({ isOpen: true, handsLayout: "stacked" });
    const grids = screen.getAllByTestId("match-history-hands-grid");
    expect(grids).toHaveLength(2);
    expect(grids[1]?.className).not.toContain("overflow-x-auto");
  });

  it("renders the footer inside the expanded detail, below the hands", () => {
    renderCard({ isOpen: true, footer: <div data-testid="card-footer">actions</div> });
    const detail = screen.getByTestId("match-history-detail");
    const footer = within(detail).getByTestId("card-footer");
    expect(screen.getByTestId("match-history-hands-grid").compareDocumentPosition(footer)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it("renders no footer slot when none is given", () => {
    renderCard({ isOpen: true });
    expect(screen.queryByTestId("card-footer")).toBeNull();
  });

  it("renders bot seats as localized bot names with no profile link", () => {
    renderCard({
      match: makeMatch({
        hasBots: true,
        players: [
          { seat: 0, userId: 10, username: "viewer", isBot: false },
          { seat: 1, userId: 0, username: "", isBot: true },
          { seat: 2, userId: 12, username: "mate", isBot: false },
          { seat: 3, userId: 0, username: "", isBot: true },
        ],
      }),
    });
    expect(screen.getByTestId("match-history-bots-marker")).toBeInTheDocument();
    const links = screen.getAllByRole("link");
    // Only the human teammate is linkable; both bot seats and the subject are not.
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveTextContent("mate");
  });
  // P2: inside the match overlay a seat-chip <Link> would PUSH-navigate away,
  // unmounting MatchPage and skipping return-to-room / leave-room entirely.
  it("renders seat chips as plain text when linkPlayers is false", () => {
    renderCard({ linkPlayers: false });
    expect(screen.queryAllByRole("link")).toHaveLength(0);
    // The names are still there — only the navigation is gone.
    expect(screen.getAllByTestId("match-seat-chip")).toHaveLength(4);
    expect(screen.getByText("mate")).toBeInTheDocument();
  });

  it("links seat chips to player profiles by default", () => {
    renderCard();
    // Three co-players; the subject's own chip is never a link.
    expect(screen.getAllByRole("link")).toHaveLength(3);
  });

  it("falls back to the same em-dash as every other seat for a nameless subject", () => {
    renderCard({
      match: makeMatch({
        players: [
          { seat: 0, userId: 10, username: "", isBot: false },
          { seat: 1, userId: 11, username: "opp1", isBot: false },
          { seat: 2, userId: 12, username: "mate", isBot: false },
          { seat: 3, userId: 13, username: "opp2", isBot: false },
        ],
      }),
    });
    const chips = screen.getAllByTestId("match-seat-chip");
    expect(chips[0]).toHaveTextContent("—");
  });
});
