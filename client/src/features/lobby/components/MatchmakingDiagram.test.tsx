import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MatchmakingDiagram } from "@/features/lobby/components/MatchmakingDiagram";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

function player(userId: number, username: string, seat: number): RoomPlayer {
  return {
    id: userId,
    roomId: 1,
    userId,
    username,
    seat,
    team: seat % 2 === 0 ? "teamA" : "teamB",
    isBot: false,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

// Defaults to the real Quick Play config: a per-move 30s timer with a coin
// stake. Overrides let individual tests exercise the relaxed / free variants.
function room(overrides: Partial<Room> = {}): Room {
  return {
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
  };
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

    // Centre is labelled "You".
    expect(screen.getByText("You")).toBeInTheDocument();
    // The three other seats are still searching.
    expect(screen.getAllByText("searching")).toHaveLength(3);
    // Progress reflects 1 of 4 seated and 3 still to go.
    expect(screen.getByText("1 / 4 seated")).toBeInTheDocument();
    expect(screen.getByText("3 more to go")).toBeInTheDocument();
    // Elapsed time is surfaced.
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
    // The per-move timer shows its real duration, NOT the old hardcoded "Relaxed".
    expect(screen.getByText("30s timer")).toBeInTheDocument();
    expect(screen.queryByText("Relaxed")).not.toBeInTheDocument();
    // The coin stake (per-human buy-in) is shown.
    expect(screen.getByText("500")).toBeInTheDocument();
  });

  it("shows 'Relaxed' and 'No stake' for a relaxed, free room", () => {
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
    expect(screen.getByText("No stake")).toBeInTheDocument();
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
