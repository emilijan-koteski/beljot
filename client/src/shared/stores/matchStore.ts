import { create } from "zustand";

import type { MatchState, TrickCard } from "@/shared/types/matchTypes";
import type {
  BelotAnnouncedPayload,
  CoinSettlementPayload,
  DeclarationsResolvedPayload,
  EmoteID,
  HandScoredPayload,
  MatchAbandonedPayload,
  MatchEndPayload,
  SeasonPointsAwardedPayload,
  SurrenderDeclinedPayload,
  SurrenderProposedPayload,
  TrumpSelectedPayload,
} from "@/shared/types/wsEvents";

/**
 * Snapshot of the just-resolved trick, captured before currentTrick clears so
 * the collect animation can still play.
 */
export interface PendingResolvedTrick {
  trick: TrickCard[];
  winnerSeat: number;
  receivedAt: number;
}

/** Per-seat emote slot; receivedAt doubles as a remount key. */
export interface ActiveEmote {
  emote: EmoteID;
  receivedAt: number;
}

export type ActiveEmotesMap = Record<0 | 1 | 2 | 3, ActiveEmote | null>;

/** Per-seat trick-one declaration banner slot; receivedAt doubles as a remount key. */
export interface ActiveDeclare {
  receivedAt: number;
}

export type ActiveDeclaresMap = Record<0 | 1 | 2 | 3, ActiveDeclare | null>;

/** Signals that the server auto-played the local player's card, so the hand-throw animation runs. */
export interface PendingAutoPlayedCard {
  cardId: string;
  receivedAt: number;
}

/** The viewer's Season Points outcome for the match that just ended. */
export type SeasonSettlement = Pick<
  SeasonPointsAwardedPayload,
  "spChange" | "newSeasonSp" | "rankTier" | "rankDivision" | "reason"
>;

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
  coinSettlement: CoinSettlementPayload | null;
  honorSettlement: { before: number; after: number; tier: string } | null;
  seasonSettlement: SeasonSettlement | null;
  matchAbandonedData: MatchAbandonedPayload | null;
  surrenderProposed: SurrenderProposedPayload | null;
  surrenderDeclined: SurrenderDeclinedPayload | null;
  pendingAutoPlayedCard: PendingAutoPlayedCard | null;
  pendingResolvedTrick: PendingResolvedTrick | null;
  activeEmotes: ActiveEmotesMap;
  activeDeclares: ActiveDeclaresMap;
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
  setCoinSettlement: (payload: CoinSettlementPayload | null) => void;
  setHonorSettlement: (payload: { before: number; after: number; tier: string } | null) => void;
  setSeasonSettlement: (payload: SeasonSettlement | null) => void;
  setMatchAbandonedData: (data: MatchAbandonedPayload | null) => void;
  setSurrenderProposed: (payload: SurrenderProposedPayload | null) => void;
  setSurrenderDeclined: (payload: SurrenderDeclinedPayload | null) => void;
  setPendingAutoPlayedCard: (cardId: string | null) => void;
  setPendingResolvedTrick: (snapshot: { trick: TrickCard[]; winnerSeat: number } | null) => void;
  clearPendingResolvedTrick: (receivedAt: number) => void;
  setActiveEmote: (seat: number, emote: EmoteID | null) => void;
  setActiveDeclare: (seat: number, active: boolean) => void;
  setLastEmoteSentAt: (value: number) => void;
  clearGame: () => void;
  reset: () => void;
}

/** Coerces Go's null slices to empty arrays and backfills handCount for servers that omit it. */
function normalizeMatchState(gs: MatchState): MatchState {
  return {
    ...gs,
    currentTrick: gs.currentTrick ?? [],
    players: gs.players.map((p) => ({
      ...p,
      hand: p.hand ?? [],
      declarations: p.declarations ?? [],
      handCount: p.handCount ?? (p.hand ?? []).length,
    })) as MatchState["players"],
  };
}

let lastResolvedTrickStamp = 0;

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
  coinSettlement: null,
  honorSettlement: null,
  seasonSettlement: null,
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
    set({
      matchState: normalizeMatchState(matchState),
      roomId: matchState.roomId,
    }),

  setMyPlayerSeat: (myPlayerSeat) => set({ myPlayerSeat }),

  setLoading: (isLoading) => set({ isLoading }),

  setLastError: (lastError) => set({ lastError }),

  setDeclarationReveal: (declarationReveal) => set({ declarationReveal }),

  setBelotReveal: (belotReveal) => set({ belotReveal }),

  setTrumpReveal: (trumpReveal) => set({ trumpReveal }),

  setScoreRevealData: (scoreRevealData) => set({ scoreRevealData }),

  setMatchEndData: (matchEndData) => set({ matchEndData }),

  setCoinSettlement: (coinSettlement) => set({ coinSettlement }),
  setHonorSettlement: (honorSettlement) => set({ honorSettlement }),
  setSeasonSettlement: (seasonSettlement) => set({ seasonSettlement }),

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
              trick: [...snapshot.trick],
              winnerSeat: snapshot.winnerSeat,
              receivedAt: (lastResolvedTrickStamp = Math.max(
                Date.now(),
                lastResolvedTrickStamp + 1,
              )),
            },
    }),

  clearPendingResolvedTrick: (receivedAt) =>
    set((state) =>
      state.pendingResolvedTrick !== null && state.pendingResolvedTrick.receivedAt === receivedAt
        ? { pendingResolvedTrick: null }
        : state,
    ),

  setActiveEmote: (seat, emote) =>
    set((state) => {
      if (seat !== 0 && seat !== 1 && seat !== 2 && seat !== 3) return state;
      const slot = seat as 0 | 1 | 2 | 3;
      const next: ActiveEmote | null = emote === null ? null : { emote, receivedAt: Date.now() };
      return {
        activeEmotes: { ...state.activeEmotes, [slot]: next } as ActiveEmotesMap,
      };
    }),

  setActiveDeclare: (seat, active) =>
    set((state) => {
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
