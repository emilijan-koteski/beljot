import i18n from "i18next";
import { useCallback } from "react";
import { toast } from "sonner";

import { handleWsMessage as handleRoomListMessage } from "@/features/lobby/useRoomUpdates";
import { queryClient } from "@/shared/api/queryClient";
import { queryKeys } from "@/shared/api/queryKeys";
import { playSfx } from "@/shared/audio/audioEngine";
import { isCardId } from "@/shared/lib/cardId";
import { honorIsNewPlayer, honorScoreOrPrior } from "@/shared/lib/honor";
import { MOTION } from "@/shared/lib/motion";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useLevelUpStore } from "@/shared/stores/levelUpStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import { rankChangeFromPayload, useRankChangeStore } from "@/shared/stores/rankChangeStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { Room } from "@/shared/types/apiTypes";
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
  FriendRequestPayload,
  HandScoredPayload,
  HonorEjectedPayload,
  HonorUpdatedPayload,
  InsolventEjectedPayload,
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
  RoomClosedInsolventPayload,
  RoomInvitePayload,
  RoomKickedPayload,
  RoomOwnerChangedPayload,
  RoomUpdatedPayload,
  SeasonPointsAwardedPayload,
  SeatUpdatedPayload,
  SurrenderDeclinedPayload,
  SurrenderProposedPayload,
  TrickResolvedPayload,
  TrumpSelectedPayload,
  WhisperPayload,
  WsMessage,
  XpAwardedPayload,
} from "@/shared/types/wsEvents";
import {
  EMOTE_IDS,
  ERROR_AUTH_FAILED,
  ERROR_ILLEGAL_PLAY,
  ERROR_INVALID_ACTION,
  ERROR_MATCH_START_FAILED,
  ERROR_MUST_PICK_TRUMP,
  ERROR_NO_ACTIVE_PAUSE,
  ERROR_NOT_FRIENDS,
  ERROR_NOT_ROOM_OWNER,
  ERROR_NOT_YOUR_TURN,
  ERROR_PAUSE_EXHAUSTED,
  ERROR_PLAYER_DISCONNECTED,
  ERROR_SURRENDER_EXHAUSTED,
  ERROR_WHISPER_BLOCKED_IN_GAME,
  ERROR_WHISPER_RECIPIENT_OFFLINE,
  ERROR_WRONG_PHASE,
  EVENT_AUTO_ACTION,
  EVENT_BELOT_ANNOUNCED,
  EVENT_CARD_PLAYED,
  EVENT_COIN_SETTLEMENT,
  EVENT_DECLARATIONS_RESOLVED,
  EVENT_HAND_SCORED,
  EVENT_HONOR_UPDATED,
  EVENT_MATCH_ABANDONED,
  EVENT_MATCH_END,
  EVENT_MATCH_PAUSED,
  EVENT_MATCH_RESUMED,
  EVENT_MATCH_STATE,
  EVENT_PLAYER_DECLARED,
  EVENT_PLAYER_DISCONNECTED,
  EVENT_PLAYER_RECONNECTED,
  EVENT_SEASON_POINTS_AWARDED,
  EVENT_SURRENDER_DECLINED,
  EVENT_SURRENDER_PROPOSED,
  EVENT_TRICK_RESOLVED,
  EVENT_TRUMP_SELECTED,
  EVENT_XP_AWARDED,
  SEASON_RANK_CHANGES,
  SEASON_SP_REASONS,
  SYSTEM_AUTHENTICATED,
  SYSTEM_BOT_ADDED,
  SYSTEM_BOT_REMOVED,
  SYSTEM_CHAT_MESSAGE,
  SYSTEM_EMOTE,
  SYSTEM_FRIEND_REQUEST,
  SYSTEM_HONOR_EJECTED,
  SYSTEM_INSOLVENT_EJECTED,
  SYSTEM_MATCH_STARTED,
  SYSTEM_PLAYER_JOINED,
  SYSTEM_PLAYER_LEFT,
  SYSTEM_PLAYER_RETURNED,
  SYSTEM_ROOM_CLOSED_INSOLVENT,
  SYSTEM_ROOM_CREATED,
  SYSTEM_ROOM_INVITE,
  SYSTEM_ROOM_KICKED,
  SYSTEM_ROOM_OWNER_CHANGED,
  SYSTEM_ROOM_UPDATED,
  SYSTEM_SEAT_UPDATED,
  SYSTEM_WHISPER,
} from "@/shared/types/wsEvents";

export function useWsDispatch() {
  const dispatch = useCallback((message: WsMessage) => {
    const { type } = message;

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

/** No-op kept so existing test setup that calls it still works; the dispatcher holds no module state. */
export function __resetWsDispatchStateForTests(): void {}

function dispatchGameEvent(message: WsMessage): void {
  const store = useMatchStore.getState();
  const { type } = message;

  if (type === EVENT_MATCH_STATE) {
    const matchState = message.payload as MatchState;
    store.setMatchState(matchState);
    return;
  }

  if (type === EVENT_CARD_PLAYED) {
    const payload = message.payload as CardPlayedPayload;
    const current = store.matchState;
    if (current) {
      if (!payload.cardId || payload.cardId.length < 2) return;
      if (payload.playerSeat !== store.myPlayerSeat || payload.autoPlayed) {
        playSfx("cardPlay");
      }
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
        pendingBelotSeat: isLocalAutoPlay ? null : current.pendingBelotSeat,
      });
    }

    if (payload.autoPlayed) {
      toast.info(i18n.t("match.timer.autoPlayed", { card: payload.cardId }), {
        duration: MOTION.TOAST_INFO,
      });
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
      if (current.currentTrick && current.currentTrick.length > 0) {
        store.setPendingResolvedTrick({
          trick: current.currentTrick,
          winnerSeat: payload.winnerSeat,
        });
      }
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
    store.setCoinSettlement(null);
    store.setHonorSettlement(null);
    store.setSeasonSettlement(null);
    queryClient.removeQueries({ queryKey: queryKeys.matches.lastByRoomAll() });
    return;
  }

  if (type === EVENT_COIN_SETTLEMENT) {
    const payload = message.payload as CoinSettlementPayload;
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

  if (type === EVENT_XP_AWARDED) {
    const payload = message.payload as XpAwardedPayload;
    if (
      !Number.isInteger(payload.xpEarned) ||
      !Number.isInteger(payload.newTotalXp) ||
      !Number.isInteger(payload.newLevel) ||
      typeof payload.leveledUp !== "boolean"
    ) {
      return;
    }
    const auth = useAuthStore.getState();
    if (auth.user) {
      auth.setUser({ ...auth.user, totalXp: payload.newTotalXp, level: payload.newLevel });
    }
    if (payload.leveledUp) {
      useLevelUpStore.getState().setPending({
        newLevel: payload.newLevel,
        newTotalXp: payload.newTotalXp,
        xpEarned: payload.xpEarned,
      });
    }
    return;
  }

  if (type === EVENT_HONOR_UPDATED) {
    const payload = message.payload as HonorUpdatedPayload;
    if (
      !Number.isInteger(payload.honorScore) ||
      typeof payload.honorTier !== "string" ||
      payload.honorTier === "" ||
      !Number.isInteger(payload.honorCompletedTotal) ||
      !Number.isInteger(payload.honorAbandonedTotal) ||
      typeof payload.isNewPlayer !== "boolean"
    ) {
      console.warn("WS: ignoring malformed event:honor_updated payload", payload);
      return;
    }
    const authState = useAuthStore.getState();
    if (authState.user) {
      if (!honorIsNewPlayer(authState.user.isNewPlayer) && !payload.isNewPlayer) {
        useMatchStore.getState().setHonorSettlement({
          before: honorScoreOrPrior(authState.user.honorScore),
          after: payload.honorScore,
          tier: payload.honorTier,
        });
      }
      authState.setUser({
        ...authState.user,
        honorScore: payload.honorScore,
        honorTier: payload.honorTier,
        isNewPlayer: payload.isNewPlayer,
      });
    }
    return;
  }

  if (type === EVENT_SEASON_POINTS_AWARDED) {
    const payload = message.payload as SeasonPointsAwardedPayload;
    if (
      !payload ||
      typeof payload !== "object" ||
      !Number.isInteger(payload.spChange) ||
      !Number.isInteger(payload.newSeasonSp) ||
      typeof payload.rankTier !== "string" ||
      payload.rankTier === "" ||
      !(payload.rankDivision === null || Number.isInteger(payload.rankDivision)) ||
      !(SEASON_RANK_CHANGES as readonly string[]).includes(payload.rankChange) ||
      !(SEASON_SP_REASONS as readonly string[]).includes(payload.reason) ||
      typeof payload.seasonName !== "string"
    ) {
      console.warn("WS: ignoring malformed event:season_points_awarded payload", payload);
      return;
    }
    void queryClient.invalidateQueries({ queryKey: queryKeys.season.current() });
    useMatchStore.getState().setSeasonSettlement({
      spChange: payload.spChange,
      newSeasonSp: payload.newSeasonSp,
      rankTier: payload.rankTier,
      rankDivision: payload.rankDivision,
      reason: payload.reason,
    });
    const rankChange = rankChangeFromPayload(payload);
    if (rankChange) {
      useRankChangeStore.getState().setPending(rankChange);
    }
    return;
  }

  if (type === EVENT_TRUMP_SELECTED) {
    const payload = message.payload as TrumpSelectedPayload;
    if (
      typeof payload.cardId !== "string" ||
      (payload.cardId !== "" && !isCardId(payload.cardId))
    ) {
      console.warn("WS: ignoring malformed event:trump_selected payload", payload);
      return;
    }
    store.setTrumpReveal(payload);
    return;
  }

  if (type === EVENT_DECLARATIONS_RESOLVED) {
    const payload = message.payload as DeclarationsResolvedPayload;
    store.setDeclarationReveal(payload);
    return;
  }

  if (type === EVENT_PLAYER_DECLARED) {
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
    playSfx("popup");
    return;
  }

  if (type === EVENT_BELOT_ANNOUNCED) {
    const payload = message.payload as BelotAnnouncedPayload;
    if (!payload.cardId || payload.cardId.length < 2) return;
    store.setBelotReveal(payload);
    return;
  }

  if (type === EVENT_MATCH_PAUSED) {
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
        payload?.type !== "skip_belot" &&
        payload?.type !== "pick_trump")
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
          : payload.type === "skip_belot"
            ? "match.timer.autoSkippedBelot"
            : "match.timer.autoPickedTrump";
    toast.info(i18n.t(i18nKey, { player: playerName }), { duration: 3000 });
    return;
  }

  if (type === EVENT_MATCH_RESUMED) {
    const payload = message.payload as MatchResumedPayload;
    if (payload.ownerOverride) {
      toast.info(i18n.t("match.pause.ownerResumedToast"), { duration: 3000 });
    }
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
    return;
  }

  if (type === EVENT_MATCH_ABANDONED) {
    const payload = message.payload as MatchAbandonedPayload;
    store.setMatchAbandonedData(payload);
    store.setCoinSettlement(null);
    store.setHonorSettlement(null);
    store.setSeasonSettlement(null);
    return;
  }

  if (type === EVENT_SURRENDER_PROPOSED) {
    if (store.matchState === null) return;
    const payload = message.payload as SurrenderProposedPayload;
    store.setSurrenderProposed(payload);
    if (
      payload.proposerSeat !== store.myPlayerSeat &&
      store.matchEndData === null &&
      store.matchAbandonedData === null
    ) {
      playSfx("popup");
    }
    return;
  }

  if (type === EVENT_SURRENDER_DECLINED) {
    if (store.matchState === null) return;
    const payload = message.payload as SurrenderDeclinedPayload;
    store.setSurrenderDeclined(payload);
    toast.info(i18n.t("match.surrender.declinedToast"), { duration: 3000 });
    return;
  }
}

function dispatchSystemEvent(message: WsMessage): void {
  const { type } = message;

  if (type === SYSTEM_FRIEND_REQUEST) {
    const payload = message.payload as FriendRequestPayload;
    if (typeof payload?.requestId !== "number" || typeof payload?.fromUserId !== "number") {
      return;
    }
    void queryClient.invalidateQueries({ queryKey: queryKeys.friends.requests() });
    void queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(payload.fromUserId) });
    const fromUsername =
      typeof payload.fromUsername === "string" && payload.fromUsername.length > 0
        ? payload.fromUsername
        : i18n.t("friends.someone");
    toast.info(i18n.t("friends.requestReceivedToast", { username: fromUsername }), {
      duration: MOTION.TOAST_INFO,
    });
    return;
  }

  if (type === SYSTEM_ROOM_INVITE) {
    const payload = message.payload as RoomInvitePayload;
    if (
      typeof payload?.inviteId !== "number" ||
      typeof payload?.roomId !== "number" ||
      typeof payload?.roomName !== "string" ||
      typeof payload?.inviterUserId !== "number" ||
      typeof payload?.coinBuyIn !== "number" ||
      typeof payload?.isPrivate !== "boolean" ||
      typeof payload?.isHostInvite !== "boolean" ||
      typeof payload?.minHonor !== "number" ||
      typeof payload?.expiresAt !== "string" ||
      payload.expiresAt === "" ||
      Number.isNaN(new Date(payload.expiresAt).getTime())
    ) {
      console.warn("WS: ignoring malformed system:room_invite payload", payload);
      return;
    }
    useRoomStore.getState().setRoomInvite({
      inviteId: payload.inviteId,
      roomId: payload.roomId,
      roomName: payload.roomName,
      inviterUserId: payload.inviterUserId,
      inviterUsername: typeof payload.inviterUsername === "string" ? payload.inviterUsername : "",
      coinBuyIn: payload.coinBuyIn,
      isPrivate: payload.isPrivate,
      isHostInvite: payload.isHostInvite,
      minHonor: payload.minHonor,
      expiresAt: payload.expiresAt,
    });
    return;
  }

  if (type === SYSTEM_ROOM_CREATED || type === SYSTEM_ROOM_UPDATED) {
    handleRoomListMessage(new MessageEvent("message", { data: JSON.stringify(message) }));
    if (type === SYSTEM_ROOM_UPDATED) {
      const payload = message.payload as RoomUpdatedPayload;
      const store = useRoomStore.getState();
      if (store.room && store.currentRoomId === payload.id) {
        store.setRoom({ ...store.room, ...(payload as Partial<Room>) });
      }
    }
    return;
  }

  if (
    type === SYSTEM_PLAYER_JOINED ||
    type === SYSTEM_PLAYER_LEFT ||
    type === SYSTEM_SEAT_UPDATED
  ) {
    handleRoomListMessage(new MessageEvent("message", { data: JSON.stringify(message) }));
  }

  if (type === SYSTEM_PLAYER_JOINED) {
    const payload = message.payload as PlayerJoinedPayload;
    const store = useRoomStore.getState();
    if (store.currentRoomId !== null && store.currentRoomId !== payload.roomId) return;
    store.addPlayer(
      {
        id: payload.userId,
        roomId: payload.roomId,
        userId: payload.userId,
        username: payload.username,
        avatarUrl: typeof payload.avatarUrl === "string" ? payload.avatarUrl : null,
        seat: null,
        team: null,
        isBot: false,
        honorScore: typeof payload.honorScore === "number" ? payload.honorScore : undefined,
        honorTier: typeof payload.honorTier === "string" ? payload.honorTier : undefined,
        level: typeof payload.level === "number" ? payload.level : undefined,
        createdAt: new Date().toISOString(),
      },
      payload.playerCount,
    );
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
    store.updatePlayerSeat(
      payload.userId,
      payload.seat,
      payload.team,
      payload.previousSeat,
      payload.avatarUrl === undefined
        ? undefined
        : typeof payload.avatarUrl === "string"
          ? payload.avatarUrl
          : null,
    );
    return;
  }

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
    if (store.currentRoomId !== payload.roomId) return;
    store.setKickedFromRoom(payload.roomId);
    return;
  }

  if (type === SYSTEM_INSOLVENT_EJECTED) {
    const payload = message.payload as InsolventEjectedPayload;
    if (
      typeof payload?.roomId !== "number" ||
      typeof payload?.buyIn !== "number" ||
      typeof payload?.balance !== "number"
    ) {
      console.warn("WS: ignoring malformed system:insolvent_ejected payload", payload);
      return;
    }
    useRoomStore.getState().setRoomEjection({
      roomId: payload.roomId,
      buyIn: payload.buyIn,
      balance: payload.balance,
      reason: "insolvent",
    });
    return;
  }

  if (type === SYSTEM_HONOR_EJECTED) {
    const payload = message.payload as HonorEjectedPayload;
    if (
      typeof payload?.roomId !== "number" ||
      typeof payload?.minHonor !== "number" ||
      typeof payload?.honor !== "number"
    ) {
      console.warn("WS: ignoring malformed system:honor_ejected payload", payload);
      return;
    }
    useRoomStore.getState().setRoomEjection({
      roomId: payload.roomId,
      buyIn: 0,
      balance: 0,
      minHonor: payload.minHonor,
      honor: payload.honor,
      reason: "honor",
    });
    return;
  }

  if (type === SYSTEM_ROOM_CLOSED_INSOLVENT) {
    const payload = message.payload as RoomClosedInsolventPayload;
    if (typeof payload?.roomId !== "number") {
      console.warn("WS: ignoring malformed system:room_closed_insolvent payload", payload);
      return;
    }
    useRoomStore.getState().setRoomEjection({
      roomId: payload.roomId,
      buyIn: 0,
      balance: 0,
      reason: "roomClosed",
    });
    return;
  }

  if (type === SYSTEM_CHAT_MESSAGE) {
    const payload = message.payload as ChatMessagePayload;
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
      if (useMatchStore.getState().roomId === null) return;
      useChatStore.getState().appendMatch(payload);
    } else if (payload.scope === "room") {
      if (useRoomStore.getState().currentRoomId === null) return;
      useChatStore.getState().appendRoom(payload);
    }
    return;
  }

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
    const matchStore = useMatchStore.getState();
    if (matchStore.matchState === null) return;
    matchStore.setActiveEmote(payload.playerSeat, payload.emote);
    const phase = matchStore.matchState.phase;
    if (
      matchStore.matchEndData === null &&
      matchStore.matchAbandonedData === null &&
      phase !== "paused" &&
      phase !== "disconnected"
    ) {
      playSfx("popup");
    }
    return;
  }

  if (type === SYSTEM_WHISPER) {
    const payload = message.payload as WhisperPayload;
    if (
      typeof payload?.fromUserId !== "number" ||
      typeof payload?.fromUsername !== "string" ||
      typeof payload?.toUserId !== "number" ||
      typeof payload?.toUsername !== "string" ||
      typeof payload?.message !== "string" ||
      typeof payload?.timestamp !== "string"
    ) {
      console.warn("WS: ignoring malformed system:whisper payload", payload);
      return;
    }
    const myId = useAuthStore.getState().user?.id;
    if (typeof myId !== "number") return;
    useChatStore.getState().appendWhisper(payload, myId);
    return;
  }
}

const GAME_ERROR_TYPES: Set<string> = new Set([
  ERROR_WRONG_PHASE,
  ERROR_NOT_YOUR_TURN,
  ERROR_INVALID_ACTION,
  ERROR_ILLEGAL_PLAY,
  ERROR_PAUSE_EXHAUSTED,
  ERROR_NO_ACTIVE_PAUSE,
  ERROR_NOT_ROOM_OWNER,
  ERROR_PLAYER_DISCONNECTED,
  ERROR_MUST_PICK_TRUMP,
]);

function dispatchErrorEvent(message: WsMessage): void {
  const payload = message.payload as { code?: string; message?: string; roomId?: number };

  if (message.type === ERROR_SURRENDER_EXHAUSTED) {
    useMatchStore.getState().setLastError(message.type);
    toast.error(i18n.t("match.surrender.errors.exhausted"));
    return;
  }

  if (message.type === ERROR_MATCH_START_FAILED) {
    if (typeof payload?.message !== "string" || typeof payload?.roomId !== "number") {
      console.warn("WS: ignoring malformed error:match_start_failed payload", payload);
      return;
    }
    useMatchStore.getState().setLastError(message.type);
    toast.error(i18n.t("room.errors.matchStartFailed"), { duration: 5000 });
    return;
  }

  if (message.type === ERROR_NOT_FRIENDS) {
    toast.error(i18n.t("whisper.errors.notFriends"));
    return;
  }
  if (message.type === ERROR_WHISPER_BLOCKED_IN_GAME) {
    toast.error(i18n.t("whisper.errors.blockedInGame"));
    return;
  }
  if (message.type === ERROR_WHISPER_RECIPIENT_OFFLINE) {
    toast.error(i18n.t("whisper.errors.recipientOffline"));
    return;
  }

  if (GAME_ERROR_TYPES.has(message.type)) {
    useMatchStore.getState().setLastError(message.type);
  }
  console.warn("WS error:", message.type, payload.message ?? "");
}
