import i18n from "i18next";
import { useCallback } from "react";
import { toast } from "sonner";

import { handleWsMessage as handleRoomListMessage } from "@/features/lobby/useRoomUpdates";
import { MOTION } from "@/shared/lib/motion";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { MatchState } from "@/shared/types/matchTypes";
import type {
  AutoActionPayload,
  BelotAnnouncedPayload,
  BotAddedPayload,
  BotRemovedPayload,
  CardPlayedPayload,
  ChatMessagePayload,
  CoinSettlementPayload,
  DeclarationsResolvedPayload,
  EmotePayload,
  HandScoredPayload,
  MatchAbandonedPayload,
  MatchEndPayload,
  MatchResumedPayload,
  MatchStartedPayload,
  PlayerDeclaredPayload,
  PlayerDisconnectedPayload,
  PlayerJoinedPayload,
  PlayerLeftPayload,
  PlayerReconnectedPayload,
  PlayerReturnedPayload,
  RoomKickedPayload,
  RoomOwnerChangedPayload,
  SeatUpdatedPayload,
  SurrenderDeclinedPayload,
  SurrenderProposedPayload,
  TrickResolvedPayload,
  TrumpSelectedPayload,
  WsMessage,
} from "@/shared/types/wsEvents";
import {
  EMOTE_IDS,
  ERROR_AUTH_FAILED,
  ERROR_ILLEGAL_PLAY,
  ERROR_INVALID_ACTION,
  ERROR_MATCH_START_FAILED,
  ERROR_NO_ACTIVE_PAUSE,
  ERROR_NOT_ROOM_OWNER,
  ERROR_NOT_YOUR_TURN,
  ERROR_PAUSE_EXHAUSTED,
  ERROR_PLAYER_DISCONNECTED,
  ERROR_SURRENDER_EXHAUSTED,
  ERROR_WRONG_PHASE,
  EVENT_AUTO_ACTION,
  EVENT_BELOT_ANNOUNCED,
  EVENT_CARD_PLAYED,
  EVENT_COIN_SETTLEMENT,
  EVENT_DECLARATIONS_RESOLVED,
  EVENT_HAND_SCORED,
  EVENT_MATCH_ABANDONED,
  EVENT_MATCH_END,
  EVENT_MATCH_PAUSED,
  EVENT_MATCH_RESUMED,
  EVENT_MATCH_STATE,
  EVENT_PLAYER_DECLARED,
  EVENT_PLAYER_DISCONNECTED,
  EVENT_PLAYER_RECONNECTED,
  EVENT_SURRENDER_DECLINED,
  EVENT_SURRENDER_PROPOSED,
  EVENT_TRICK_RESOLVED,
  EVENT_TRUMP_SELECTED,
  SYSTEM_AUTHENTICATED,
  SYSTEM_BOT_ADDED,
  SYSTEM_BOT_REMOVED,
  SYSTEM_CHAT_MESSAGE,
  SYSTEM_EMOTE,
  SYSTEM_MATCH_STARTED,
  SYSTEM_PLAYER_JOINED,
  SYSTEM_PLAYER_LEFT,
  SYSTEM_PLAYER_RETURNED,
  SYSTEM_ROOM_CREATED,
  SYSTEM_ROOM_KICKED,
  SYSTEM_ROOM_OWNER_CHANGED,
  SYSTEM_ROOM_UPDATED,
  SYSTEM_SEAT_UPDATED,
} from "@/shared/types/wsEvents";

export function useWsDispatch() {
  const dispatch = useCallback((message: WsMessage) => {
    const { type } = message;

    // Auth events are handled by useWebSocket directly
    if (type === SYSTEM_AUTHENTICATED || type === ERROR_AUTH_FAILED) {
      return;
    }

    const prefix = type.indexOf(":") >= 0 ? type.slice(0, type.indexOf(":")) : "";

    switch (prefix) {
      case "event":
        dispatchGameEvent(message);
        break;
      case "system":
        dispatchSystemEvent(message);
        break;
      case "error":
        dispatchErrorEvent(message);
        break;
      default:
        console.warn("WS: unknown event prefix", type);
    }
  }, []);

  return dispatch;
}

// Test-only hook retained for API stability. The dispatcher no longer keeps
// module-level reveal state (the reveal lifecycle is owned by the reveal
// components' own countdown + the MatchPage overlay-clear effect), so this is
// a no-op. Kept exported so existing test setup that calls it doesn't break.
export function __resetWsDispatchStateForTests(): void {}

function dispatchGameEvent(message: WsMessage): void {
  const store = useMatchStore.getState();
  const { type } = message;

  if (type === EVENT_MATCH_STATE) {
    const matchState = message.payload as MatchState;
    // NOTE: declarationReveal / belotReveal are intentionally NOT cleared here.
    // A reveal must live for its full on-screen countdown (8 s) or until the
    // viewer dismisses it — NOT be wiped by an unrelated trailing match_state,
    // e.g. the snapshot another player's card-play broadcasts mid-reveal (that
    // was the bug: the reveal closed the instant anyone played). The reveal
    // components own their own dismissal; MatchPage clears a stale/orphaned
    // reveal when an overlay covers the table (disconnect / pause / hand-end),
    // which is the only situation a reveal can be left up after the moment it
    // describes has passed (D69 / D71 reconnect case included — the disconnect
    // overlay clears it before the resync snapshot arrives).
    //
    // NOTE: pendingResolvedTrick is intentionally NOT cleared here. The
    // server emits a trailing event:match_state ~immediately after every
    // event:trick_resolved (see live_match.go); clearing the snapshot here
    // would tear it down before the collect animation can run. MatchPage's
    // pendingResolvedTrick effect installs a fallback clear timer scoped
    // to the animation duration, which also handles the reconnect-mid-
    // collect case (snapshot survives reconnect, the remounted effect
    // re-arms the timer and the snapshot clears within ~1.6s of remount).
    store.setMatchState(matchState);
    return;
  }

  if (type === EVENT_CARD_PLAYED) {
    // Card played is an incremental event — update trick only.
    // activePlayerSeat is NOT computed client-side (no local game logic).
    // The server will send the authoritative state via event:match_state or
    // the next action's broadcast will carry the correct active player.
    const payload = message.payload as CardPlayedPayload;
    const current = store.matchState;
    if (current) {
      if (!payload.cardId || payload.cardId.length < 2) return;
      const rank = payload.cardId[0];
      const suit = payload.cardId[1];
      const isLocalAutoPlay = payload.autoPlayed && payload.playerSeat === store.myPlayerSeat;
      store.setMatchState({
        ...current,
        currentTrick: [
          ...(current.currentTrick ?? []),
          {
            card: {
              rank: rank as "7" | "8" | "9" | "T" | "J" | "Q" | "K" | "A",
              suit: suit as "S" | "H" | "D" | "C",
            },
            playerSeat: payload.playerSeat,
          },
        ],
        // Optimistically clear pendingBelotSeat when the server auto-plays for
        // the local player while a Belot prompt is open. Without this, a stale
        // click on Announce/Decline races against the trailing event:match_state
        // and lands on the server with a wrong-phase error toast.
        pendingBelotSeat: isLocalAutoPlay ? null : current.pendingBelotSeat,
      });
    }

    // Auto-play toast notification
    if (payload.autoPlayed) {
      toast.info(i18n.t("match.timer.autoPlayed", { card: payload.cardId }), {
        duration: MOTION.TOAST_INFO,
      });
      // Signal MatchPage to drive the same hand-throw animation that
      // handlePlayCard runs for a manual click — otherwise the auto-played
      // card disappears from the local hand without an exit animation.
      if (payload.playerSeat === store.myPlayerSeat) {
        store.setPendingAutoPlayedCard(payload.cardId);
      }
    }
    return;
  }

  if (type === EVENT_TRICK_RESOLVED) {
    const payload = message.payload as TrickResolvedPayload;
    const current = store.matchState;
    if (current) {
      // Snapshot the full 4-card trick BEFORE clearing currentTrick. By the
      // time this dispatcher runs, the preceding event:card_played for the
      // 4th card has already pushed it into current.currentTrick — but
      // React batches both store updates into one render, so without a
      // snapshot the UI sees currentTrick jump 3 → 0 and the collect
      // animation never has a chance to run. The snapshot decouples the
      // collect animation from currentTrick's authoritative-but-transient
      // state. MatchPage clears it after the take flight completes.
      if (current.currentTrick && current.currentTrick.length > 0) {
        store.setPendingResolvedTrick({
          trick: current.currentTrick,
          winnerSeat: payload.winnerSeat,
        });
      }
      // Clear turnExpiresAt here so the previous active seat's TimerRing stops
      // ticking the moment the trick resolves. The trailing event:match_state
      // will set the next turn's deadline; until then, no seat counts down.
      // Without this, the just-played seat keeps decrementing under the
      // trick-resolve animation and can flip urgent-red on a slow snapshot.
      store.setMatchState({
        ...current,
        currentTrick: [],
        trickNumber: current.trickNumber + 1,
        trickWinnerSeat: payload.winnerSeat,
        turnExpiresAt: null,
      });
    }
    return;
  }

  if (type === EVENT_HAND_SCORED) {
    const payload = message.payload as HandScoredPayload;
    const current = store.matchState;
    if (current) {
      // Clear per-hand fields here so the ScorePanel's "this hand" line disappears
      // immediately. The follow-up event:match_state will replace them with the
      // new-hand defaults, but zeroing now avoids a flicker of stale potentials.
      store.setMatchState({
        ...current,
        teamScores: [payload.teamAMatchScore, payload.teamBMatchScore],
        handPoints: [0, 0],
        declarationPoints: [0, 0],
        belotPoints: [0, 0],
      });
    }
    store.setScoreRevealData(payload);
    return;
  }

  if (type === EVENT_MATCH_END) {
    const payload = message.payload as MatchEndPayload;
    const current = store.matchState;
    if (current) {
      store.setMatchState({
        ...current,
        phase: "match_end",
        teamScores: [payload.teamAFinalScore, payload.teamBFinalScore],
      });
    }
    store.setMatchEndData(payload);
    // Reset any prior settlement so the result dialog never shows a stale
    // coin delta — the matching event:coin_settlement (if this match had a
    // buy-in) arrives immediately after per the ordering contract and sets it.
    store.setCoinSettlement(null);
    return;
  }

  if (type === EVENT_COIN_SETTLEMENT) {
    // Story 9.2: per-human coin settlement, arriving right after match_end while
    // still on the match page. Update the persisted wallet balance on authStore
    // (balance lives on authStore.user, NOT gameStore — it must survive the
    // later navigation away that wipes gameStore) and stash the settlement on
    // the match store so the end-of-match score dialog reports the won/lost
    // amount. No toast — the result dialog is the single place that shows it.
    const payload = message.payload as CoinSettlementPayload;
    // Defensive validation — Go zero values are real values, so guard on type,
    // not truthiness (a 0 delta/balance is legitimate).
    if (
      !Number.isInteger(payload.coinDelta) ||
      !Number.isInteger(payload.newBalance) ||
      !Number.isInteger(payload.pot)
    ) {
      return;
    }
    const auth = useAuthStore.getState();
    if (auth.user) {
      auth.setUser({ ...auth.user, walletBalance: payload.newBalance });
    }
    store.setCoinSettlement(payload);
    return;
  }

  if (type === EVENT_TRUMP_SELECTED) {
    // Drives the table-wide TrumpReveal dialog. The full event:match_state that
    // follows carries the persistent fields (trumpSuit, trumpCallerSeat, phase,
    // activePlayerSeat); the cardId here is the originally face-up
    // trumpCandidate the picker absorbed and lives only on this event.
    const payload = message.payload as TrumpSelectedPayload;
    if (!payload.cardId || payload.cardId.length < 2) {
      console.warn("WS: ignoring malformed event:trump_selected payload", payload);
      return;
    }
    store.setTrumpReveal(payload);
    return;
  }

  if (type === EVENT_DECLARATIONS_RESOLVED) {
    const payload = message.payload as DeclarationsResolvedPayload;
    store.setDeclarationReveal(payload);
    // Full game state update follows via event:match_state. That trailing
    // snapshot (and any later one) leaves this reveal untouched — see the
    // EVENT_MATCH_STATE branch.
    return;
  }

  if (type === EVENT_PLAYER_DECLARED) {
    // Trick-1 "who declared" announcement — ephemeral per-seat banner, not
    // part of MatchState. Mirrors the system:emote defensive validation; the
    // banner component clears its own slot via its auto-dismiss timer.
    const payload = message.payload as PlayerDeclaredPayload;
    if (
      typeof payload?.playerSeat !== "number" ||
      payload.playerSeat < 0 ||
      payload.playerSeat > 3
    ) {
      console.warn("WS: ignoring malformed event:player_declared payload", payload);
      return;
    }
    if (useMatchStore.getState().matchState === null) return;
    useMatchStore.getState().setActiveDeclare(payload.playerSeat, true);
    return;
  }

  if (type === EVENT_BELOT_ANNOUNCED) {
    const payload = message.payload as BelotAnnouncedPayload;
    // Mirror the card_played guard: drop malformed payloads rather than propagate
    // an empty cardId to BelotReveal where rendering/rank detection would silently break.
    if (!payload.cardId || payload.cardId.length < 2) return;
    store.setBelotReveal(payload);
    // Full state update follows via event:match_state; it leaves this reveal
    // untouched (same as declarations_resolved above).
    return;
  }

  if (type === EVENT_MATCH_PAUSED) {
    // Informational — the full event:match_state that follows carries pause state
    return;
  }

  if (type === EVENT_AUTO_ACTION) {
    if (store.matchState === null) return;
    const payload = message.payload as AutoActionPayload;
    if (
      typeof payload?.playerSeat !== "number" ||
      !Number.isInteger(payload.playerSeat) ||
      payload.playerSeat < 0 ||
      payload.playerSeat > 3 ||
      (payload?.type !== "pass_trump" &&
        payload?.type !== "skip_declare" &&
        payload?.type !== "skip_belot")
    ) {
      console.warn("WS: ignoring malformed event:auto_action payload", payload);
      return;
    }
    const playerName =
      store.matchState.players[payload.playerSeat]?.username ?? `Player ${payload.playerSeat + 1}`;
    const i18nKey =
      payload.type === "pass_trump"
        ? "match.timer.autoPassed"
        : payload.type === "skip_declare"
          ? "match.timer.autoSkippedDeclare"
          : "match.timer.autoSkippedBelot";
    toast.info(i18n.t(i18nKey, { player: playerName }), { duration: 3000 });
    return;
  }

  if (type === EVENT_MATCH_RESUMED) {
    const payload = message.payload as MatchResumedPayload;
    if (payload.ownerOverride) {
      toast.info(i18n.t("match.pause.ownerResumedToast"), { duration: 3000 });
    }
    // Full state update follows via event:match_state
    return;
  }

  if (type === EVENT_PLAYER_DISCONNECTED) {
    const payload = message.payload as PlayerDisconnectedPayload;
    const current = store.matchState;
    const playerName =
      current?.players[payload.playerSeat]?.username ??
      payload.username ??
      `Player ${payload.playerSeat + 1}`;
    toast.warning(i18n.t("match.disconnect.playerDisconnected", { player: playerName }), {
      duration: MOTION.TOAST_LONG,
    });
    // Full state update follows via event:match_state
    return;
  }

  if (type === EVENT_PLAYER_RECONNECTED) {
    const payload = message.payload as PlayerReconnectedPayload;
    const current = store.matchState;
    const playerName =
      current?.players[payload.playerSeat]?.username ?? `Player ${payload.playerSeat + 1}`;
    toast.success(i18n.t("match.disconnect.playerReconnected", { player: playerName }), {
      duration: MOTION.TOAST_INFO,
    });
    // Full state update follows via event:match_state
    return;
  }

  if (type === EVENT_MATCH_ABANDONED) {
    const payload = message.payload as MatchAbandonedPayload;
    store.setMatchAbandonedData(payload);
    return;
  }

  if (type === EVENT_SURRENDER_PROPOSED) {
    // Defence in depth: only surface a proposal if the user is in an active
    // match (Story 8.1 dispatcher hardening pattern).
    if (store.matchState === null) return;
    const payload = message.payload as SurrenderProposedPayload;
    store.setSurrenderProposed(payload);
    // Full game state update follows via event:match_state — clears the
    // pending flag on resolve.
    return;
  }

  if (type === EVENT_SURRENDER_DECLINED) {
    if (store.matchState === null) return;
    const payload = message.payload as SurrenderDeclinedPayload;
    store.setSurrenderDeclined(payload);
    toast.info(i18n.t("match.surrender.declinedToast"), { duration: 3000 });
    // Full game state update follows via event:match_state — clears
    // surrenderProposerSeat so the prompt/banner unmount.
    return;
  }
}

function dispatchSystemEvent(message: WsMessage): void {
  const { type } = message;

  // Room list updates — delegate to existing useRoomUpdates handler
  if (type === SYSTEM_ROOM_CREATED || type === SYSTEM_ROOM_UPDATED) {
    handleRoomListMessage(new MessageEvent("message", { data: JSON.stringify(message) }));
    return;
  }

  // Lobby grid cache updates — keep the ["rooms","waiting"] cache live for
  // playerCount + per-seat names without touching the roomStore path
  // below (which only fires for the user's currently viewed room). Falls
  // through so the roomStore branch still runs.
  if (
    type === SYSTEM_PLAYER_JOINED ||
    type === SYSTEM_PLAYER_LEFT ||
    type === SYSTEM_SEAT_UPDATED
  ) {
    handleRoomListMessage(new MessageEvent("message", { data: JSON.stringify(message) }));
    // intentional fall-through to the per-room store dispatch below
  }

  // Room lobby updates — dispatch to roomStore (only if event matches the currently viewed room)
  if (type === SYSTEM_PLAYER_JOINED) {
    const payload = message.payload as PlayerJoinedPayload;
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.addPlayer(
      {
        id: payload.userId, // Use userId as a client-side ID (fixes D24: no longer hardcodes 0)
        roomId: payload.roomId,
        userId: payload.userId,
        username: payload.username,
        seat: null,
        team: null,
        isBot: false,
        createdAt: new Date().toISOString(),
      },
      payload.playerCount,
    );
    // A joiner is "present" by definition — without this, the owner's Start gate
    // (all seated humans present) would never satisfy for normally-joined rooms,
    // since joins arrive as player_joined, not player_returned.
    store.markReturned(payload.userId);
    return;
  }

  if (type === SYSTEM_PLAYER_LEFT) {
    const payload = message.payload as PlayerLeftPayload;
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.removePlayer(payload.userId, payload.playerCount, payload.newOwnerId);
    return;
  }

  if (type === SYSTEM_SEAT_UPDATED) {
    const payload = message.payload as SeatUpdatedPayload;
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.updatePlayerSeat(payload.userId, payload.seat, payload.team, payload.previousSeat);
    return;
  }

  // Bot seating (Story 10.3). Room-lobby events are not zod-validated (see
  // wsEvents.schemas.ts coverage note) — guard the fields defensively so
  // protocol drift can't inject bogus seats into the store. `typeof x ===
  // "number"` alone admits NaN, fractions, and out-of-range values; a seat
  // that isn't an integer 0–3 could insert a phantom player no tile renders
  // and no removal ever matches.
  if (type === SYSTEM_BOT_ADDED) {
    const payload = message.payload as BotAddedPayload;
    if (
      typeof payload?.roomId !== "number" ||
      !Number.isInteger(payload?.seat) ||
      payload.seat < 0 ||
      payload.seat > 3 ||
      typeof payload?.team !== "string"
    ) {
      return;
    }
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.addBot(payload.roomId, payload.seat, payload.team);
    return;
  }

  if (type === SYSTEM_BOT_REMOVED) {
    const payload = message.payload as BotRemovedPayload;
    if (
      typeof payload?.roomId !== "number" ||
      !Number.isInteger(payload?.seat) ||
      payload.seat < 0 ||
      payload.seat > 3
    ) {
      return;
    }
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.removeBotBySeat(payload.seat);
    return;
  }

  if (type === SYSTEM_PLAYER_RETURNED) {
    const payload = message.payload as PlayerReturnedPayload;
    // Defensive guards mirror the bot handlers: reject NaN/fractional ids so
    // protocol drift can't pollute the presence set.
    if (!Number.isInteger(payload?.roomId) || !Number.isInteger(payload?.userId)) {
      return;
    }
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.markReturned(payload.userId);
    return;
  }

  if (type === SYSTEM_MATCH_STARTED) {
    const payload = message.payload as MatchStartedPayload;
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.setMatchStarted(true);
    // D145b: record the room whose match started so the always-mounted
    // navigator can route a seated player into it even if they are not on
    // RoomPage (currentRoomId === null after leaving it for the prior match).
    // match_started is sent only to room members, so receiving it implies
    // membership — navigating is always correct.
    store.setMatchStartedRoomId(payload.roomId);
    return;
  }

  if (type === SYSTEM_ROOM_OWNER_CHANGED) {
    const payload = message.payload as RoomOwnerChangedPayload;
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.setRoomOwner(payload.newOwnerId);
    return;
  }

  if (type === SYSTEM_ROOM_KICKED) {
    const payload = message.payload as RoomKickedPayload;
    const store = useRoomStore.getState();
    // Require a positive room match. A null currentRoomId means the user is
    // not currently viewing any room — processing the event would set a sticky
    // kickedFromRoomId that traps them on a later re-entry to the same room.
    if (store.currentRoomId !== payload.roomId) return;
    store.setKickedFromRoom(payload.roomId);
    return;
  }

  // Chat events
  if (type === SYSTEM_CHAT_MESSAGE) {
    const payload = message.payload as ChatMessagePayload;
    // Defensive: reject malformed payloads so a server bug or protocol drift
    // cannot inject "Invalid Date" / null username into the chat UI.
    if (
      typeof payload?.userId !== "number" ||
      typeof payload?.username !== "string" ||
      typeof payload?.message !== "string" ||
      typeof payload?.timestamp !== "string" ||
      typeof payload?.scope !== "string"
    ) {
      console.warn("WS: ignoring malformed system:chat_message payload", payload);
      return;
    }
    if (payload.scope === "lobby") {
      useChatStore.getState().appendLobby(payload);
    } else if (payload.scope === "match") {
      // Defence in depth: server only broadcasts match chat to session
      // participants, but if a stale frame arrives after clearGame (or during
      // the race window before a fresh match state lands), drop it so the
      // next match doesn't see leaked history from the previous one.
      if (useMatchStore.getState().roomId === null) return;
      useChatStore.getState().appendMatch(payload);
    } else if (payload.scope === "room") {
      // Defence in depth: drop stale room chat frames that arrive after the
      // user has left the room page, so they don't bleed into the next
      // room's history.
      if (useRoomStore.getState().currentRoomId === null) return;
      useChatStore.getState().appendRoom(payload);
    }
    return;
  }

  // Emote events (Story 8.3) — ephemeral per-seat broadcast, not part of
  // MatchState. Mirrors the chat-message defensive-validation pattern.
  if (type === SYSTEM_EMOTE) {
    const payload = message.payload as EmotePayload;
    if (
      typeof payload?.playerSeat !== "number" ||
      payload.playerSeat < 0 ||
      payload.playerSeat > 3 ||
      typeof payload?.emote !== "string" ||
      !EMOTE_IDS.includes(payload.emote)
    ) {
      console.warn("WS: ignoring malformed system:emote payload", payload);
      return;
    }
    // Defence in depth (Story 8.1 dispatcher hardening): only commit when the
    // user is in an active match. A stray emote frame after clearGame() must
    // not seed the next match's bubble state.
    if (useMatchStore.getState().matchState === null) return;
    useMatchStore.getState().setActiveEmote(payload.playerSeat, payload.emote);
    return;
  }
}

// Game error types that should trigger a user-visible toast
const GAME_ERROR_TYPES: Set<string> = new Set([
  ERROR_WRONG_PHASE,
  ERROR_NOT_YOUR_TURN,
  ERROR_INVALID_ACTION,
  ERROR_ILLEGAL_PLAY,
  ERROR_PAUSE_EXHAUSTED,
  ERROR_NO_ACTIVE_PAUSE,
  ERROR_NOT_ROOM_OWNER,
  ERROR_PLAYER_DISCONNECTED,
]);

function dispatchErrorEvent(message: WsMessage): void {
  const payload = message.payload as { code?: string; message?: string; roomId?: number };

  // Surrender-exhausted has its own dedicated toast (the UI gates already
  // prevent reaching this branch in the happy path; defence in depth).
  // Mirror the GAME_ERROR_TYPES lastError write so debug/store consumers see
  // the event before the early return.
  if (message.type === ERROR_SURRENDER_EXHAUSTED) {
    useMatchStore.getState().setLastError(message.type);
    toast.error(i18n.t("match.surrender.errors.exhausted"));
    return;
  }

  // Story 8.5-1 AC2: auto-start failed server-side. Surface a toast so the
  // four would-be participants know the room reverted to "waiting" rather
  // than navigating them to a non-existent /game/{roomId}. Defence in depth:
  // validate payload shape before showing the toast.
  if (message.type === ERROR_MATCH_START_FAILED) {
    if (typeof payload?.message !== "string" || typeof payload?.roomId !== "number") {
      console.warn("WS: ignoring malformed error:match_start_failed payload", payload);
      return;
    }
    useMatchStore.getState().setLastError(message.type);
    toast.error(i18n.t("room.errors.matchStartFailed"), { duration: 5000 });
    return;
  }

  if (GAME_ERROR_TYPES.has(message.type)) {
    useMatchStore.getState().setLastError(message.type);
  }
  console.warn("WS error:", message.type, payload.message ?? "");
}
