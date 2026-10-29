import { describe, expect, it } from "vitest";

import { makeRoom, makeRoomPlayer } from "@/test-utils";

import { filterAndSort, filterCounts } from "./lobbyRooms";

const viewer = { honorScore: 80, isNewPlayer: false };

function seated(count: number, bots = 0) {
  return Array.from({ length: count }, (_, seat) =>
    makeRoomPlayer({
      id: seat + 1,
      userId: seat < count - bots ? seat + 1 : 0,
      seat,
      isBot: seat >= count - bots,
    }),
  );
}

const rooms = [
  makeRoom({
    id: 1,
    name: "Full",
    code: "FUL111",
    createdAt: "2026-01-01T10:00:00Z",
    players: seated(4),
  }),
  makeRoom({
    id: 2,
    name: "Bots",
    code: "BOT222",
    createdAt: "2026-01-01T11:00:00Z",
    players: seated(3, 2),
  }),
  makeRoom({
    id: 3,
    name: "Timed",
    code: "TIM333",
    createdAt: "2026-01-01T12:00:00Z",
    timerStyle: "per-move",
    timerDurationSeconds: 20,
    players: seated(1),
  }),
  makeRoom({
    id: 4,
    name: "Gated",
    code: "GAT444",
    createdAt: "2026-01-01T09:00:00Z",
    minHonor: 85,
    players: seated(2),
  }),
];

describe("filterAndSort", () => {
  it("puts the rooms nearest to starting first and full rooms last", () => {
    expect(filterAndSort(rooms, "", "all", "filling", viewer).map((r) => r.name)).toEqual([
      "Bots",
      "Gated",
      "Timed",
      "Full",
    ]);
  });

  it("sorts newest first by creation time", () => {
    expect(filterAndSort(rooms, "", "all", "newest", viewer).map((r) => r.name)).toEqual([
      "Timed",
      "Bots",
      "Full",
      "Gated",
    ]);
  });

  it("matches search on the name or the code", () => {
    expect(filterAndSort(rooms, "bot", "all", "filling", viewer).map((r) => r.id)).toEqual([2]);
    expect(filterAndSort(rooms, "tim333", "all", "filling", viewer).map((r) => r.id)).toEqual([3]);
  });

  it("keeps rooms with a free seat, counting bots as seated", () => {
    const full = makeRoom({ id: 5, players: seated(4, 2) });
    expect(filterAndSort([...rooms, full], "", "open", "filling", viewer).map((r) => r.id)).toEqual(
      [2, 4, 3],
    );
  });
});

describe("filterCounts", () => {
  it("counts every filter chip", () => {
    expect(filterCounts(rooms, viewer)).toEqual({
      all: 4,
      qualify: 3,
      open: 3,
      relaxed: 3,
      timed: 1,
    });
  });
});
