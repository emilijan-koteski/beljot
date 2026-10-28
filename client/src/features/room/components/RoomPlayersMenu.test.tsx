import "@/shared/i18n/i18n";

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { makeRoomPlayer } from "@/test-utils";

import { RoomPlayersMenu } from "./RoomPlayersMenu";

const HOST = makeRoomPlayer({ userId: 10, username: "kiro", seat: 0 });
const DRAGAN = makeRoomPlayer({ userId: 20, username: "Dragan", seat: 2 });
const IVANA = makeRoomPlayer({ userId: 30, username: "Ivana", seat: null });
const BOT = makeRoomPlayer({ userId: 0, username: "", seat: 1, isBot: true });

function renderMenu(overrides: Partial<Parameters<typeof RoomPlayersMenu>[0]> = {}) {
  const props = {
    players: [HOST, DRAGAN, IVANA, BOT],
    ownerId: 10,
    viewerId: 10,
    canManage: true,
    teamOf: () => null,
    promotePending: false,
    kickPending: false,
    onPromote: vi.fn(),
    onKick: vi.fn(),
    ...overrides,
  };
  render(<RoomPlayersMenu {...props} />);
  return props;
}

describe("RoomPlayersMenu", () => {
  it("gives the host menu items for seated and standing players but not for themselves or bots", async () => {
    const user = userEvent.setup();
    renderMenu();
    await user.click(screen.getByTestId("in-room-count"));

    expect(
      await screen.findByRole("menuitem", { name: "Make Dragan the host" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Kick Dragan" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Kick Ivana" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Make Ivana the host" })).not.toBeInTheDocument();
    expect(screen.queryByTestId("kick-list-10")).not.toBeInTheDocument();
    expect(within(screen.getByTestId("in-room-list-bot-1")).queryByRole("menuitem")).toBeNull();
  });

  it("moves between the row actions with the arrow keys and runs one with Enter", async () => {
    const user = userEvent.setup();
    const props = renderMenu();
    await user.click(screen.getByTestId("in-room-count"));
    await screen.findByRole("menuitem", { name: "Make Dragan the host" });

    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("menuitem", { name: "Make Dragan the host" })).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("menuitem", { name: "Kick Dragan" })).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(props.onKick).toHaveBeenCalledWith(DRAGAN);
  });

  it("lists the players without actions for everyone but the host", async () => {
    const user = userEvent.setup();
    renderMenu({ viewerId: 20, canManage: false });
    await user.click(screen.getByTestId("in-room-count"));

    expect(await screen.findByTestId("in-room-list-item-20")).toBeInTheDocument();
    expect(screen.queryAllByRole("menuitem")).toHaveLength(0);
  });
});
