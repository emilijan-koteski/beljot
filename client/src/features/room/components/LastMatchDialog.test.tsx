import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchListItem } from "@/shared/api/matches";
import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser, QueryWrapper } from "@/test-utils";

const mockGetRoomLastMatch = vi.fn();
vi.mock("@/shared/api/matches", () => ({
  getRoomLastMatch: (roomId: number) => mockGetRoomLastMatch(roomId),
}));

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

import { LastMatchDialog } from "./LastMatchDialog";

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

function renderDialog({ open = true, playersInRoom = [] as number[] } = {}) {
  return render(
    <QueryWrapper>
      <BrowserRouter>
        <LastMatchDialog
          open={open}
          roomId={5}
          onOpenChange={vi.fn()}
          playersInRoom={playersInRoom}
        />
      </BrowserRouter>
    </QueryWrapper>,
  );
}

describe("LastMatchDialog", () => {
  beforeEach(() => {
    mockGetRoomLastMatch.mockReset();
    mockGetRoomLastMatch.mockResolvedValue(makeMatch());
    useAuthStore.setState({ user: makeUser({ id: 10, username: "viewer" }) });
  });

  it("renders nothing and fetches nothing while closed", () => {
    renderDialog({ open: false });
    expect(screen.queryByTestId("last-match-dialog")).toBeNull();
    expect(mockGetRoomLastMatch).not.toHaveBeenCalled();
  });

  it("fetches the room's last match on open and shows it expanded", async () => {
    renderDialog();
    await waitFor(() => expect(mockGetRoomLastMatch).toHaveBeenCalledWith(5));
    expect(await screen.findByTestId("match-history-row")).toBeInTheDocument();
    // Expanded by default — the breakdown is the reason this dialog exists.
    expect(screen.getByTestId("match-history-detail")).toBeInTheDocument();
    expect(screen.getByTestId("match-history-hand-row")).toBeInTheDocument();
  });

  it("collapses and re-expands from the card's chevron", async () => {
    renderDialog();
    await screen.findByTestId("match-history-detail");

    await userEvent.click(screen.getByTestId("match-history-row-header"));
    expect(screen.queryByTestId("match-history-detail")).toBeNull();

    await userEvent.click(screen.getByTestId("match-history-row-header"));
    expect(screen.getByTestId("match-history-detail")).toBeInTheDocument();
  });

  it("offers Reinvite for a friend who is not currently in the room", async () => {
    renderDialog();
    expect(await screen.findByTestId("match-player-reinvite-11")).toBeInTheDocument();
  });

  it("hides Reinvite for a co-player already back at the table", async () => {
    renderDialog({ playersInRoom: [11] });
    await screen.findByTestId("match-player-reinvite-12");
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });

  it("renders the unavailable line instead of crashing on a 404", async () => {
    mockGetRoomLastMatch.mockRejectedValue(new FetchError(404, "NOT_FOUND", "not found"));
    renderDialog();
    expect(await screen.findByTestId("last-match-unavailable")).toBeInTheDocument();
    expect(screen.queryByTestId("match-history-row")).toBeNull();
  });
});
