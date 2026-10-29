import "@/shared/i18n/i18n";

import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RoomCard } from "@/features/lobby/components/RoomCard";
import { i18n } from "@/shared/i18n/i18n";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Room } from "@/shared/types/apiTypes";
import { makeRoom, makeRoomPlayer, makeUser } from "@/test-utils";

const host = makeRoomPlayer({ id: 1, userId: 1, username: "host", seat: 0, team: "A" });
const baseRoom: Room = makeRoom({ id: 1, name: "Table One", playerCount: 1, players: [host] });

function botAt(seat: number) {
  return makeRoomPlayer({ id: 0, userId: 0, username: "", seat, isBot: true });
}

describe("RoomCard", () => {
  beforeEach(() => {
    useAuthStore.setState({ user: makeUser({ honorScore: 80, isNewPlayer: false }) });
  });

  afterEach(async () => {
    cleanup();
    await i18n.changeLanguage("en");
  });

  it("lists variant, target, timer, buy-in, honor and rules as label-value rows", () => {
    render(
      <RoomCard
        room={{
          ...baseRoom,
          variant: "croatia",
          matchMode: "501",
          timerStyle: "per-move",
          timerDurationSeconds: 20,
          coinBuyIn: 1000,
          minHonor: 70,
        }}
        onJoin={() => {}}
      />,
    );

    const card = screen.getByTestId("room-card");
    expect(within(card).getByRole("heading")).toHaveTextContent("Table One");
    expect(card).toHaveTextContent("VariantCroatian");
    expect(card).toHaveTextContent("Target501");
    expect(card).toHaveTextContent("Timer20s");
    expect(screen.getByTestId("room-card-buy-in")).toHaveTextContent("1,000");
    expect(screen.getByTestId("room-card-honor")).toHaveTextContent("70+");
    expect(screen.getByTestId("room-rules-trigger")).toBeInTheDocument();
  });

  it("reads Relaxed, Free and Any for an unhurried, free, ungated room", () => {
    render(<RoomCard room={baseRoom} onJoin={() => {}} />);

    expect(screen.getByTestId("room-card")).toHaveTextContent("TimerRelaxed");
    expect(screen.getByTestId("room-card-buy-in")).toHaveTextContent("Free");
    expect(screen.getByTestId("room-card-honor")).toHaveTextContent("Any");
  });

  it("localizes the row labels and values in mk", async () => {
    await i18n.changeLanguage("mk");
    render(<RoomCard room={{ ...baseRoom, variant: "croatia" }} onJoin={() => {}} />);

    const card = screen.getByTestId("room-card");
    expect(card).toHaveTextContent("Варијанта");
    expect(card).toHaveTextContent("Хрватска");
    expect(card).toHaveTextContent("Влог");
  });

  it("marks a Quick Play room with a bolt and a private room with a padlock", () => {
    const { unmount } = render(
      <RoomCard room={{ ...baseRoom, isQuickPlay: true }} onJoin={() => {}} />,
    );
    expect(screen.getByRole("img", { name: "Quick Play" })).toBeInTheDocument();
    unmount();

    render(<RoomCard room={{ ...baseRoom, isPrivate: true }} onJoin={() => {}} />);
    expect(screen.getByRole("img", { name: "Private" })).toBeInTheDocument();
  });

  it("joins when a joinable card is tapped", async () => {
    const user = userEvent.setup();
    const onJoin = vi.fn();
    render(<RoomCard room={baseRoom} onJoin={onJoin} />);

    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "open");
    await user.click(screen.getByRole("button", { name: "Join Table One" }));

    expect(onJoin).toHaveBeenCalledWith(baseRoom);
  });

  it("disables a room whose four seats are taken, bots included", async () => {
    const user = userEvent.setup();
    const onJoin = vi.fn();
    const players = [
      host,
      botAt(1),
      makeRoomPlayer({ id: 2, userId: 2, username: "ana", seat: 2 }),
      botAt(3),
    ];
    render(<RoomCard room={{ ...baseRoom, playerCount: 2, players }} onJoin={onJoin} />);

    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "disabled");
    const join = screen.getByTestId("room-card-join");
    expect(join).toBeDisabled();
    expect(join).toHaveAttribute("title", "This room is full");
    await user.click(join);
    expect(onJoin).not.toHaveBeenCalled();
  });

  it("disables a room above the viewer's honor and says why", () => {
    render(<RoomCard room={{ ...baseRoom, minHonor: 85 }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "disabled");
    const title = screen.getByTestId("room-card-join").getAttribute("title") ?? "";
    expect(title).toContain("85");
    expect(title).toContain("80");
  });

  it("disables a veterans-only room for a new player", () => {
    useAuthStore.setState({ user: makeUser({ honorScore: 0, isNewPlayer: true }) });
    render(<RoomCard room={{ ...baseRoom, allowNewPlayers: false }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "disabled");
    expect(screen.getByTestId("room-card-join")).toHaveAttribute(
      "title",
      i18n.t("room.errors.newPlayerNotAllowed"),
    );
  });

  it("seats players in team order with a bot face and dashed open seats", async () => {
    await i18n.changeLanguage("mk");
    render(<RoomCard room={{ ...baseRoom, players: [host, botAt(1)] }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-1-seat-0")).toHaveAttribute("data-seat", "player");
    expect(screen.getByTestId("room-1-seat-1")).toHaveAttribute("data-seat", "bot");
    expect(screen.getByTestId("room-1-seat-1")).toHaveTextContent("Бот 2");
    expect(screen.getByTestId("room-1-seat-2")).toHaveAttribute("data-seat", "open");
    const order = within(screen.getByTestId("room-card-seats"))
      .getAllByTestId(/room-1-seat-/)
      .map((seat) => seat.dataset.testid);
    expect(order).toEqual(["room-1-seat-0", "room-1-seat-2", "room-1-seat-1", "room-1-seat-3"]);
  });

  it("shows a seated player's picture, and initials without one", () => {
    const url = "https://assets.test/avatars/h/128.webp";
    const { unmount } = render(
      <RoomCard room={{ ...baseRoom, players: [{ ...host, avatarUrl: url }] }} onJoin={() => {}} />,
    );
    expect(screen.getByTestId("room-1-seat-0").querySelector("img")).toHaveAttribute("src", url);
    unmount();

    render(<RoomCard room={baseRoom} onJoin={() => {}} />);
    expect(screen.getByTestId("room-1-seat-0").querySelector("img")).toBeNull();
  });

  it("renders a live match read-only with a Live tag", () => {
    render(<RoomCard room={{ ...baseRoom, status: "playing", minHonor: 95 }} live />);

    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "live");
    expect(screen.queryByTestId("room-card-join")).not.toBeInTheDocument();
    expect(screen.getByText("Live")).toBeInTheDocument();
  });

  it("previews a room without a join action and never dims it", () => {
    render(<RoomCard room={{ ...baseRoom, minHonor: 95 }} preview />);

    expect(screen.getByTestId("room-card")).toHaveAttribute("data-state", "open");
    expect(screen.queryByTestId("room-card-join")).not.toBeInTheDocument();
  });
});
