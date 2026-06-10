import { create } from "zustand";

import type { MatchState, TrickCard } from "@/shared/types/matchTypes";
import type {
  BelotAnnouncedPayload,
  DeclarationsResolvedPayload,
  EmoteID,
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
  scoreRevealData: HandScoredPayload | null;
  matchEndData: MatchEndPayload | null;
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
  setScoreRevealData: (data: HandScoredPayload | null) => void;
  setMatchEndData: (data: MatchEndPayload | null) => void;
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
    deck: gs.deck ?? [],
    players: gs.players.map((p) => ({
      ...p,
      hand: p.hand ?? [],
      declarations: p.declarations ?? [],
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
  scoreRevealData: null,
  matchEndData: null,
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
    set({ matchState: normalizeMatchState(matchState), roomId: matchState.roomId }),

  setMyPlayerSeat: (myPlayerSeat) => set({ myPlayerSeat }),

  setLoading: (isLoading) => set({ isLoading }),

  setLastError: (lastError) => set({ lastError }),

  setDeclarationReveal: (declarationReveal) => set({ declarationReveal }),

  setBelotReveal: (belotReveal) => set({ belotReveal }),

  setTrumpReveal: (trumpReveal) => set({ trumpReveal }),

  setScoreRevealData: (scoreRevealData) => set({ scoreRevealData }),

  setMatchEndData: (matchEndData) => set({ matchEndData }),

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
