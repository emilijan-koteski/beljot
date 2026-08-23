import "@/shared/i18n/i18n";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchListItem } from "@/shared/api/matches";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import type { FriendshipStatus } from "@/shared/types/apiTypes";
import { makeUser, QueryWrapper } from "@/test-utils";

const mockGetFriendshipStatus = vi.fn();
vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: (id: number) => mockGetFriendshipStatus(id),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}));

const mockInviteToRoom = vi.fn();
vi.mock("@/shared/api/rooms", () => ({
  inviteToRoom: (...args: unknown[]) => mockInviteToRoom(...args),
  createRoom: vi.fn(),
  joinRoom: vi.fn(),
  leaveRoom: vi.fn(),
  leaveSeat: vi.fn(),
  quickJoin: vi.fn(),
  quickPlay: vi.fn(),
  removeBot: vi.fn(),
  addBot: vi.fn(),
  selectSeat: vi.fn(),
  startMatch: vi.fn(),
  swapSeats: vi.fn(),
  transferOwnership: vi.fn(),
  updateRoomPrivacy: vi.fn(),
  declineRoomInvite: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { toast } from "sonner";

import { MatchPlayerActions } from "./MatchPlayerActions";

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
    hands: [],
    ...overrides,
  };
}

function statusFor(status: FriendshipStatus["status"]): FriendshipStatus {
  return { status, requestId: status === "none" ? null : 1 };
}

function renderActions(props: Partial<Parameters<typeof MatchPlayerActions>[0]> = {}) {
  return render(
    <QueryWrapper>
      <MatchPlayerActions
        match={props.match ?? makeMatch()}
        roomId={props.roomId}
        showReinvite={props.showReinvite}
        playersInRoom={props.playersInRoom}
        allowRemoveFriend={props.allowRemoveFriend}
      />
    </QueryWrapper>,
  );
}

describe("MatchPlayerActions", () => {
  beforeEach(() => {
    mockGetFriendshipStatus.mockReset();
    mockInviteToRoom.mockReset();
    vi.mocked(toast.error).mockClear();
    vi.mocked(toast.success).mockClear();
    mockGetFriendshipStatus.mockResolvedValue(statusFor("none"));
    mockInviteToRoom.mockResolvedValue(undefined);
    useAuthStore.setState({ user: makeUser({ id: 10, username: "viewer" }) });
  });

  it("renders one row per human co-player, never the viewer's own seat", async () => {
    renderActions();
    const rows = await screen.findAllByTestId("match-player-action-row");
    expect(rows).toHaveLength(3);
    expect(rows.map((r) => r.getAttribute("data-user-id"))).toEqual(["11", "12", "13"]);
  });

  it("skips bot seats and soft-deleted participants", async () => {
    renderActions({
      match: makeMatch({
        players: [
          { seat: 0, userId: 10, username: "viewer", isBot: false },
          { seat: 1, userId: 0, username: "", isBot: true },
          // Soft-deleted: a real id with no username left to render or befriend.
          { seat: 2, userId: 12, username: "", isBot: false },
          { seat: 3, userId: 13, username: "opp2", isBot: false },
        ],
      }),
    });
    const rows = await screen.findAllByTestId("match-player-action-row");
    expect(rows).toHaveLength(1);
    expect(rows[0]).toHaveAttribute("data-user-id", "13");
  });

  it("renders nothing when every other seat is a bot", () => {
    const { container } = renderActions({
      match: makeMatch({
        players: [
          { seat: 0, userId: 10, username: "viewer", isBot: false },
          { seat: 1, userId: 0, username: "", isBot: true },
          { seat: 2, userId: 0, username: "", isBot: true },
          { seat: 3, userId: 0, username: "", isBot: true },
        ],
      }),
    });
    expect(container).toBeEmptyDOMElement();
  });

  it("shows Add-friend and no reinvite for a non-friend", async () => {
    renderActions({ roomId: 5, showReinvite: true });
    await waitFor(() => expect(screen.getAllByTestId("friend-button-add")).toHaveLength(3));
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });

  it("shows reinvite for a friend who is not in the room", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions({ roomId: 5, showReinvite: true });
    expect(await screen.findByTestId("match-player-reinvite-11")).toBeInTheDocument();
  });

  it("hides reinvite for a friend already seated in the room", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions({ roomId: 5, showReinvite: true, playersInRoom: [11] });
    await screen.findByTestId("match-player-reinvite-12");
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });

  it("hides reinvite entirely when showReinvite is off", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions({ roomId: 5 });
    await screen.findAllByTestId("match-player-action-row");
    expect(screen.queryByTestId("match-player-reinvite-11")).toBeNull();
  });

  it("issues a real room invite and confirms it on the row", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions({ roomId: 5, showReinvite: true });

    await userEvent.click(await screen.findByTestId("match-player-reinvite-11"));

    await waitFor(() => expect(mockInviteToRoom).toHaveBeenCalledWith(5, 11));
    expect(toast.success).toHaveBeenCalledWith(
      i18n.t("matchStats.reinviteSent", { username: "opp1" }),
    );
    await waitFor(() =>
      expect(screen.getByTestId("match-player-reinvite-11")).toHaveTextContent(
        i18n.t("matchStats.reinvited"),
      ),
    );
  });

  // The confirmation must NOT latch the row: an invite can be declined or lapse
  // on its TTL, and this row cannot observe either — a second press has to be
  // able to send a real second invite.
  it("stays pressable after a successful invite so a lapsed one can be re-sent", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions({ roomId: 5, showReinvite: true });

    await userEvent.click(await screen.findByTestId("match-player-reinvite-11"));
    await waitFor(() => expect(mockInviteToRoom).toHaveBeenCalledTimes(1));

    const button = screen.getByTestId("match-player-reinvite-11");
    expect(button).toBeEnabled();
    await userEvent.click(button);
    await waitFor(() => expect(mockInviteToRoom).toHaveBeenCalledTimes(2));
  });

  // The mapping itself lives in shared/lib/inviteFailure — asserted here against
  // the SAME roomInvite.errors.* keys the invite panel renders, so the two
  // surfaces cannot drift apart again.
  it.each([
    ["ROOM_FULL", "roomInvite.errors.roomFull"],
    ["INVITE_ALREADY_PENDING", "roomInvite.errors.alreadyPending"],
    ["FRIEND_NOT_AVAILABLE", "roomInvite.errors.notAvailable"],
    ["NOT_FRIENDS", "roomInvite.errors.notFriends"],
    ["ROOM_NOT_FOUND", "roomInvite.errors.roomGone"],
    ["SOMETHING_ELSE", "roomInvite.errors.sendFailed"],
  ])(
    "maps a %s rejection to its shared toast and keeps the row interactive",
    async (code, messageKey) => {
      mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
      mockInviteToRoom.mockRejectedValue(new FetchError(409, code, "nope"));
      renderActions({ roomId: 5, showReinvite: true });

      await userEvent.click(await screen.findByTestId("match-player-reinvite-11"));

      await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1));
      expect(toast.error).toHaveBeenCalledWith(i18n.t(messageKey));
      // The row stays interactive: room-full / not-available are transient.
      expect(screen.getByTestId("match-player-reinvite-11")).toBeEnabled();
    },
  );
  // P3: the confirm RemoveFriendDialog opens is a shadcn Dialog at z-50, while
  // the match-result overlay is Z.PROMPT (74) — it would open behind the panel,
  // invisible and unclickable. The overlay suppresses the affordance instead.
  it("offers Remove friend by default for an existing friend", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions();
    await waitFor(() => expect(screen.getAllByTestId("friend-button-friends")).toHaveLength(3));
    expect(screen.getAllByTestId("friend-button-remove")).toHaveLength(3);
  });

  it("suppresses Remove friend when allowRemoveFriend is false", async () => {
    mockGetFriendshipStatus.mockResolvedValue(statusFor("friends"));
    renderActions({ allowRemoveFriend: false });
    await waitFor(() => expect(screen.getAllByTestId("friend-button-friends")).toHaveLength(3));
    expect(screen.queryByTestId("friend-button-remove")).toBeNull();
  });
});
