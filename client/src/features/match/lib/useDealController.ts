import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { playSfx } from "@/shared/audio/audioEngine";
import type { Card, MatchState, Phase } from "@/shared/types/matchTypes";

import type { CardFlightDescriptor, FlightRect } from "../components/CardFlight";
import { CARD_SIZES, cardBox, SEAT_STACK_CARD_WIDTH } from "./cardFace";
import { buildDealSchedule, type DealKind, type DealSchedule } from "./dealSchedule";

/**
 * What the deal controller remembers about one table state. Only the fields a
 * deal can change are kept, so an ordinary card play never looks like a deal.
 */
export interface DealObservation {
  /** `handNumber:dealerSeat` — the identity of one deal. A reshuffle keeps the
   *  hand number and rotates the dealer, so it is a new deal too. */
  dealId: string;
  phase: Phase;
  handNumber: number;
  dealerSeat: number;
  biddingRound: number;
  biddingPassCount: number;
  candidate: Card | null;
  trumpSuitSet: boolean;
  trumpCallerSeat: number | null;
  /** Cards each seat physically holds: open plus face-down. */
  seatCounts: [number, number, number, number];
  /** The viewer's own open cards, in the server's (deal) order. */
  myHandIds: string[];
}

export interface DetectedDeal {
  id: string;
  kind: DealKind;
  dealerSeat: number;
  takerSeat: number | null;
  isReshuffle: boolean;
  /** Cards each seat held before this deal (all zero for an opening deal). */
  baseCounts: [number, number, number, number];
  /** The viewer's cards before this deal; null for an opening deal. */
  baseHandIds: string[] | null;
  /** The face-up candidate that flies to the taker in the second deal. */
  candidate: Card | null;
}

export interface ActiveDeal extends DetectedDeal {
  schedule: DealSchedule;
  /** Date.now() when the deal started. */
  startedAt: number;
  /** Packets landed so far. */
  landed: number;
  /** The candidate has turned face-up (opening candidate deal). */
  flipped: boolean;
  done: boolean;
}

function dealRunId(deal: ActiveDeal): string {
  return `${deal.id}@${deal.startedAt}`;
}

function cardIdOf(card: Card): string {
  return `${card.rank}${card.suit}`;
}

export function observeDeal(state: MatchState, mySeat: number): DealObservation {
  const counts = state.players.map(
    (p) => (p.handCount ?? p.hand.length) + (p.faceDownCount ?? 0),
  ) as [number, number, number, number];
  const me = state.players.find((p) => p.seat === mySeat);
  return {
    dealId: `${state.handNumber}:${state.dealerSeat}`,
    phase: state.phase,
    handNumber: state.handNumber,
    dealerSeat: state.dealerSeat,
    biddingRound: state.biddingRound,
    biddingPassCount: state.biddingPassCount,
    candidate: state.trumpCandidate,
    trumpSuitSet: state.trumpSuit !== null,
    trumpCallerSeat: state.trumpCallerSeat,
    seatCounts: counts,
    myHandIds: (me?.hand ?? []).map(cardIdOf),
  };
}

/** True for a hand whose opening deal has only just happened: bidding has not
 *  moved yet, so nobody has seen the table in this state. */
function isFreshDeal(o: DealObservation): boolean {
  return (
    (o.phase === "dealing" || o.phase === "bidding") &&
    o.biddingRound === 1 &&
    o.biddingPassCount === 0 &&
    !o.trumpSuitSet
  );
}

function openingDeal(o: DealObservation, isReshuffle: boolean): DetectedDeal {
  return {
    id: o.dealId,
    // The shape comes from what the server dealt, never from the variant: a
    // face-up candidate means the two-stage deal, none means everything was
    // dealt before bidding.
    kind: o.candidate !== null ? "candidateFirst" : "allBeforeBidding",
    dealerSeat: o.dealerSeat,
    takerSeat: null,
    isReshuffle,
    baseCounts: [0, 0, 0, 0],
    baseHandIds: null,
    candidate: o.candidate,
  };
}

/**
 * Decide whether the table just went through a deal worth animating.
 *
 * `prev` is null for the first state the table shows. That one animates only
 * when `animateOpening` says the viewer arrived for a fresh match (straight
 * from the room): a reload or reconnect mounts into whatever is on the table
 * and never replays a deal.
 */
export function detectDeal(
  prev: DealObservation | null,
  cur: DealObservation,
  animateOpening: boolean,
): DetectedDeal | null {
  if (prev === null) {
    return animateOpening && cur.handNumber === 1 && isFreshDeal(cur)
      ? openingDeal(cur, false)
      : null;
  }
  if (prev.dealId !== cur.dealId) {
    if (!isFreshDeal(cur)) return null;
    return openingDeal(cur, prev.handNumber === cur.handNumber);
  }
  // Second deal: the candidate was on the table during bidding and has now
  // gone to the taker, together with the reserve.
  const tookCandidate =
    prev.phase === "bidding" &&
    prev.candidate !== null &&
    cur.candidate === null &&
    cur.trumpCallerSeat !== null &&
    (cur.phase === "playing" || cur.phase === "declaring");
  const grew =
    cur.seatCounts.reduce((a, b) => a + b, 0) > prev.seatCounts.reduce((a, b) => a + b, 0);
  if (tookCandidate && grew) {
    return {
      id: `${cur.dealId}:2`,
      kind: "candidateSecond",
      dealerSeat: cur.dealerSeat,
      takerSeat: cur.trumpCallerSeat,
      isReshuffle: false,
      baseCounts: prev.seatCounts,
      baseHandIds: prev.myHandIds,
      candidate: prev.candidate,
    };
  }
  return null;
}

interface Tracker {
  key: string | null;
  last: DealObservation | null;
  deal: ActiveDeal | null;
}

function observationKey(o: DealObservation): string {
  return [o.dealId, o.phase, o.candidate ? cardIdOf(o.candidate) : "-", o.trumpCallerSeat].join(
    "|",
  );
}

function rectFrom(el: Element | null): FlightRect | null {
  if (!el) return null;
  const r = el.getBoundingClientRect();
  if (r.width === 0 && r.height === 0) return null;
  return { left: r.left, top: r.top, width: r.width, height: r.height };
}

/** A card-shaped rect of the given width centred on `around`. */
function cardRectAt(around: FlightRect, width: number, dx = 0, dy = 0): FlightRect {
  const box = cardBox(width);
  return {
    left: around.left + around.width / 2 - box.width / 2 + dx,
    top: around.top + around.height / 2 - box.height / 2 + dy,
    width: box.width,
    height: box.height,
  };
}

/** Every deal flight's id starts with this. */
const DEAL_FLIGHT = "deal-";
/** How many backs a flying packet shows — enough to read as a small stack. */
const PACKET_CARDS_SHOWN = 3;
/** Offset between the backs of one packet. */
const PACKET_FAN_PX = 3;
/** Width of the deck resting at the dealer's seat. */
const DEALER_DECK_W = 36;

function dealFlights(deal: ActiveDeal, mySeat: number, compact: boolean): CardFlightDescriptor[] {
  const avatar = (seat: number) =>
    rectFrom(
      document.querySelector(
        `[data-testid="player-seat-${seat}"] [data-testid="player-seat-avatar"]`,
      ),
    );
  const origin = avatar(deal.dealerSeat);
  if (!origin) return [];
  const table = rectFrom(document.querySelector('[data-testid="trick-area"]'));
  const flights: CardFlightDescriptor[] = [];
  const elapsed = Date.now() - deal.startedAt;
  deal.schedule.packets.forEach((packet, index) => {
    const isSelf = packet.seat === mySeat;
    const target = isSelf
      ? rectFrom(document.querySelector('[data-testid="hand-cards"]'))
      : (rectFrom(document.querySelector(`[data-seat-deck="${packet.seat}"]`)) ??
        avatar(packet.seat));
    if (!target) return;
    const toWidth = isSelf
      ? CARD_SIZES.lg.width
      : compact
        ? SEAT_STACK_CARD_WIDTH.compact
        : SEAT_STACK_CARD_WIDTH.desktop;
    const delayMs = Math.max(0, packet.startMs - elapsed);
    const shown = Math.min(packet.count, PACKET_CARDS_SHOWN);
    for (let c = 0; c < shown; c++) {
      const fan = c * PACKET_FAN_PX;
      flights.push({
        id: `${DEAL_FLIGHT}${dealRunId(deal)}-${index}-${c}`,
        card: null,
        fromRect: cardRectAt(origin, DEALER_DECK_W, fan, -fan),
        toRect: cardRectAt(target, toWidth, fan, -fan),
        durationMs: packet.landMs - packet.startMs,
        delayMs,
        easing: "cubic-bezier(0.25, 0.8, 0.35, 1)",
      });
    }
    if (packet.withCandidate && deal.candidate && table) {
      flights.push({
        id: `${DEAL_FLIGHT}${dealRunId(deal)}-${index}-candidate`,
        card: deal.candidate,
        fromRect: cardRectAt(table, CARD_SIZES.md.width),
        toRect: cardRectAt(target, toWidth),
        durationMs: packet.landMs - packet.startMs,
        delayMs,
        easing: "cubic-bezier(0.25, 0.8, 0.35, 1)",
      });
    }
  });
  return flights;
}

interface DealControllerInput {
  matchState: MatchState | null;
  myPlayerSeat: number | null;
  /** The table is on screen (splash over, state and seat known). Nothing is
   *  observed before, so the first observation is the first thing seen. */
  tableVisible: boolean;
  /** The viewer arrived straight from the room: the first state they see is a
   *  fresh match whose opening deal they have not watched yet. */
  animateOpening: boolean;
  prefersReducedMotion: boolean;
  compact: boolean;
  /** Updates the page's CardFlight overlay: the deal adds its packets when it
   *  starts and takes back any still there when it ends. */
  updateFlights: (update: (flights: CardFlightDescriptor[]) => CardFlightDescriptor[]) => void;
}

export interface DealView {
  /** A deal is animating; the table asks nobody for a decision until it ends. */
  dealing: boolean;
  deal: ActiveDeal | null;
  /** Backs to show at a seat mid-deal, or null to show its real count. */
  seatCardCount: (seat: number) => number | null;
  /** The part of the viewer's own hand that has landed so far. */
  visibleHand: (hand: Card[], faceDownCount: number) => { hand: Card[]; faceDownCount: number };
}

/**
 * Deal controller: watches the table for a deal (opening deal, reshuffle,
 * candidate second deal), runs its schedule, and reports what the table should
 * show at each moment of it.
 *
 * Detection happens during render (the "adjust state when props change"
 * pattern), so the very first frame of a new hand already shows empty seats and
 * no bidding prompt — no flash of the finished deal before the animation.
 * Landings, sounds and the end are driven by timers against the deal's start
 * time, so a re-run effect resumes rather than restarts.
 */
export function useDealController({
  matchState,
  myPlayerSeat,
  tableVisible,
  animateOpening,
  prefersReducedMotion,
  compact,
  updateFlights,
}: DealControllerInput): DealView {
  const [tracker, setTracker] = useState<Tracker>({ key: null, last: null, deal: null });

  let current = tracker;
  if (tableVisible && matchState !== null && myPlayerSeat !== null) {
    const observation = observeDeal(matchState, myPlayerSeat);
    const key = observationKey(observation);
    if (key !== tracker.key) {
      const detected = detectDeal(tracker.last, observation, animateOpening);
      current = {
        key,
        last: observation,
        deal: detected
          ? {
              ...detected,
              schedule: buildDealSchedule(detected.kind, detected.dealerSeat, detected.takerSeat),
              startedAt: Date.now(),
              landed: 0,
              flipped: false,
              done: false,
            }
          : tracker.deal,
      };
      setTracker(current);
    }
  }

  const deal = current.deal;
  // One run of a deal. The deal id alone can recur — four all-pass reshuffles
  // bring the dealer back round, and a new match starts at hand 1 again — so
  // the start time is part of what keys its effects, flights and sounds.
  const runId = deal !== null && !deal.done ? dealRunId(deal) : null;

  // Timers: the shuffle, each packet's landing (with its sound), the candidate
  // flip, and the end. Scheduled against the deal's start, so a re-run resumes.
  useEffect(() => {
    if (runId === null) return;
    const active = tracker.deal;
    if (active === null || dealRunId(active) !== runId) return;
    const elapsed = Date.now() - active.startedAt;
    const timers: number[] = [];
    const at = (ms: number, fn: () => void) => {
      timers.push(window.setTimeout(fn, Math.max(0, ms - elapsed)));
    };
    const update = (fn: (d: ActiveDeal) => ActiveDeal) =>
      setTracker((t) =>
        t.deal !== null && dealRunId(t.deal) === runId ? { ...t, deal: fn(t.deal) } : t,
      );

    if (active.kind !== "candidateSecond") {
      at(0, () => playSfx("deal", { dedupeKey: runId }));
    }
    active.schedule.packets.forEach((packet, index) => {
      at(packet.landMs, () => {
        playSfx("dealPacket", { dedupeKey: `${runId}:${index}` });
        update((d) => ({ ...d, landed: Math.max(d.landed, index + 1) }));
      });
    });
    const flipMs = active.schedule.candidateFlipMs;
    if (flipMs !== null) at(flipMs, () => update((d) => ({ ...d, flipped: true })));
    at(active.schedule.totalMs, () =>
      update((d) => ({ ...d, landed: d.schedule.packets.length, flipped: true, done: true })),
    );
    return () => timers.forEach((id) => clearTimeout(id));
    // Keyed on the run alone: its schedule and start time never change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runId]);

  // Flights: measured once the new hand's table has committed, once per deal.
  // Whatever of them is still in the overlay when the deal ends (a hidden tab
  // never fires animationend) is taken back then.
  const flownRef = useRef<string | null>(null);
  const updateFlightsRef = useRef(updateFlights);
  updateFlightsRef.current = updateFlights;
  useLayoutEffect(() => {
    if (runId === null || prefersReducedMotion || myPlayerSeat === null) return;
    if (flownRef.current === runId) return;
    const active = tracker.deal;
    if (active === null || dealRunId(active) !== runId) return;
    flownRef.current = runId;
    const flights = dealFlights(active, myPlayerSeat, compact);
    if (flights.length > 0) updateFlightsRef.current((prev) => [...prev, ...flights]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runId]);
  useEffect(() => {
    if (runId !== null) return;
    updateFlightsRef.current((prev) => {
      const next = prev.filter((f) => !f.id.startsWith(DEAL_FLIGHT));
      return next.length === prev.length ? prev : next;
    });
  }, [runId]);

  const dealing = deal !== null && !deal.done;

  const landedFor = (seat: number) => {
    let open = 0;
    let faceDown = 0;
    if (deal === null) return { open, faceDown, packets: 0 };
    let packets = 0;
    for (const packet of deal.schedule.packets.slice(0, deal.landed)) {
      if (packet.seat !== seat) continue;
      packets += 1;
      const n = packet.count + (packet.withCandidate ? 1 : 0);
      if (packet.faceDown) faceDown += n;
      else open += n;
    }
    return { open, faceDown, packets };
  };

  return {
    dealing,
    deal,
    seatCardCount: (seat) => {
      if (!dealing || deal === null) return null;
      const { open, faceDown } = landedFor(seat);
      return (deal.baseCounts[seat] ?? 0) + open + faceDown;
    },
    visibleHand: (hand, faceDownCount) => {
      if (!dealing || deal === null || myPlayerSeat === null) return { hand, faceDownCount };
      const landed = landedFor(myPlayerSeat);
      if (deal.baseHandIds !== null) {
        // Second deal: the viewer's earlier cards stay; the new ones appear
        // with their packet.
        if (landed.packets > 0) return { hand, faceDownCount };
        const base = new Set(deal.baseHandIds);
        return { hand: hand.filter((c) => base.has(cardIdOf(c))), faceDownCount };
      }
      return {
        hand: hand.slice(0, landed.open),
        faceDownCount: Math.min(faceDownCount, landed.faceDown),
      };
    },
  };
}
