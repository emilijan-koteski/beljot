import { describe, expect, it } from "vitest";

import { makeRoom, makeRoomPlayer } from "@/test-utils";

import { isRoomFull, roomBotCount, roomOccupancy } from "./roomOccupancy";

const human = (seat: number) => makeRoomPlayer({ id: seat + 1, userId: seat + 1, seat });
const bot = (seat: number) => makeRoomPlayer({ id: 0, userId: 0, seat, isBot: true });

describe("roomOccupancy", () => {
  it("counts humans and bots from the player list", () => {
    const room = makeRoom({ playerCount: 2, players: [human(0), human(2), bot(1)] });
    expect(roomOccupancy(room)).toBe(3);
    expect(roomBotCount(room)).toBe(1);
  });

  it("falls back to the human count when the list is missing", () => {
    expect(roomOccupancy(makeRoom({ playerCount: 3 }))).toBe(3);
    expect(roomBotCount(makeRoom({ playerCount: 3 }))).toBe(0);
  });

  it("calls a room full once bots take the last seats", () => {
    expect(
      isRoomFull(makeRoom({ playerCount: 2, players: [human(0), human(2), bot(1), bot(3)] })),
    ).toBe(true);
    expect(isRoomFull(makeRoom({ playerCount: 2, players: [human(0), human(2), bot(1)] }))).toBe(
      false,
    );
  });
});
