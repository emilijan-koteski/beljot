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

  describe("hanging points (Bitola tie)", () => {
    const handRow = (handNumber: number): HTMLElement => {
      const row = screen
        .getAllByTestId("match-history-hand-row")
        .find((r) => r.dataset.handNumber === String(handNumber));
      if (row === undefined) throw new Error(`no row for hand ${handNumber}`);
      return row;
    };
    const hangingMatch = (viewerSeat: number) =>
      makeMatch({
        viewerSeat,
        status: "abandoned",
        teamAScore: 262,
        teamBScore: 62,
        hands: [
          makeHand({
            handNumber: 1,
            teamACardPoints: 81,
            teamBCardPoints: 71,
            teamADeclPoints: 0,
            lastTrickTeam: 1,
            contractingTeam: 1,
            teamAHandTotal: 81,
            teamBHandTotal: 0,
            hangingCreated: { amount: 81, team: 1 },
            hangingPaid: null,
          }),
          makeHand({
            handNumber: 2,
            teamACardPoints: 90,
            teamBCardPoints: 62,
            teamADeclPoints: 0,
            lastTrickTeam: 0,
            contractingTeam: 0,
            teamAHandTotal: 181,
            teamBHandTotal: 62,
            hangingCreated: null,
            hangingPaid: { amount: 81, team: 0 },
          }),
        ],
      });

    it.each([
      ["stacked", "stacked" as const],
      ["auto", "auto" as const],
    ])("notes the tie and the payout per hand in the %s layout", (_name, handsLayout) => {
      renderCard({ match: hangingMatch(0), isOpen: true, handsLayout });

      expect(within(handRow(1)).getByTestId("match-history-hand-hanging")).toHaveTextContent(
        "tie · their 81 hangs",
      );
      expect(within(handRow(1)).queryByTestId("match-history-hand-hanging-paid")).toBeNull();
      expect(within(handRow(2)).getByTestId("match-history-hand-hanging-paid")).toHaveTextContent(
        "+81 hanging · us",
      );
      expect(within(handRow(2)).queryByTestId("match-history-hand-hanging")).toBeNull();
    });

    it("reads the tie and the payout from the other team's seat", () => {
      renderCard({ match: hangingMatch(1), isOpen: true, handsLayout: "stacked" });

      expect(within(handRow(1)).getByTestId("match-history-hand-hanging")).toHaveTextContent(
        "tie · our 81 hangs",
      );
      expect(within(handRow(2)).getByTestId("match-history-hand-hanging-paid")).toHaveTextContent(
        "+81 hanging · them",
      );
    });

    it.each([
      [0, "tie · their 81 discarded"],
      [1, "tie · our 81 discarded"],
    ])(
      "marks a tie in the last recorded hand as discarded (viewer seat %i)",
      (viewerSeat, note) => {
        const match = makeMatch({
          viewerSeat,
          status: "abandoned",
          teamAScore: 201,
          teamBScore: 72,
          hands: [
            makeHand({ handNumber: 1 }),
            makeHand({
              handNumber: 2,
              teamACardPoints: 81,
              teamBCardPoints: 71,
              teamADeclPoints: 0,
              lastTrickTeam: 1,
              contractingTeam: 1,
              teamAHandTotal: 81,
              teamBHandTotal: 0,
              hangingCreated: { amount: 81, team: 1 },
              hangingPaid: null,
            }),
          ],
        });

        renderCard({ match, isOpen: true, handsLayout: "stacked" });

        expect(within(handRow(2)).getByTestId("match-history-hand-hanging")).toHaveTextContent(
          note,
        );
      },
    );

    it("counts a payout inside the hand totals, leaving no unfinished hand", () => {
      renderCard({ match: hangingMatch(0), isOpen: true });

      expect(screen.queryByTestId("match-history-unscored-hand")).not.toBeInTheDocument();
    });

    it("shows no hanging note for a hand from an older server without the fields", () => {
      renderCard({ match: makeMatch(), isOpen: true });

      expect(screen.queryByTestId("match-history-hand-hanging")).not.toBeInTheDocument();
      expect(screen.queryByTestId("match-history-hand-hanging-paid")).not.toBeInTheDocument();
    });
  });

  describe("rules", () => {
    it("names the five match rules on the rule icons, brass where they changed", () => {
      renderCard({
        match: makeMatch({
          declarationsEnabled: false,
          stopAtTarget: true,
          declarationsTransfer: true,
          clockwise: true,
        }),
      });

      expect(screen.getByTestId("room-rules-trigger")).toHaveAccessibleName(
        "Rules: Direction: To the left, Declarations: Off, Declare until trick: Not used, Declarations on a fall: Not used, Match end: Stop at target",
      );
    });

    it("reads a match from an older server without the rule columns as standard", () => {
      const legacy = makeMatch();
      delete (legacy as { declarationsEnabled?: boolean }).declarationsEnabled;
      delete (legacy as { stopAtTarget?: boolean }).stopAtTarget;
      delete (legacy as { declarationsTransfer?: boolean }).declarationsTransfer;
      delete (legacy as { clockwise?: boolean }).clockwise;
      delete (legacy as { declarationLastTrick?: number }).declarationLastTrick;
      renderCard({ match: legacy });

      expect(screen.getByTestId("room-rules-trigger")).toHaveAccessibleName(
        "Rules: Direction: To the right, Declarations: On, Declare until trick: 3, Declarations on a fall: To the winners, Match end: Finish the hand",
      );
    });

    it("opens the match rules in words without toggling the card", async () => {
      const user = userEvent.setup();
      const { onToggle } = renderCard({ match: makeMatch({ declarationsTransfer: false }) });

      await user.click(screen.getByTestId("room-rules-trigger"));

      const popup = await screen.findByTestId("room-rules-popup");
      expect(within(popup).getByTestId("rule-fall")).toHaveTextContent("Voided");
      expect(within(popup).getByTestId("rule-fall")).toHaveAttribute("data-changed", "true");
      expect(within(popup).queryByTestId("rules-room")).not.toBeInTheDocument();
      await user.click(within(popup).getByTestId("rule-fall"));
      expect(onToggle).not.toHaveBeenCalled();
    });
  });
});
