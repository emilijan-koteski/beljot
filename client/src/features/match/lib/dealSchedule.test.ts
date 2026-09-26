import { describe, expect, it } from "vitest";

import { MOTION } from "@/shared/lib/motion";

import { buildDealSchedule, DEAL_DURATION, type DealKind } from "./dealSchedule";

describe("buildDealSchedule", () => {
  it("deals 3 then 2 from the dealer's left, then flips the candidate", () => {
    const s = buildDealSchedule("candidateFirst", 2);

    expect(s.packets.map((p) => [p.seat, p.count])).toEqual([
      [3, 3],
      [0, 3],
      [1, 3],
      [2, 3],
      [3, 2],
      [0, 2],
      [1, 2],
      [2, 2],
    ]);
    expect(s.packets.every((p) => !p.faceDown && !p.withCandidate)).toBe(true);
    expect(s.packets[0]!.startMs).toBe(MOTION.DEAL_LEAD_IN);
    expect(s.candidateFlipMs).toBe(s.packets[7]!.landMs);
  });

  it("deals 3, 3, then 2 face-down for the all-before-bidding shape", () => {
    const s = buildDealSchedule("allBeforeBidding", 0);

    expect(s.packets).toHaveLength(12);
    expect(s.packets.map((p) => p.count)).toEqual([3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2]);
    expect(s.packets.map((p) => p.faceDown)).toEqual([
      ...Array(8).fill(false),
      ...Array(4).fill(true),
    ]);
    expect(s.packets.slice(0, 4).map((p) => p.seat)).toEqual([1, 2, 3, 0]);
    expect(s.candidateFlipMs).toBeNull();
  });

  it("gives the taker 2 plus the candidate in the second deal, with no shuffle lead-in", () => {
    const s = buildDealSchedule("candidateSecond", 0, 2);

    expect(s.packets.map((p) => [p.seat, p.count, p.withCandidate])).toEqual([
      [1, 3, false],
      [2, 2, true],
      [3, 3, false],
      [0, 3, false],
    ]);
    expect(s.packets[0]!.startMs).toBe(0);
    expect(s.candidateFlipMs).toBeNull();
  });

  it("staggers packets and lands each one a flight after it leaves", () => {
    const s = buildDealSchedule("allBeforeBidding", 1);
    s.packets.forEach((p, i) => {
      expect(p.startMs).toBe(MOTION.DEAL_LEAD_IN + i * MOTION.DEAL_PACKET_STAGGER);
      expect(p.landMs).toBe(p.startMs + MOTION.DEAL_PACKET_FLIGHT);
    });
  });

  // The server pushes the next actor's deadline back by these same numbers
  // (server/internal/match/deal_grace.go). If the schedule changes, both the
  // MOTION constants and the Go constants have to move with it.
  it.each<[DealKind, number]>([
    ["candidateFirst", 2100],
    ["allBeforeBidding", 2260],
    ["candidateSecond", 740],
  ])("%s lasts exactly its published duration (%i ms)", (kind, ms) => {
    expect(buildDealSchedule(kind, 0, 1).totalMs).toBe(DEAL_DURATION[kind]);
    expect(DEAL_DURATION[kind]).toBe(ms);
  });

  // The server's match-start grace adds this splash hold (deal_grace.go
  // matchStartSplash) on top of the opening deal.
  it("keeps the match-start splash the server's grace counts on (1500 ms)", () => {
    expect(MOTION.GAME_STARTING_SPLASH).toBe(1500);
  });
});
