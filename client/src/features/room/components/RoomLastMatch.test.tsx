import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { MatchListItem } from "@/shared/api/matches";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser, QueryWrapper } from "@/test-utils";

vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: vi.fn(() => Promise.resolve({ status: "friends", requestId: 1 })),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}));

vi.mock("@/shared/api/rooms", () => ({
  inviteToRoom: vi.fn(() => Promise.resolve()),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { LastMatchOutcome, RoomLastMatch } from "./RoomLastMatch";

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
    hands: [
      {
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
      },
    ],
    ...overrides,
  };
}

function renderPanel(playersInRoom: number[] = []) {
  return render(
    <QueryWrapper>
      <BrowserRouter>
        <RoomLastMatch match={makeMatch()} roomId={5} playersInRoom={playersInRoom} compact />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

describe("RoomLastMatch", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: makeUser({ id: 10, username: "viewer" }) });
  });

  it("shows the match expanded with its hands", async () => {
    renderPanel();
    expect(await screen.findByTestId("match-history-row")).toBeInTheDocument();
    expect(screen.getByTestId("match-history-detail")).toBeInTheDocument();
    expect(screen.getByTestId("match-history-hand-row")).toBeInTheDocument();
  });

  it("collapses and re-expands from the card's chevron", async () => {
    renderPanel();
    await screen.findByTestId("match-history-detail");

    await userEvent.click(screen.getByTestId("match-history-row-header"));
    expect(screen.queryByTestId("match-history-detail")).toBeNull();

    await userEvent.click(screen.getByTestId("match-history-row-header"));
    expect(screen.getByTestId("match-history-detail")).toBeInTheDocument();
  });

  it("offers Invite back to a player who is not in the room", async () => {
    renderPanel();
    expect(await screen.findByTestId("match-player-reinvite-11")).toBeInTheDocument();
  });

  it("hides Invite back for a player already back in the room", async () => {
    renderPanel([11]);
    await screen.findByTestId("match-player-reinvite-12");
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });
});

describe("LastMatchOutcome", () => {
  it("tags a win, a loss, and nothing for an abandoned match", () => {
    const { rerender, container } = render(<LastMatchOutcome outcome="win" />);
    expect(screen.getByText("Win")).toBeInTheDocument();
    rerender(<LastMatchOutcome outcome="loss" />);
    expect(screen.getByText("Loss")).toBeInTheDocument();
    rerender(<LastMatchOutcome outcome="abandoned" />);
    expect(container).toBeEmptyDOMElement();
  });
});
