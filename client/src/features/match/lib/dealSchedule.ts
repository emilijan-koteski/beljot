import { MOTION } from "@/shared/lib/motion";

/**
 * The three deals the table animates, named by shape rather than variant:
 *
 *  - `candidateFirst`: 3 then 2 cards to each seat from the dealer's left,
 *    then the trump candidate turns face-up (Bitola's opening deal).
 *  - `allBeforeBidding`: 3, 3, then 2 face-down cards to each seat
 *    (Croatia's whole deal).
 *  - `candidateSecond`: once the candidate is taken, 3 to each seat and 2 plus
 *    the candidate to the taker, in the same seat order (Bitola's second deal).
 */
export type DealKind = "candidateFirst" | "allBeforeBidding" | "candidateSecond";

export interface DealPacket {
  /** Seat the packet lands at. */
  seat: number;
  /** Cards in the packet, the candidate not included. */
  count: number;
  /** The packet stays face-down in the recipient's hand (the last round of an
   *  all-before-bidding deal). */
  faceDown: boolean;
  /** The taker's packet in the second deal also carries the face-up candidate. */
  withCandidate: boolean;
  /** ms from the start of the deal when the packet leaves the dealer. */
  startMs: number;
  /** ms from the start of the deal when the packet lands. */
  landMs: number;
}

export interface DealSchedule {
  kind: DealKind;
  dealerSeat: number;
  packets: DealPacket[];
  /** ms at which the candidate turns face-up at the table centre, or null when
   *  this deal has no flip. */
  candidateFlipMs: number | null;
  /** ms at which the deal is over and the table may ask for a decision. */
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
  // The taker's slot is the one exception (2 plus the candidate); see below.
  candidateSecond: [{ count: 3, faceDown: false }],
};

/**
 * Build the timed packet list for one deal. Packets go counter-clockwise from
 * the seat after the dealer, one round at a time, exactly as the server deals
 * (game/state.go dealCards, game/bidding.go handlePickTrump).
 *
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

/** The published length of each deal shape (mirrored by the server's grace). */
export const DEAL_DURATION: Record<DealKind, number> = {
  candidateFirst: MOTION.DEAL_DURATION_CANDIDATE_FIRST,
  allBeforeBidding: MOTION.DEAL_DURATION_ALL_BEFORE_BIDDING,
  candidateSecond: MOTION.DEAL_DURATION_CANDIDATE_SECOND,
};
