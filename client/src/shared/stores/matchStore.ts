import { create } from "zustand";

import type { MatchState, TrickCard } from "@/shared/types/matchTypes";
import type {
  BelotAnnouncedPayload,
  CoinSettlementPayload,
  DeclarationsResolvedPayload,
  EmoteID,
  FaceDownRevealedPayload,
  HandScoredPayload,
  MatchAbandonedPayload,
  MatchEndPayload,
  SurrenderDeclinedPayload,
  SurrenderProposedPayload,
  TrumpSelectedPayload,
} from "@/shared/types/wsEvents";

// Snapshot of the just-resolved trick. Captured by the EVENT_TRICK_RESOLVED
// dispatcher BEFORE clearing currentTrick — without this, a same-tick batch
// of event:card_played (#4) + event:trick_resolved would render the trick
// going from 3 → 0 cards directly, leaving no chance to animate the four
// cards flowing toward the winner. The TrickArea + CardFlight overlay read
// this to drive the resolve-glow + collect flight; MatchPage clears it after
// the take animation completes (or on receiving the next event:match_state).
export interface PendingResolvedTrick {
  trick: TrickCard[];
  winnerSeat: number;
  /** Stamped on capture so consumers can debounce duplicate captures during
   *  rapid trick cycles. */
  receivedAt: number;
}

// Per-seat ephemeral emote slot. The receivedAt stamp doubles as a remount
// key so a second emote on the same seat replaces the first cleanly.
export interface ActiveEmote {
  emote: EmoteID;
  receivedAt: number;
}

export type ActiveEmotesMap = Record<0 | 1 | 2 | 3, ActiveEmote | null>;

// Per-seat ephemeral "has a declaration" banner slot (trick 1). Mirrors the
// emote pattern: the receivedAt stamp doubles as a remount key, and the
// banner component clears its own slot via its auto-dismiss timer.
export interface ActiveDeclare {
  receivedAt: number;
}

export type ActiveDeclaresMap = Record<0 | 1 | 2 | 3, ActiveDeclare | null>;

// Transient signal that the server auto-played a card for the local player.
// Dispatcher writes this on EVENT_CARD_PLAYED with autoPlayed=true and
// payload.playerSeat === myPlayerSeat; MatchPage observes it to drive the
// hand-throw animation that handlePlayCard would have triggered for a manual
// click. The receivedAt stamp doubles as a remount key so two consecutive
// auto-plays of the same card (rare but possible across hands) replay the
// animation cleanly.
export interface PendingAutoPlayedCard {
  cardId: string;
  receivedAt: number;
}

export interface MatchStoreState {
  matchState: MatchState | null;
  myPlayerSeat: number | null;
  roomId: number | null;
  isLoading: boolean;
  lastError: string | null;
  declarationReveal: DeclarationsResolvedPayload | null;
  belotReveal: BelotAnnouncedPayload | null;
  trumpReveal: TrumpSelectedPayload | null;
  // The viewer's OWN two face-down cards, turned up when Croatian bidding
  // round 1 is passed out. Kept in its own slice rather than merged into
  // matchState.players[me].hand because the server deliberately never puts
  // them in any snapshot — a later match_state would wipe them straight back
  // out. MatchPage merges this into the rendered hand; setMatchState drops it
  // once bidding has RESOLVED (see the predicate there), by which point the
  // authoritative hand already contains the cards.
  faceDownReveal: FaceDownRevealedPayload | null;
  scoreRevealData: HandScoredPayload | null;
  matchEndData: MatchEndPayload | null;
  // Story 9.2: the per-human coin settlement that arrives right after
  // event:match_end. Stored (rather than toasted) so the won/lost amount can
  // be shown inside the end-of-match score dialog. Null for free (0 buy-in)
  // matches, which emit no settlement event.
  coinSettlement: CoinSettlementPayload | null;
  // Honour movement for the result overlay (honour redesign R7). event:honor_updated
  // lands at exactly the moment the match ends and was previously discarded
  // visually — the score changed and nothing on screen said so.
  //
  // `before` is captured from authStore at the instant the event arrives, because
  // the payload carries only the NEW score. That is enough for "95 -> 96" without
  // widening the WS contract (and its six drift-gate touchpoints).
  //
  // Reset in BOTH the match_end and match_abandoned handlers, exactly as the
  // comment on the honor_updated handler in useWsDispatch demands: the abandoning
  // player is the one who gets no follow-up event, so a stale flourish would
  // otherwise show them someone else's last movement.
  honorSettlement: { before: number; after: number; tier: string } | null;
  matchAbandonedData: MatchAbandonedPayload | null;
  surrenderProposed: SurrenderProposedPayload | null;
  surrenderDeclined: SurrenderDeclinedPayload | null;
  pendingAutoPlayedCard: PendingAutoPlayedCard | null;
  pendingResolvedTrick: PendingResolvedTrick | null;
  activeEmotes: ActiveEmotesMap;
  activeDeclares: ActiveDeclaresMap;
  // Monotonic timestamp (performance.now()) of the most recent emote sent
  // from this client. Lifted out of EmotePickerButton's local useState so
  // the picker's cooldown survives mount/unmount across phase transitions
  // (D107). performance.now() is monotonic so OS clock backsteps cannot
  // lock the picker for arbitrary time (D108). 0 means "no emote sent".
  lastEmoteSentAt: number;

  setMatchState: (state: MatchState) => void;
  setMyPlayerSeat: (seat: number) => void;
  setLoading: (loading: boolean) => void;
  setLastError: (error: string | null) => void;
  setDeclarationReveal: (payload: DeclarationsResolvedPayload | null) => void;
  setBelotReveal: (payload: BelotAnnouncedPayload | null) => void;
  setTrumpReveal: (payload: TrumpSelectedPayload | null) => void;
  setFaceDownReveal: (payload: FaceDownRevealedPayload | null) => void;
  setScoreRevealData: (data: HandScoredPayload | null) => void;
  setMatchEndData: (data: MatchEndPayload | null) => void;
  setCoinSettlement: (payload: CoinSettlementPayload | null) => void;
  setHonorSettlement: (payload: { before: number; after: number; tier: string } | null) => void;
  setMatchAbandonedData: (data: MatchAbandonedPayload | null) => void;
  setSurrenderProposed: (payload: SurrenderProposedPayload | null) => void;
  setSurrenderDeclined: (payload: SurrenderDeclinedPayload | null) => void;
  setPendingAutoPlayedCard: (cardId: string | null) => void;
  setPendingResolvedTrick: (snapshot: { trick: TrickCard[]; winnerSeat: number } | null) => void;
  setActiveEmote: (seat: number, emote: EmoteID | null) => void;
  setActiveDeclare: (seat: number, active: boolean) => void;
  setLastEmoteSentAt: (value: number) => void;
  clearGame: () => void;
  reset: () => void;
}

// Go JSON serializes nil slices as `null`. Coerce the nullable array fields to
// empty arrays so every consumer can iterate without a null guard.
function normalizeMatchState(gs: MatchState): MatchState {
  return {
    ...gs,
    currentTrick: gs.currentTrick ?? [],
    players: gs.players.map((p) => ({
      ...p,
      hand: p.hand ?? [],
      declarations: p.declarations ?? [],
      // Backfill for the rolling-deploy window: a server that predates
      // Story 12.10 sends full hands and no handCount, and the live path CASTS
      // the payload (never parses), so the required field would be undefined
      // for every consumer. Against such a server hand.length IS the count;
      // against a current one this is a no-op.
      handCount: p.handCount ?? (p.hand ?? []).length,
    })) as MatchState["players"],
  };
}

const initialState = {
  matchState: null,
  myPlayerSeat: null,
  roomId: null,
  isLoading: false,
  lastError: null,
  declarationReveal: null,
  belotReveal: null,
  trumpReveal: null,
  faceDownReveal: null,
  scoreRevealData: null,
  matchEndData: null,
  coinSettlement: null,
  honorSettlement: null,
  matchAbandonedData: null,
  surrenderProposed: null,
  surrenderDeclined: null,
  pendingAutoPlayedCard: null,
  pendingResolvedTrick: null,
  activeEmotes: { 0: null, 1: null, 2: null, 3: null } as ActiveEmotesMap,
  activeDeclares: { 0: null, 1: null, 2: null, 3: null } as ActiveDeclaresMap,
  lastEmoteSentAt: 0,
};

export const useMatchStore = create<MatchStoreState>((set) => ({
  ...initialState,

  setMatchState: (matchState) =>
    set((state) => ({
      matchState: normalizeMatchState(matchState),
      roomId: matchState.roomId,
      // Drop the face-down reveal once bidding has RESOLVED — at that moment
      // the engine folds those cards into the authoritative hand, so keeping
      // the slice would render them twice.
      //
      // The test is "trump is set", NOT "phase is literally bidding". Bidding
      // can be interrupted without resolving: a pause moves the phase to
      // `paused` and any seat dropping moves it to `disconnected`, and both are
      // broadcast to all four seats — so a phase test would wipe the viewer's
      // two cards on someone else's pause or drop, and only a seat that
      // actually reconnects gets a replay. trumpSuit is nil for the whole of
      // bidding (and is reset to nil by startNewHand / reshuffleAndRedeal) and
      // is set by the same engine step that merges the cards in, so it is
      // exactly the "has bidding resolved" signal.
      faceDownReveal: matchState.trumpSuit === null ? state.faceDownReveal : null,
    })),

  setMyPlayerSeat: (myPlayerSeat) => set({ myPlayerSeat }),

  setLoading: (isLoading) => set({ isLoading }),

  setLastError: (lastError) => set({ lastError }),

  setDeclarationReveal: (declarationReveal) => set({ declarationReveal }),

  setBelotReveal: (belotReveal) => set({ belotReveal }),

  setTrumpReveal: (trumpReveal) => set({ trumpReveal }),

  setFaceDownReveal: (faceDownReveal) => set({ faceDownReveal }),

  setScoreRevealData: (scoreRevealData) => set({ scoreRevealData }),

  setMatchEndData: (matchEndData) => set({ matchEndData }),

  setCoinSettlement: (coinSettlement) => set({ coinSettlement }),
  setHonorSettlement: (honorSettlement) => set({ honorSettlement }),

  setMatchAbandonedData: (matchAbandonedData) => set({ matchAbandonedData }),

  setSurrenderProposed: (surrenderProposed) => set({ surrenderProposed }),

  setSurrenderDeclined: (surrenderDeclined) => set({ surrenderDeclined }),

  setPendingAutoPlayedCard: (cardId) =>
    set({
      pendingAutoPlayedCard: cardId === null ? null : { cardId, receivedAt: Date.now() },
    }),

  setPendingResolvedTrick: (snapshot) =>
    set({
      pendingResolvedTrick:
        snapshot === null
          ? null
          : {
              // Shallow-clone the trick array so the snapshot doesn't share
              // its backing storage with `matchState.currentTrick` — the
              // dispatcher zeroes that array immediately after this setter,
              // and we want the snapshot to remain a stable, isolated
              // snapshot of the just-resolved trick.
              trick: [...snapshot.trick],
              winnerSeat: snapshot.winnerSeat,
              receivedAt: Date.now(),
            },
    }),

  setActiveEmote: (seat, emote) =>
    set((state) => {
      // Defensive: out-of-range seat is a noop. The dispatcher already
      // validates this server payload, but the setter must not corrupt the
      // map shape if a test or stray caller passes a bad index.
      if (seat !== 0 && seat !== 1 && seat !== 2 && seat !== 3) return state;
      const slot = seat as 0 | 1 | 2 | 3;
      const next: ActiveEmote | null = emote === null ? null : { emote, receivedAt: Date.now() };
      return {
        activeEmotes: { ...state.activeEmotes, [slot]: next } as ActiveEmotesMap,
      };
    }),

  setActiveDeclare: (seat, active) =>
    set((state) => {
      // Defensive: out-of-range seat is a noop, same as setActiveEmote.
      if (seat !== 0 && seat !== 1 && seat !== 2 && seat !== 3) return state;
      const slot = seat as 0 | 1 | 2 | 3;
      const next: ActiveDeclare | null = active ? { receivedAt: Date.now() } : null;
      return {
        activeDeclares: { ...state.activeDeclares, [slot]: next } as ActiveDeclaresMap,
      };
    }),

  setLastEmoteSentAt: (lastEmoteSentAt) => set({ lastEmoteSentAt }),

  clearGame: () => set(initialState),

  reset: () => set(initialState),
}));
