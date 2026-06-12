import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { RoomCard } from "@/features/lobby/components/RoomCard";
import { i18n } from "@/shared/i18n/i18n";
import type { Room } from "@/shared/types/apiTypes";

const baseRoom: Room = {
  id: 1,
  name: "Table One",
  code: "ABC123",
  ownerId: 1,
  ownerUsername: "host",
  variant: "bitola",
  matchMode: "1001",
  timerStyle: "relaxed",
  timerDurationSeconds: null,
  status: "waiting",
  playerCount: 1,
  isQuickPlay: false,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  players: [
    {
      id: 1,
      roomId: 1,
      userId: 1,
      username: "host",
      seat: 0,
      team: "teamA",
      isBot: false,
      createdAt: "",
    },
  ],
};

describe("RoomCard", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("renders the localized 501 match-mode label in mk locale", async () => {
    // mk distinguishes the i18n label ("501 поен") from the unlocalized
    // "501 pts" fallback, which is identical to the en label.
    await i18n.changeLanguage("mk");

    render(<RoomCard room={{ ...baseRoom, matchMode: "501" }} onJoin={() => {}} />);

    expect(screen.getByText(/501 поен/)).toBeInTheDocument();
  });

  it("renders the localized bot name for bot seats in mk locale", async () => {
    // mk renders "Бот 2" — all-Cyrillic, seat-derived — never a blank chip
    // from the bot's empty wire username.
    await i18n.changeLanguage("mk");

    const players = [
      ...(baseRoom.players ?? []),
      {
        id: 0,
        roomId: 1,
        userId: 0,
        username: "",
        seat: 1,
        team: "teamB",
        isBot: true,
        createdAt: "",
      },
    ];
    render(<RoomCard room={{ ...baseRoom, players }} onJoin={() => {}} />);

    expect(screen.getByTestId("room-1-seat-1")).toHaveTextContent("Бот 2");
    // The chip disc shows the bot glyph, not a name initial.
    expect(screen.getByTestId("seat-chip-bot-icon")).toBeInTheDocument();
  });

  it("labels a quick-play room with the Quick Play badge and a 'Join queue' action", () => {
    render(<RoomCard room={{ ...baseRoom, isQuickPlay: true }} onJoin={() => {}} />);

    expect(screen.getByTestId("quick-play-badge")).toBeInTheDocument();
    expect(screen.getByTestId("room-card-join")).toHaveTextContent("Join queue");
  });

  it("renders a custom room with the plain Join action and no badge", () => {
    render(<RoomCard room={baseRoom} onJoin={() => {}} />);

    expect(screen.queryByTestId("quick-play-badge")).not.toBeInTheDocument();
    const join = screen.getByTestId("room-card-join");
    expect(join).toHaveTextContent("Join");
    expect(join).not.toHaveTextContent("Join queue");
  });
});
