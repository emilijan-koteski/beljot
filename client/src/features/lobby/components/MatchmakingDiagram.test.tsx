import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MatchmakingDiagram } from "@/features/lobby/components/MatchmakingDiagram";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";
import { makeRoom } from "@/test-utils";

function player(userId: number, username: string, seat: number): RoomPlayer {
  return {
    id: userId,
    roomId: 1,
    userId,
    username,
    avatarUrl: null,
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

// A bot occupant as it arrives over the wire (Quick Play auto-fill): userId 0,
// an EMPTY username, isBot true. Its display name is seat-derived client-side.
function botPlayer(seat: number): RoomPlayer {
  return {
    id: 0,
    roomId: 1,
    userId: 0,
    username: "",
    avatarUrl: null,
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: true,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

// Defaults to the real Quick Play config: a per-move 30s timer with a coin
// stake. Overrides let individual tests exercise the relaxed / free variants.
function room(overrides: Partial<Room> = {}): Room {
  return makeRoom({
    id: 1,
    name: "Quick Play ABC123",
    code: "ABC123",
    ownerId: 7,
    ownerUsername: "dejan_k",
    variant: "bitola",
    matchMode: "1001",
    timerStyle: "per-move",
    timerDurationSeconds: 30,
    coinBuyIn: 500,
    status: "waiting",
    playerCount: 1,
    isQuickPlay: true,
    isPrivate: false,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  });
}

describe("MatchmakingDiagram", () => {
  it("shows the viewer at centre and a searching slot for every empty seat", () => {
    render(
      <MatchmakingDiagram
        room={room()}
        found={1}
        players={[player(7, "dejan_k", 0)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:07"
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText("You")).toBeInTheDocument();
    expect(screen.getAllByText("searching")).toHaveLength(3);
    expect(screen.getByText("1 / 4 seated")).toBeInTheDocument();
    expect(screen.getByText("3 more to go")).toBeInTheDocument();
    expect(screen.getByText("00:07")).toBeInTheDocument();
  });

  it("renders joined opponents by username and leaves remaining seats searching", () => {
    render(
      <MatchmakingDiagram
        room={room()}
        found={3}
        players={[player(7, "dejan_k", 0), player(8, "ena_h", 1), player(9, "filip", 2)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:21"
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText("ena_h")).toBeInTheDocument();
    expect(screen.getByText("filip")).toBeInTheDocument();
    expect(screen.getAllByText("searching")).toHaveLength(1);
    expect(screen.getByText("3 / 4 seated")).toBeInTheDocument();
    expect(screen.getByText("1 more to go")).toBeInTheDocument();
  });

  it("draws the viewer's and each human occupant's picture, never a bot's", () => {
    const you = "https://assets.test/avatars/you/128.webp";
    const them = "https://assets.test/avatars/ena/128.webp";
    const { container } = render(
      <MatchmakingDiagram
        room={room()}
        found={3}
        players={[
          { ...player(7, "dejan_k", 0), avatarUrl: you },
          { ...player(8, "ena_h", 1), avatarUrl: them },
          { ...botPlayer(2), avatarUrl: them },
        ]}
        viewerSeat={0}
        currentUsername="dejan_k"
        currentAvatarUrl={you}
        elapsed="00:21"
        onCancel={() => {}}
      />,
    );

    const srcs = Array.from(container.querySelectorAll("img")).map((i) => i.getAttribute("src"));
    expect(srcs).toContain(you);
    expect(screen.getByTestId("matchmaking-orbit-1").querySelector("img")).toHaveAttribute(
      "src",
      them,
    );
    expect(screen.getByTestId("matchmaking-orbit-2").querySelector("img")).toBeNull();
  });

  it("surfaces the room's real variant, mode, per-move timer, and coin stake", () => {
    render(
      <MatchmakingDiagram
        room={room({ timerStyle: "per-move", timerDurationSeconds: 30, coinBuyIn: 500 })}
        found={1}
        players={[player(7, "dejan_k", 0)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:07"
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText("Bitola")).toBeInTheDocument();
    expect(screen.getByText("1001 pts")).toBeInTheDocument();
    expect(screen.getByText("30s")).toBeInTheDocument();
    expect(screen.queryByText("Relaxed")).not.toBeInTheDocument();
    expect(screen.getByText("500")).toBeInTheDocument();
  });

  it("shows 'Relaxed' and 'Free' for a relaxed, free room", () => {
    render(
      <MatchmakingDiagram
        room={room({ timerStyle: "relaxed", timerDurationSeconds: null, coinBuyIn: 0 })}
        found={1}
        players={[player(7, "dejan_k", 0)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:07"
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText("Relaxed")).toBeInTheDocument();
    expect(screen.getByText("Free")).toBeInTheDocument();
  });

  it("renders bot occupants with the localized bot name and glyph, never a blank", () => {
    render(
      <MatchmakingDiagram
        room={room()}
        found={4}
        players={[player(7, "dejan_k", 0), botPlayer(1), botPlayer(2), botPlayer(3)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:09"
        onCancel={() => {}}
      />,
    );

    expect(screen.getByTestId("matchmaking-bot-name-1")).toHaveTextContent("Bot 2");
    expect(screen.getByTestId("matchmaking-bot-name-2")).toHaveTextContent("Bot 3");
    expect(screen.getByTestId("matchmaking-bot-name-3")).toHaveTextContent("Bot 4");
    expect(screen.queryByText("searching")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("avatar-icon")).toHaveLength(3);
    expect(screen.getByText("4 / 4 seated")).toBeInTheDocument();
  });

  it("surfaces a Croatia / 501 quick-play room's chips", () => {
    render(
      <MatchmakingDiagram
        room={room({ variant: "croatia", matchMode: "501" })}
        found={1}
        players={[player(7, "dejan_k", 0)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:03"
        onCancel={() => {}}
      />,
    );

    expect(screen.getByText("Croatian")).toBeInTheDocument();
    expect(screen.getByText("501 pts")).toBeInTheDocument();
  });

  it("invokes onCancel when the cancel button is pressed", async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(
      <MatchmakingDiagram
        room={room()}
        found={1}
        players={[player(7, "dejan_k", 0)]}
        viewerSeat={0}
        currentUsername="dejan_k"
        elapsed="00:07"
        onCancel={onCancel}
      />,
    );

    await user.click(screen.getByRole("button", { name: /cancel search/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
