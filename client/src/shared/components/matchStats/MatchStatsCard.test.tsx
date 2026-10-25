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
      { seat: 0, userId: 10, username: "viewer", avatarUrl: null, isBot: false },
      { seat: 1, userId: 11, username: "opp1", avatarUrl: null, isBot: false },
      { seat: 2, userId: 12, username: "mate", avatarUrl: null, isBot: false },
      { seat: 3, userId: 13, username: "opp2", avatarUrl: null, isBot: false },
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
    const scores = within(row).getAllByText(/^(1010|640)$/);
    expect(scores[0]).toHaveTextContent("1010");
  });

  it("derives the subject's seat chip from viewerSeat", () => {
    renderCard({ match: makeMatch({ viewerSeat: 1 }) });
    const chips = screen.getAllByTestId("match-seat-chip");
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
          { seat: 0, userId: 10, username: "viewer", avatarUrl: null, isBot: false },
          { seat: 1, userId: 0, username: "", avatarUrl: null, isBot: true },
          { seat: 2, userId: 12, username: "mate", avatarUrl: null, isBot: false },
          { seat: 3, userId: 0, username: "", avatarUrl: null, isBot: true },
        ],
      }),
    });
    expect(screen.getByTestId("match-history-bots-marker")).toBeInTheDocument();
    const links = screen.getAllByRole("link");
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveTextContent("mate");
  });
  it("renders seat chips as plain text when linkPlayers is false", () => {
    renderCard({ linkPlayers: false });
    expect(screen.queryAllByRole("link")).toHaveLength(0);
    expect(screen.getAllByTestId("match-seat-chip")).toHaveLength(4);
    expect(screen.getByText("mate")).toBeInTheDocument();
  });

  it("links seat chips to player profiles by default", () => {
    renderCard();
    expect(screen.getAllByRole("link")).toHaveLength(3);
  });

  it("falls back to the same em-dash as every other seat for a nameless subject", () => {
    renderCard({
      match: makeMatch({
        players: [
          { seat: 0, userId: 10, username: "", avatarUrl: null, isBot: false },
          { seat: 1, userId: 11, username: "opp1", avatarUrl: null, isBot: false },
          { seat: 2, userId: 12, username: "mate", avatarUrl: null, isBot: false },
          { seat: 3, userId: 13, username: "opp2", avatarUrl: null, isBot: false },
        ],
      }),
    });
    const chips = screen.getAllByTestId("match-seat-chip");
    expect(chips[0]).toHaveTextContent("—");
  });

  describe("unfinished hand (stop at target)", () => {
    it("accounts for points the scored hands do not cover", () => {
      const match = makeMatch({
        teamAScore: 119,
        teamBScore: 516,
        winnerTeam: 1,
        outcome: "loss",
        hands: [
          makeHand({ handNumber: 1, teamAHandTotal: 0, teamBHandTotal: 182 }),
          makeHand({ handNumber: 2, teamAHandTotal: 14, teamBHandTotal: 148 }),
          makeHand({ handNumber: 3, teamAHandTotal: 105, teamBHandTotal: 77 }),
        ],
      });

      renderCard({ match, isOpen: true });

      const row = screen.getByTestId("match-history-unscored-hand");
      expect(row).toHaveTextContent("109");
      expect(row).toHaveTextContent("04");
    });

    it("renders nothing when the scored hands already sum to the final score", () => {
      const match = makeMatch({
        teamAScore: 120,
        teamBScore: 72,
        hands: [makeHand({ handNumber: 1, teamAHandTotal: 120, teamBHandTotal: 72 })],
      });

      renderCard({ match, isOpen: true });

      expect(screen.queryByTestId("match-history-unscored-hand")).not.toBeInTheDocument();
    });
  });

  describe("rule chips", () => {
    it("names the rules when they are not the defaults", () => {
      renderCard({
        match: makeMatch({ declarationsEnabled: false, stopAtTarget: true }),
      });

      const decl = screen.getByTestId("match-history-no-declarations");
      const stop = screen.getByTestId("match-history-stop-at-target");
      expect(decl).toHaveAccessibleName(/without declarations and without Belote/i);
      expect(stop).toHaveAccessibleName(/without finishing the hand/i);
    });

    it("stays unchipped on a default match, and on one from a server without the columns", () => {
      renderCard({ match: makeMatch({ declarationsEnabled: true, stopAtTarget: false }) });
      expect(screen.queryByTestId("match-history-no-declarations")).not.toBeInTheDocument();
      expect(screen.queryByTestId("match-history-stop-at-target")).not.toBeInTheDocument();

      const legacy = makeMatch();
      delete (legacy as { declarationsEnabled?: boolean }).declarationsEnabled;
      delete (legacy as { stopAtTarget?: boolean }).stopAtTarget;
      renderCard({ match: legacy });
      expect(screen.queryByTestId("match-history-no-declarations")).not.toBeInTheDocument();
      expect(screen.queryByTestId("match-history-stop-at-target")).not.toBeInTheDocument();
    });

    it("names a match whose declarations did not transfer", () => {
      renderCard({ match: makeMatch({ declarationsTransfer: false }) });

      const chip = screen.getByTestId("match-history-no-declarations-transfer");
      expect(chip).toHaveTextContent("No declaration transfer");
      expect(chip).toHaveAccessibleName(/voided instead of going to the other team/i);
    });

    it("leaves the transfer unchipped when it is on or absent from an older server", () => {
      renderCard({ match: makeMatch({ declarationsTransfer: true }) });
      expect(
        screen.queryByTestId("match-history-no-declarations-transfer"),
      ).not.toBeInTheDocument();

      const legacy = makeMatch();
      delete (legacy as { declarationsTransfer?: boolean }).declarationsTransfer;
      renderCard({ match: legacy });
      expect(
        screen.queryByTestId("match-history-no-declarations-transfer"),
      ).not.toBeInTheDocument();
    });

    it("names a match that was played clockwise", () => {
      renderCard({ match: makeMatch({ clockwise: true }) });

      const chip = screen.getByTestId("match-history-clockwise");
      expect(chip).toHaveTextContent("Clockwise");
      expect(chip).toHaveAccessibleName(/play moves to the left/i);
    });

    it("leaves the direction unchipped when it is counter-clockwise or absent from an older server", () => {
      renderCard({ match: makeMatch({ clockwise: false }) });
      expect(screen.queryByTestId("match-history-clockwise")).not.toBeInTheDocument();

      const legacy = makeMatch();
      delete (legacy as { clockwise?: boolean }).clockwise;
      renderCard({ match: legacy });
      expect(screen.queryByTestId("match-history-clockwise")).not.toBeInTheDocument();
    });
  });
});
