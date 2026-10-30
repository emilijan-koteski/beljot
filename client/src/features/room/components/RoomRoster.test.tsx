import "@/shared/i18n/i18n";

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { makeRoomPlayer, QueryWrapper } from "@/test-utils";

vi.mock("@/shared/api/friends", () => ({
  getFriendshipStatus: vi.fn(() => Promise.resolve({ status: "none", requestId: null })),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  declineFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}));

import { cardinalFor, type TableSeat } from "../lib/tableSeat";
import { RoomRoster, type RoomRosterProps } from "./RoomRoster";

const HOST = makeRoomPlayer({ userId: 10, username: "kiro", seat: 0 });
const DRAGAN = makeRoomPlayer({ userId: 20, username: "Dragan", seat: 2 });
const IVANA = makeRoomPlayer({ userId: 30, username: "Ivana", seat: null });
const BOT = makeRoomPlayer({ userId: 0, username: "", seat: 1, isBot: true });

function seatOf(seat: 0 | 1 | 2 | 3, player?: typeof HOST): TableSeat {
  return {
    seat,
    cardinal: cardinalFor(seat, 0),
    team: seat % 2 === 0 ? "A" : "B",
    player,
    name: player === undefined ? "" : player.isBot ? `Bot ${seat + 1}` : player.username,
    isYou: player?.userId === 10,
    isHost: player?.userId === 10,
    returning: false,
    belowHonorBar: false,
    swapSource: false,
    pending: false,
    clickable: true,
    canAddBot: false,
    hostMenu: false,
  };
}

function renderRoster(overrides: Partial<RoomRosterProps> = {}) {
  const props: RoomRosterProps = {
    table: [seatOf(0, HOST), seatOf(1, BOT), seatOf(2, DRAGAN), seatOf(3)],
    standing: [IVANA],
    viewerId: 10,
    ownerId: 10,
    canManage: true,
    canInvite: true,
    onPromote: vi.fn(),
    onKick: vi.fn(),
    onRemoveBot: vi.fn(),
    onInvite: vi.fn(),
    ...overrides,
  };
  render(
    <QueryWrapper>
      <RoomRoster {...props} />
    </QueryWrapper>,
  );
  return props;
}

describe("RoomRoster", () => {
  it("gives the host Make host and Kick on seated players, Kick on standing ones, and nothing on themselves", () => {
    renderRoster();

    expect(screen.getByTestId("promote-player-20")).toBeInTheDocument();
    expect(screen.getByTestId("kick-list-20")).toBeInTheDocument();
    expect(screen.getByTestId("kick-list-30")).toBeInTheDocument();
    expect(screen.queryByTestId("promote-player-30")).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-list-10")).not.toBeInTheDocument();
    expect(screen.getByTestId("roster-remove-bot-1")).toBeInTheDocument();
  });

  it("runs the host's actions with the row's player", async () => {
    const user = userEvent.setup();
    const props = renderRoster();

    await user.click(screen.getByTestId("kick-list-20"));
    await user.click(screen.getByTestId("promote-player-20"));
    await user.click(screen.getByTestId("roster-remove-bot-1"));
    await user.click(screen.getByTestId("roster-invite-3"));

    expect(props.onKick).toHaveBeenCalledWith(DRAGAN);
    expect(props.onPromote).toHaveBeenCalledWith(DRAGAN);
    expect(props.onRemoveBot).toHaveBeenCalledWith(1);
    expect(props.onInvite).toHaveBeenCalled();
  });

  it("lists the seats from the viewer round the table, then the players without a seat", () => {
    renderRoster();

    const items = screen.getAllByRole("listitem").map((li) => li.textContent ?? "");
    expect(items[0]).toContain("kiro");
    expect(items[1]).toContain("Bot 2");
    expect(items[2]).toContain("Dragan");
    expect(items[3]).toContain("Open seat");
    expect(items[4]).toContain("Ivana");
  });

  it("shows a guest Add friend instead of host actions", async () => {
    renderRoster({ viewerId: 20, canManage: false });

    expect(screen.queryByTestId("kick-list-30")).not.toBeInTheDocument();
    const row = screen.getByTestId("in-room-list-item-30");
    expect(await within(row).findByRole("button", { name: /add friend/i })).toBeInTheDocument();
    expect(within(screen.getByTestId("in-room-list-item-20")).queryByRole("button")).toBeNull();
  });
});
