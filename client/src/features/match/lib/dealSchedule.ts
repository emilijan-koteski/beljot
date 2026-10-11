import { MOTION } from "@/shared/lib/motion";

/**
 * The three deal shapes the table animates: 3+2 then a candidate flip, 3+3+2 with the last round face-down,
 * and the post-take deal of 3 to each seat with 2 plus the candidate to the taker.
 */
export type DealKind = "candidateFirst" | "allBeforeBidding" | "candidateSecond";

export interface DealPacket {
  seat: number;
  count: number;
  faceDown: boolean;
  withCandidate: boolean;
  startMs: number;
  landMs: number;
}

export interface DealSchedule {
  kind: DealKind;
  dealerSeat: number;
  packets: DealPacket[];
  candidateFlipMs: number | null;
  totalMs: number;
}

const ROUNDS: Record<DealKind, ReadonlyArray<{ count: number; faceDown: boolean }>> = {
  candidateFirst: [
    { count: 3, faceDown: false },
    { count: 2, faceDown: false },
  ],
  allBeforeBidding: [
    { count: 3, faceDown: false },
    { count: 3, faceDown: false },
    { count: 2, faceDown: true },
  ],
  candidateSecond: [{ count: 3, faceDown: false }],
};

/**
 * Build the timed packet list for one deal, round by round from the seat after the dealer, exactly as the server deals.
 * `takerSeat` is required for `candidateSecond` and ignored otherwise.
 */
export function buildDealSchedule(
  kind: DealKind,
  dealerSeat: number,
  takerSeat: number | null = null,
): DealSchedule {
  const leadIn = kind === "candidateSecond" ? 0 : MOTION.DEAL_LEAD_IN;
  const packets: DealPacket[] = [];
  for (const round of ROUNDS[kind]) {
    for (let i = 0; i < 4; i++) {
      const seat = (dealerSeat + 1 + i) % 4;
      const isTaker = kind === "candidateSecond" && seat === takerSeat;
      const startMs = leadIn + packets.length * MOTION.DEAL_PACKET_STAGGER;
      packets.push({
        seat,
        count: isTaker ? 2 : round.count,
        faceDown: round.faceDown,
        withCandidate: isTaker,
        startMs,
        landMs: startMs + MOTION.DEAL_PACKET_FLIGHT,
      });
    }
  }
  const lastLand = packets[packets.length - 1]?.landMs ?? leadIn;
  const candidateFlipMs = kind === "candidateFirst" ? lastLand : null;
  const totalMs =
    candidateFlipMs === null ? lastLand : candidateFlipMs + MOTION.DEAL_CANDIDATE_FLIP;
  return { kind, dealerSeat, packets, candidateFlipMs, totalMs };
}

export const DEAL_DURATION: Record<DealKind, number> = {
  candidateFirst: MOTION.DEAL_DURATION_CANDIDATE_FIRST,
  allBeforeBidding: MOTION.DEAL_DURATION_ALL_BEFORE_BIDDING,
  candidateSecond: MOTION.DEAL_DURATION_CANDIDATE_SECOND,
};
