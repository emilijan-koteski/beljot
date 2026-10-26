import {
  Flag,
  HelpCircle,
  Menu as MenuIcon,
  Pause,
  Settings as SettingsIcon,
  Volume2,
  VolumeX,
  X,
} from "lucide-react";
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  type BlockerFunction,
  NavigationType,
  useBlocker,
  useLocation,
  useNavigate,
  useParams,
} from "react-router";

import { FetchError } from "@/shared/api/axiosClient";
import { getRoom, leaveRoom, returnToRoom } from "@/shared/api/rooms";
import { playSfx } from "@/shared/audio/audioEngine";
import { useMatchAudio } from "@/shared/audio/useMatchAudio";
import { payloadSoundKey } from "@/shared/audio/useSfxOnce";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { persistAudioPreferences, resolveAudioEnabled } from "@/shared/lib/audioPreference";
import { playerDisplayName } from "@/shared/lib/botName";
import { honorScoreOrPrior } from "@/shared/lib/honor";
import { FLAG_LIFETIME, MOTION } from "@/shared/lib/motion";
import { variantLabel } from "@/shared/lib/roomLabels";
import { Z } from "@/shared/lib/zLayers";
import { useWsConnectionState, useWsSendMessage } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import { matchTargetFor, type Suit, type TeamString } from "@/shared/types/matchTypes";
import {
  ACTION_ANNOUNCE_BELOT,
  ACTION_CONTINUE,
  ACTION_DECLARE,
  ACTION_DECLINE_BELOT,
  ACTION_EMOTE,
  ACTION_OWNER_UNPAUSE,
  ACTION_PASS_TRUMP,
  ACTION_PAUSE,
  ACTION_PICK_TRUMP,
  ACTION_PLAY_CARD,
  ACTION_SKIP_DECLARE,
  ACTION_SURRENDER_ACCEPT,
  ACTION_SURRENDER_DECLINE,
  ACTION_SURRENDER_REQUEST,
  ACTION_UNPAUSE,
  type EmoteID,
  type HandScoredPayload,
} from "@/shared/types/wsEvents";

import { BelotPrompt } from "./components/BelotPrompt";
import { BelotReveal } from "./components/BelotReveal";
import { CapotAnimation } from "./components/CapotAnimation";
import { CardFlight, type CardFlightDescriptor, type FlightRect } from "./components/CardFlight";
import { DealAnimation } from "./components/DealAnimation";
import { DeclarationPrompt } from "./components/DeclarationPrompt";
import { DeclarationReveal } from "./components/DeclarationReveal";
import { DeclareBanner } from "./components/DeclareBanner";
import { EmoteBubble } from "./components/EmoteBubble";
import { EmotePickerButton } from "./components/EmotePickerButton";
import { HandCards } from "./components/HandCards";
import { HUDButton } from "./components/HUDButton";
import { MatchChatDock } from "./components/MatchChatDock";
import { MatchResult } from "./components/MatchResult";
import { PauseOverlay } from "./components/PauseOverlay";
import { PlayerSeat, type SeatOrientation } from "./components/PlayerSeat";
import { ReconnectOverlay } from "./components/ReconnectOverlay";
import { RulesDialog } from "./components/RulesDialog";
import { ScorePanel } from "./components/ScorePanel";
import { ScoreReveal } from "./components/ScoreReveal";
import { SettingsDialog } from "./components/SettingsDialog";
import { StakePill } from "./components/StakePill";
import { SurrenderButton, SurrenderConfirmDialog } from "./components/SurrenderButton";
import { SurrenderOpponentBanner } from "./components/SurrenderOpponentBanner";
import { SurrenderPrompt } from "./components/SurrenderPrompt";
import { TableAmbience } from "./components/TableAmbience";
import { TableBackdrop } from "./components/TableBackdrop";
import { TRICK_SLOT_H, TRICK_SLOT_W, TrickArea } from "./components/TrickArea";
import { TrumpIndicator } from "./components/TrumpIndicator";
import { TrumpPrompt } from "./components/TrumpPrompt";
import { TrumpReveal } from "./components/TrumpReveal";
import { UrgentTick } from "./components/UrgentTick";
import { Wordmark } from "./components/Wordmark";
import { detectDeclarations } from "./lib/declarations";
import { isBelotEligible, legalCardIds } from "./lib/legalCards";
import { seatTeam } from "./lib/tableTheme";
import { compassOffset, SLOT_POSITIONS } from "./lib/trickLayout";
import { useDealController } from "./lib/useDealController";
import { rulesForVariant } from "./lib/variantRules";

const TAILWIND_MD_PX = 768;

function rectFrom(el: Element | null): FlightRect | null {
  if (!el) return null;
  const r = el.getBoundingClientRect();
  if (r.width === 0 && r.height === 0) return null;
  return { left: r.left, top: r.top, width: r.width, height: r.height };
}

/** Measures a trick slot's rect from its DOM anchor, falling back to trick-area
 *  offsets when the anchor is not laid out yet. */
function measureTrickSlotRect(compass: 0 | 1 | 2 | 3): FlightRect | null {
  const slotEl = document.querySelector(`[data-testid="trick-slot-${compass}"]`);
  const slotRect = rectFrom(slotEl);
  if (slotRect) return slotRect;

  const ta = document.querySelector('[data-testid="trick-area"]');
  const r = rectFrom(ta);
  if (!r) return null;
  const slot = SLOT_POSITIONS[compass];
  const cx = r.left + r.width / 2 + slot.offsetX;
  const cy = r.top + r.height / 2 + slot.offsetY;
  return {
    left: cx - TRICK_SLOT_W / 2,
    top: cy - TRICK_SLOT_H / 2,
    width: TRICK_SLOT_W,
    height: TRICK_SLOT_H,
  };
}

/** Mid-flight waypoint for the self throw: a slot-sized rect at viewport bottom-center. */
function viewportBottomCenterRect(): FlightRect {
  const w = TRICK_SLOT_W;
  const h = TRICK_SLOT_H;
  return {
    left: window.innerWidth / 2 - w / 2,
    top: window.innerHeight - h / 2,
    width: w,
    height: h,
  };
}

/**
 * Final rect for a trick-collect flight, anchored to the winner's compass position in viewport
 * coords. Seats are CSS-anchored to the viewport edges, so this skips the flaky DOM read.
 */
function winnerCollectRect(winner: number, myPlayerSeat: number): FlightRect {
  const compass = compassOffset(winner, myPlayerSeat);
  const vw = window.innerWidth;
  const vh = window.innerHeight;

  const compact = vw < TAILWIND_MD_PX;
  const sideInset = compact ? 12 : 64;
  const topInset = compact ? 24 : 64;
  const inboard = compact ? 40 : 56;

  let cx: number;
  let cy: number;
  switch (compass) {
    case 0:
      cx = vw / 2;
      cy = vh - TRICK_SLOT_H / 2 - 24;
      break;
    case 1:
      cx = vw - sideInset - inboard;
      cy = vh / 2;
      break;
    case 2:
      cx = vw / 2;
      cy = topInset + inboard;
      break;
    case 3:
      cx = sideInset + inboard;
      cy = vh / 2;
      break;
  }

  return {
    left: cx - TRICK_SLOT_W / 2,
    top: cy - TRICK_SLOT_H / 2,
    width: TRICK_SLOT_W,
    height: TRICK_SLOT_H,
  };
}

const COLLECT_FLIGHT = "collect-";
function collectFlightPrefix(receivedAt: number): string {
  return `${COLLECT_FLIGHT}${receivedAt}-`;
}

const SEAT_POSITIONS: Record<number, string> = {
  0: "bottom-48 left-1/2 -translate-x-1/2 md:bottom-44",
  1: "right-3 top-1/2 -translate-y-1/2 md:right-16",
  2: "top-6 left-1/2 -translate-x-1/2 md:top-16",
  3: "left-3 top-1/2 -translate-y-1/2 md:left-16",
};

const SEAT_ORIENTATIONS: Record<number, SeatOrientation> = {
  0: "bottom",
  1: "right",
  2: "top",
  3: "left",
};

export function MatchPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const returnToLobby = useLobbyReturn();
  const { roomId: roomIdParam } = useParams<{ roomId: string }>();
  const parsedRoomId = roomIdParam ? Number(roomIdParam) : null;
  const roomIdNum =
    parsedRoomId !== null && Number.isFinite(parsedRoomId) && parsedRoomId > 0
      ? parsedRoomId
      : null;

  const sendMessage = useWsSendMessage();

  const user = useAuthStore((s) => s.user);
  const setRoomEjection = useRoomStore((s) => s.setRoomEjection);
  const matchState = useMatchStore((s) => s.matchState);
  const myPlayerSeat = useMatchStore((s) => s.myPlayerSeat);
  const setMyPlayerSeat = useMatchStore((s) => s.setMyPlayerSeat);
  const clearGame = useMatchStore((s) => s.clearGame);
  const lastError = useMatchStore((s) => s.lastError);
  const setLastError = useMatchStore((s) => s.setLastError);
  const declarationReveal = useMatchStore((s) => s.declarationReveal);
  const setDeclarationReveal = useMatchStore((s) => s.setDeclarationReveal);
  const belotReveal = useMatchStore((s) => s.belotReveal);
  const setBelotReveal = useMatchStore((s) => s.setBelotReveal);
  const trumpReveal = useMatchStore((s) => s.trumpReveal);
  const setTrumpReveal = useMatchStore((s) => s.setTrumpReveal);
  const scoreRevealData = useMatchStore((s) => s.scoreRevealData);
  const setScoreRevealData = useMatchStore((s) => s.setScoreRevealData);
  const matchEndData = useMatchStore((s) => s.matchEndData);
  const setMatchEndData = useMatchStore((s) => s.setMatchEndData);
  const coinSettlement = useMatchStore((s) => s.coinSettlement);
  const honorSettlement = useMatchStore((s) => s.honorSettlement);
  const seasonSettlement = useMatchStore((s) => s.seasonSettlement);
  const matchAbandonedData = useMatchStore((s) => s.matchAbandonedData);
  const setMatchAbandonedData = useMatchStore((s) => s.setMatchAbandonedData);
  const activeEmotes = useMatchStore((s) => s.activeEmotes);
  const setActiveEmote = useMatchStore((s) => s.setActiveEmote);
  const activeDeclares = useMatchStore((s) => s.activeDeclares);
  const setActiveDeclare = useMatchStore((s) => s.setActiveDeclare);
  const pendingAutoPlayedCard = useMatchStore((s) => s.pendingAutoPlayedCard);
  const setPendingAutoPlayedCard = useMatchStore((s) => s.setPendingAutoPlayedCard);
  const pendingResolvedTrick = useMatchStore((s) => s.pendingResolvedTrick);
  const clearPendingResolvedTrick = useMatchStore((s) => s.clearPendingResolvedTrick);

  useMatchAudio();
  const audioOn =
    resolveAudioEnabled(user?.soundEnabled) || resolveAudioEnabled(user?.musicEnabled);
  const handleToggleMute = useCallback(() => {
    const next = !audioOn;
    void persistAudioPreferences({ soundEnabled: next, musicEnabled: next });
  }, [audioOn]);
  const muteLabel = audioOn ? t("match.hud.mute") : t("match.hud.unmute");
  const MuteIcon = audioOn ? Volume2 : VolumeX;

  const location = useLocation();
  const [cameFromRoom] = useState(
    () => (location.state as { fromRoom?: boolean } | null)?.fromRoom === true,
  );
  useEffect(() => {
    if (!cameFromRoom) return;
    navigate(
      { pathname: location.pathname, search: location.search, hash: location.hash },
      { replace: true, state: null },
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the arrival flag is consumed exactly once, at mount
  }, []);
  const prefersReducedMotion = useReducedMotion();
  const isCompactTable = useMediaQuery("(max-width: 767px)");
  const [splashElapsed, setSplashElapsed] = useState(!cameFromRoom);
  useEffect(() => {
    if (!cameFromRoom) return;
    const duration = prefersReducedMotion
      ? MOTION.GAME_STARTING_SPLASH_REDUCED
      : MOTION.GAME_STARTING_SPLASH;
    const timer = window.setTimeout(() => setSplashElapsed(true), duration);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- duration is locked at mount; a motion-preference flip must not restart it
  }, [cameFromRoom]);

  const [errorToast, setErrorToast] = useState<string | null>(null);
  const errorToastTimerRef = useRef<number | null>(null);
  const roomBuyInRef = useRef<number | null>(null);
  const roomMinHonorRef = useRef<number | null>(null);
  const [roomBuyIn, setRoomBuyIn] = useState<number | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [rulesOpen, setRulesOpen] = useState(false);
  const [hudMenuOpen, setHudMenuOpen] = useState(false);
  const [surrenderConfirmOpen, setSurrenderConfirmOpen] = useState(false);
  const [flyingCardId, setFlyingCardId] = useState<string | null>(null);
  const flyingClearTimerRef = useRef<number | null>(null);

  const [answeredDeadline, setAnsweredDeadline] = useState<string | null>(null);
  const [answeredWindowHand, setAnsweredWindowHand] = useState<number | null>(null);
  const markTurnAnswered = useCallback(() => {
    setAnsweredDeadline(useMatchStore.getState().matchState?.turnExpiresAt ?? null);
  }, []);
  const markWindowAnswered = useCallback(() => {
    const live = useMatchStore.getState().matchState;
    if (live?.phase === "declaring") setAnsweredWindowHand(live.handNumber);
  }, []);

  const [belotPromptCardId, setBelotPromptCardId] = useState<string | null>(null);
  const [pendingLocalBelotChoice, setPendingLocalBelotChoice] = useState<{
    cardId: string;
    announce: boolean;
  } | null>(null);
  const pendingBelotSeat = useMatchStore((s) => s.matchState?.pendingBelotSeat ?? null);
  useEffect(() => {
    if (pendingLocalBelotChoice === null) return;
    if (pendingBelotSeat !== myPlayerSeat) return;
    sendMessage(
      pendingLocalBelotChoice.announce ? ACTION_ANNOUNCE_BELOT : ACTION_DECLINE_BELOT,
      {},
    );
    setPendingLocalBelotChoice(null);
  }, [pendingBelotSeat, pendingLocalBelotChoice, myPlayerSeat, sendMessage]);

  const belotTurnOpen = useMatchStore((s) => {
    const live = s.matchState;
    return live !== null && live.phase === "playing" && live.activePlayerSeat === myPlayerSeat;
  });
  const myHandIds = useMatchStore(
    (s) =>
      s.matchState?.players
        .find((p) => p.seat === myPlayerSeat)
        ?.hand.map((c) => `${c.rank}${c.suit}`)
        .join(",") ?? "",
  );
  const holdsCard = (cardId: string | null) =>
    cardId !== null && myHandIds.split(",").includes(cardId);
  const promptCardHeld = holdsCard(belotPromptCardId);
  const choiceCardHeld = holdsCard(pendingLocalBelotChoice?.cardId ?? null);
  useEffect(() => {
    if (belotPromptCardId !== null && (!belotTurnOpen || !promptCardHeld)) {
      setBelotPromptCardId(null);
    }
  }, [belotPromptCardId, belotTurnOpen, promptCardHeld]);
  useEffect(() => {
    if (pendingLocalBelotChoice === null || pendingBelotSeat === myPlayerSeat) return;
    if (!belotTurnOpen || !choiceCardHeld) setPendingLocalBelotChoice(null);
  }, [pendingLocalBelotChoice, pendingBelotSeat, myPlayerSeat, belotTurnOpen, choiceCardHeld]);

  const [activeFlights, setActiveFlights] = useState<CardFlightDescriptor[]>([]);
  const flightingCardIds = useMemo(
    () =>
      new Set(
        activeFlights.flatMap((f) => (f.card === null ? [] : [`${f.card.rank}${f.card.suit}`])),
      ),
    [activeFlights],
  );
  const [collectingReceivedAt, setCollectingReceivedAt] = useState<number | null>(null);
  const isCollecting =
    pendingResolvedTrick !== null && collectingReceivedAt === pendingResolvedTrick.receivedAt;
  const prevTrickLenRef = useRef(0);

  useLayoutEffect(() => {
    if (!pendingAutoPlayedCard) return;
    setFlyingCardId(pendingAutoPlayedCard.cardId);
    if (flyingClearTimerRef.current !== null) {
      clearTimeout(flyingClearTimerRef.current);
    }
    flyingClearTimerRef.current = window.setTimeout(() => {
      setFlyingCardId(null);
      flyingClearTimerRef.current = null;
    }, FLAG_LIFETIME.FLYING_CARD);
    if (myPlayerSeat !== null && !prefersReducedMotion) {
      const cardId = pendingAutoPlayedCard.cardId;
      const receivedAt = pendingAutoPlayedCard.receivedAt;
      const myPlayer = useMatchStore
        .getState()
        .matchState?.players.find((p) => p.seat === myPlayerSeat);
      const card = myPlayer?.hand.find((c) => `${c.rank}${c.suit}` === cardId);
      const sourceRect = rectFrom(document.querySelector(`[data-testid="hand-card-${cardId}"]`));
      const destRect = measureTrickSlotRect(0);
      if (card && sourceRect && destRect) {
        setActiveFlights((prev) => [
          ...prev,
          {
            id: `throw-self-${cardId}-${receivedAt}`,
            card,
            fromRect: sourceRect,
            toRect: destRect,
            waypointRect: viewportBottomCenterRect(),
            durationMs: MOTION.CARD_FLIGHT_SELF_THROW,
          },
        ]);
      }
    }
    setPendingAutoPlayedCard(null);
  }, [pendingAutoPlayedCard, setPendingAutoPlayedCard, myPlayerSeat, prefersReducedMotion]);

  const currentTrick = matchState?.currentTrick;
  useLayoutEffect(() => {
    const trick = currentTrick ?? [];
    const prev = prevTrickLenRef.current;
    prevTrickLenRef.current = trick.length;
    if (prefersReducedMotion) return;
    if (myPlayerSeat === null) return;
    if (trick.length > prev && trick.length > 0) {
      const newest = trick[trick.length - 1];
      if (newest && newest.playerSeat !== myPlayerSeat) {
        const compass = compassOffset(newest.playerSeat, myPlayerSeat);
        const card = newest.card;
        const playerSeat = newest.playerSeat;
        const sourceRect = rectFrom(document.querySelector(`[data-seat-deck="${playerSeat}"]`));
        const destRect = measureTrickSlotRect(compass);
        if (sourceRect && destRect) {
          const cardId = `${card.rank}${card.suit}`;
          setActiveFlights((prevFlights) => [
            ...prevFlights,
            {
              id: `throw-opp-${playerSeat}-${cardId}-${Date.now()}`,
              card,
              fromRect: sourceRect,
              toRect: destRect,
              durationMs: MOTION.CARD_FLIGHT_OPPONENT_THROW,
            },
          ]);
        }
      }
    }
  }, [currentTrick, myPlayerSeat, prefersReducedMotion]);

  useEffect(() => {
    if (!pendingResolvedTrick) return;
    if (myPlayerSeat === null) return;

    const collectSoundKey = String(pendingResolvedTrick.receivedAt);

    if (prefersReducedMotion) {
      const reducedTimer = window.setTimeout(() => {
        playSfx("trickCollect", { dedupeKey: collectSoundKey });
        clearPendingResolvedTrick(pendingResolvedTrick.receivedAt);
      }, MOTION.TRICK_RESOLVE_PAUSE);
      return () => clearTimeout(reducedTimer);
    }

    const FALLBACK_CLEAR_MS = MOTION.TRICK_RESOLVE_PAUSE + MOTION.CARD_FLIGHT_COLLECT + 400;
    const fallbackClearTimer = window.setTimeout(() => {
      clearPendingResolvedTrick(pendingResolvedTrick.receivedAt);
    }, FALLBACK_CLEAR_MS);

    const winner = pendingResolvedTrick.winnerSeat;
    const snapshot = pendingResolvedTrick.trick;
    const receivedAt = pendingResolvedTrick.receivedAt;

    const glowTimer = window.setTimeout(() => {
      playSfx("trickCollect", { dedupeKey: collectSoundKey });
      const destRect = winnerCollectRect(winner, myPlayerSeat);

      const newFlights: CardFlightDescriptor[] = [];
      for (const tc of snapshot) {
        const compass = compassOffset(tc.playerSeat, myPlayerSeat);
        const fromRect = measureTrickSlotRect(compass);
        if (!fromRect) continue;
        const cardId = `${tc.card.rank}${tc.card.suit}`;
        newFlights.push({
          id: `${collectFlightPrefix(receivedAt)}${cardId}`,
          card: tc.card,
          fromRect,
          toRect: destRect,
          durationMs: MOTION.CARD_FLIGHT_COLLECT,
          endOpacity: 0,
        });
      }

      if (newFlights.length === 0) {
        clearPendingResolvedTrick(receivedAt);
        return;
      }
      setCollectingReceivedAt(receivedAt);
      setActiveFlights((prev) => [...prev, ...newFlights]);
    }, MOTION.TRICK_RESOLVE_PAUSE);

    return () => {
      clearTimeout(glowTimer);
      clearTimeout(fallbackClearTimer);
    };
  }, [pendingResolvedTrick, myPlayerSeat, prefersReducedMotion, clearPendingResolvedTrick]);

  const liveResolvedAt = pendingResolvedTrick?.receivedAt ?? null;
  useLayoutEffect(() => {
    const livePrefix = liveResolvedAt === null ? null : collectFlightPrefix(liveResolvedAt);
    setActiveFlights((prev) => {
      const next = prev.filter(
        (f) =>
          !f.id.startsWith(COLLECT_FLIGHT) || (livePrefix !== null && f.id.startsWith(livePrefix)),
      );
      return next.length === prev.length ? prev : next;
    });
  }, [liveResolvedAt]);

  const handleFlightComplete = useCallback((flightId: string) => {
    setActiveFlights((prev) => prev.filter((f) => f.id !== flightId));
  }, []);

  useLayoutEffect(() => {
    if (!isCollecting || pendingResolvedTrick === null) return;
    const prefix = collectFlightPrefix(pendingResolvedTrick.receivedAt);
    if (!activeFlights.some((f) => f.id.startsWith(prefix))) {
      clearPendingResolvedTrick(pendingResolvedTrick.receivedAt);
    }
  }, [activeFlights, isCollecting, pendingResolvedTrick, clearPendingResolvedTrick]);

  const suppressedCardIds = useMemo(() => {
    if (!isCollecting || pendingResolvedTrick === null) return flightingCardIds;
    const ids = new Set(flightingCardIds);
    for (const tc of pendingResolvedTrick.trick) {
      ids.add(`${tc.card.rank}${tc.card.suit}`);
    }
    return ids;
  }, [isCollecting, pendingResolvedTrick, flightingCardIds]);

  const dealView = useDealController({
    matchState,
    myPlayerSeat,
    tableVisible: splashElapsed && matchState !== null && myPlayerSeat !== null,
    animateOpening: cameFromRoom,
    prefersReducedMotion,
    compact: isCompactTable,
    updateFlights: setActiveFlights,
  });
  const dealing = dealView.dealing;

  const lastTrumpRef = useRef<{
    suit: Suit | null;
    callerSeat: number | null;
    handNumber: number | null;
  }>({
    suit: null,
    callerSeat: null,
    handNumber: null,
  });
  if (matchState?.trumpSuit) {
    lastTrumpRef.current = {
      suit: matchState.trumpSuit,
      callerSeat: matchState.trumpCallerSeat,
      handNumber: matchState.handNumber,
    };
  }

  const dismissErrorToast = useCallback(() => {
    if (errorToastTimerRef.current !== null) {
      clearTimeout(errorToastTimerRef.current);
      errorToastTimerRef.current = null;
    }
    setErrorToast(null);
  }, []);

  type OverlayPhase = "normal" | "capot_animation" | "score_reveal" | "match_result";
  const [overlayPhase, setOverlayPhase] = useState<OverlayPhase>("normal");

  const [handCompleteAcked, setHandCompleteAcked] = useState(false);

  const [isChatOpen, setIsChatOpen] = useState(false);

  useEffect(() => {
    return () => {
      useChatStore.getState().clearMatch();
    };
  }, []);

  useEffect(() => {
    if (scoreRevealData !== null && overlayPhase === "normal" && pendingResolvedTrick === null) {
      if (scoreRevealData.capot) {
        setOverlayPhase("capot_animation");
      } else {
        setOverlayPhase("score_reveal");
      }
    }
  }, [scoreRevealData, overlayPhase, pendingResolvedTrick]);

  const prevHandCompletePhaseRef = useRef<string | undefined>(undefined);
  useEffect(() => {
    const phase = matchState?.phase;
    if (prevHandCompletePhaseRef.current === "hand_complete" && phase !== "hand_complete") {
      setScoreRevealData(null);
      setHandCompleteAcked(false);
      setOverlayPhase("normal");
    }
    prevHandCompletePhaseRef.current = phase;
  }, [matchState?.phase, setScoreRevealData]);

  const rebuiltScoreRevealRef = useRef<HandScoredPayload | null>(null);
  useEffect(() => {
    if (
      matchState?.phase === "hand_complete" &&
      scoreRevealData === null &&
      matchState.lastHandResult
    ) {
      const rebuilt: HandScoredPayload = {
        ...matchState.lastHandResult,
        teamAMatchScore: matchState.teamScores[0],
        teamBMatchScore: matchState.teamScores[1],
      };
      rebuiltScoreRevealRef.current = rebuilt;
      setScoreRevealData(rebuilt);
    }
  }, [matchState, scoreRevealData, setScoreRevealData]);

  useEffect(() => {
    const phase = matchState?.phase;
    const overlayCovering =
      phase === "paused" ||
      phase === "disconnected" ||
      matchEndData !== null ||
      matchAbandonedData !== null ||
      overlayPhase !== "normal";
    if (overlayCovering) {
      setDeclarationReveal(null);
      setBelotReveal(null);
    }
  }, [
    matchState?.phase,
    matchEndData,
    matchAbandonedData,
    overlayPhase,
    setDeclarationReveal,
    setBelotReveal,
  ]);

  const [declRevealReady, setDeclRevealReady] = useState(false);
  useEffect(() => {
    if (declarationReveal === null) {
      setDeclRevealReady(false);
      return;
    }
    if (pendingResolvedTrick === null) {
      setDeclRevealReady(true);
    }
  }, [declarationReveal, pendingResolvedTrick]);

  useEffect(() => {
    if (matchState && user && myPlayerSeat === null) {
      const myPlayer = matchState.players.find((p) => p.userId === user.id);
      if (myPlayer !== undefined) {
        setMyPlayerSeat(myPlayer.seat);
      }
    }
  }, [matchState, user, myPlayerSeat, setMyPlayerSeat]);

  useEffect(() => {
    if (
      matchState &&
      matchState.phase === "match_end" &&
      matchEndData === null &&
      matchAbandonedData === null
    ) {
      clearGame();
      navigate("/lobby", { replace: true });
    }
  }, [matchState, matchEndData, matchAbandonedData, clearGame, navigate]);

  type SplashIssue = "noActiveSession" | "notMember" | "invalid";
  const wsConnectionState = useWsConnectionState();
  const [splashIssue, setSplashIssue] = useState<SplashIssue | null>(null);

  useEffect(() => {
    if (!roomIdNum || !user) return;
    let cancelled = false;
    getRoom(roomIdNum)
      .then((detail) => {
        if (cancelled) return;
        roomBuyInRef.current = detail.room.coinBuyIn;
        roomMinHonorRef.current = detail.room.minHonor;
        setRoomBuyIn(detail.room.coinBuyIn);
        const isPlayer = detail.players.some((p) => p.userId === user.id);
        if (!isPlayer) {
          setSplashIssue("notMember");
          return;
        }
        if (detail.room.status !== "waiting" && detail.room.status !== "in_progress") {
          setSplashIssue("noActiveSession");
        }
      })
      .catch(() => {
        if (cancelled) return;
        setSplashIssue("invalid");
      });
    return () => {
      cancelled = true;
    };
  }, [roomIdNum, user]);

  useEffect(() => {
    if (matchState) {
      if (splashIssue === "noActiveSession") setSplashIssue(null);
      return;
    }
    if (splashIssue !== null) return;
    if (wsConnectionState !== "connected") return;
    const detectTimer = window.setTimeout(() => {
      setSplashIssue("noActiveSession");
    }, 2500);
    return () => clearTimeout(detectTimer);
  }, [matchState, wsConnectionState, splashIssue]);

  useEffect(() => {
    if (splashIssue === null || matchState) return;
    const redirectTimer = window.setTimeout(() => {
      clearGame();
      navigate("/lobby", { replace: true });
    }, 2000);
    return () => clearTimeout(redirectTimer);
  }, [splashIssue, matchState, clearGame, navigate]);

  useEffect(() => {
    if (
      matchEndData !== null &&
      overlayPhase === "normal" &&
      scoreRevealData === null &&
      pendingResolvedTrick === null
    ) {
      setOverlayPhase("match_result");
    }
  }, [matchEndData, overlayPhase, scoreRevealData, pendingResolvedTrick]);

  useEffect(() => {
    const phase = matchState?.phase;
    if (matchEndData !== null && phase !== undefined && phase !== "match_end") {
      setMatchEndData(null);
      setOverlayPhase("normal");
    }
  }, [matchState?.phase, matchEndData, setMatchEndData]);

  const surrenderActionInFlightRef = useRef(false);

  useEffect(() => {
    if (!lastError) return;
    setAnsweredDeadline(null);
    setAnsweredWindowHand(null);
    const ERROR_I18N: Record<string, string> = {
      "error:wrong_phase": "match.errors.wrongPhase",
      "error:not_your_turn": "match.errors.notYourTurn",
      "error:invalid_action": "match.errors.invalidAction",
      "error:illegal_play": "match.errors.illegalPlay",
      "error:pause_exhausted": "match.errors.pauseExhausted",
      "error:no_active_pause": "match.errors.noActivePause",
      "error:not_room_owner": "match.errors.notRoomOwner",
      "error:must_pick_trump": "match.errors.mustPickTrump",
    };
    const SURRENDER_ERROR_I18N: Record<string, string> = {
      "error:invalid_action": "match.surrender.errors.actionRequired",
      "error:wrong_phase": "match.surrender.errors.wrongPhase",
    };
    const surrenderKey = surrenderActionInFlightRef.current
      ? SURRENDER_ERROR_I18N[lastError]
      : undefined;
    surrenderActionInFlightRef.current = false;
    const i18nKey = surrenderKey ?? ERROR_I18N[lastError];
    setLastError(null);
    if (!i18nKey) return;
    if (errorToastTimerRef.current !== null) {
      clearTimeout(errorToastTimerRef.current);
    }
    setErrorToast(t(i18nKey));
    errorToastTimerRef.current = window.setTimeout(() => {
      setErrorToast(null);
      errorToastTimerRef.current = null;
    }, MOTION.TOAST_ERROR);
  }, [lastError, setLastError, t]);

  useEffect(() => {
    return () => {
      if (errorToastTimerRef.current !== null) {
        clearTimeout(errorToastTimerRef.current);
      }
      if (flyingClearTimerRef.current !== null) {
        clearTimeout(flyingClearTimerRef.current);
      }
    };
  }, []);

  const matchActive =
    matchState !== null &&
    matchState.phase !== "match_end" &&
    matchEndData === null &&
    matchAbandonedData === null;
  const shouldBlockBack = useCallback<BlockerFunction>(
    ({ historyAction }) => matchActive && historyAction === NavigationType.Pop,
    [matchActive],
  );
  const backBlocker = useBlocker(shouldBlockBack);
  useEffect(() => {
    if (backBlocker.state !== "blocked") return;
    if (window.confirm(t("match.leaveConfirm"))) {
      clearGame();
      if (backBlocker.location.pathname === "/lobby") {
        backBlocker.proceed();
      } else {
        backBlocker.reset();
        navigate("/lobby", { replace: true });
      }
    } else {
      backBlocker.reset();
    }
  }, [backBlocker, clearGame, navigate, t]);

  useEffect(() => {
    return () => {
      if (window.location.pathname.startsWith("/match")) return;
      const store = useMatchStore.getState();
      if (
        store.matchEndData !== null ||
        store.matchAbandonedData !== null ||
        store.matchState?.phase === "match_end"
      ) {
        store.clearGame();
      }
    };
  }, []);

  const executePlayCard = useCallback(
    (cardId: string) => {
      let flight: CardFlightDescriptor | null = null;
      if (!prefersReducedMotion && myPlayerSeat !== null) {
        const liveGameState = useMatchStore.getState().matchState;
        const myPlayer = liveGameState?.players.find((p) => p.seat === myPlayerSeat);
        const card = myPlayer?.hand.find((c) => `${c.rank}${c.suit}` === cardId);
        const sourceRect = rectFrom(document.querySelector(`[data-testid="hand-card-${cardId}"]`));
        const destRect = measureTrickSlotRect(0);
        if (card && sourceRect && destRect) {
          flight = {
            id: `throw-self-${cardId}-${Date.now()}`,
            card,
            fromRect: sourceRect,
            toRect: destRect,
            waypointRect: viewportBottomCenterRect(),
            durationMs: MOTION.CARD_FLIGHT_SELF_THROW,
          };
        }
      }

      setFlyingCardId(cardId);
      const flightToPush = flight;
      if (flightToPush) {
        setActiveFlights((prev) => [...prev, flightToPush]);
      }
      if (flyingClearTimerRef.current !== null) {
        clearTimeout(flyingClearTimerRef.current);
      }
      flyingClearTimerRef.current = window.setTimeout(() => {
        setFlyingCardId(null);
        flyingClearTimerRef.current = null;
      }, FLAG_LIFETIME.FLYING_CARD);
      playSfx("cardPlay");
      markTurnAnswered();
      sendMessage(ACTION_PLAY_CARD, { cardId });
    },
    [sendMessage, prefersReducedMotion, myPlayerSeat, markTurnAnswered],
  );

  const handlePlayCard = useCallback(
    (cardId: string) => {
      const live = useMatchStore.getState().matchState;
      const trump = live?.trumpSuit ?? null;
      if (trump !== null && myPlayerSeat !== null) {
        const hand = live?.players.find((p) => p.seat === myPlayerSeat)?.hand ?? [];
        const card = hand.find((c) => `${c.rank}${c.suit}` === cardId);
        if (card && isBelotEligible(card, hand, trump)) {
          setBelotPromptCardId(cardId);
          return;
        }
      }
      executePlayCard(cardId);
    },
    [executePlayCard, myPlayerSeat],
  );

  const handleLocalBelotDecision = useCallback(
    (announce: boolean) => {
      const cardId = belotPromptCardId;
      if (cardId === null) return;
      setBelotPromptCardId(null);
      setPendingLocalBelotChoice({ cardId, announce });
      executePlayCard(cardId);
    },
    [belotPromptCardId, executePlayCard],
  );

  const handlePickTrump = useCallback(
    (suit?: Suit) => {
      markTurnAnswered();
      sendMessage(ACTION_PICK_TRUMP, suit ? { suit } : {});
    },
    [sendMessage, markTurnAnswered],
  );

  const handlePassTrump = useCallback(() => {
    markTurnAnswered();
    sendMessage(ACTION_PASS_TRUMP, {});
  }, [sendMessage, markTurnAnswered]);

  const handleDeclare = useCallback(() => {
    markWindowAnswered();
    sendMessage(ACTION_DECLARE, {});
  }, [sendMessage, markWindowAnswered]);

  const handleSkipDeclare = useCallback(() => {
    markWindowAnswered();
    sendMessage(ACTION_SKIP_DECLARE, {});
  }, [sendMessage, markWindowAnswered]);

  const handleAnnounceBelot = useCallback(() => {
    markTurnAnswered();
    sendMessage(ACTION_ANNOUNCE_BELOT, {});
  }, [sendMessage, markTurnAnswered]);

  const handleDeclineBelot = useCallback(() => {
    markTurnAnswered();
    sendMessage(ACTION_DECLINE_BELOT, {});
  }, [sendMessage, markTurnAnswered]);

  const handlePause = useCallback(() => {
    sendMessage(ACTION_PAUSE, {});
  }, [sendMessage]);

  const handleUnpause = useCallback(() => {
    sendMessage(ACTION_UNPAUSE, {});
  }, [sendMessage]);

  const handleOwnerUnpause = useCallback(() => {
    sendMessage(ACTION_OWNER_UNPAUSE, {});
  }, [sendMessage]);

  const handleSendEmote = useCallback(
    (emote: EmoteID) => {
      sendMessage(ACTION_EMOTE, { emote });
    },
    [sendMessage],
  );

  const handleSurrenderRequest = useCallback(() => {
    surrenderActionInFlightRef.current = true;
    sendMessage(ACTION_SURRENDER_REQUEST, {});
  }, [sendMessage]);

  const handleSurrenderAccept = useCallback(() => {
    surrenderActionInFlightRef.current = true;
    sendMessage(ACTION_SURRENDER_ACCEPT, {});
  }, [sendMessage]);

  const handleSurrenderDecline = useCallback(() => {
    surrenderActionInFlightRef.current = true;
    sendMessage(ACTION_SURRENDER_DECLINE, {});
  }, [sendMessage]);

  const handleDeclarationRevealComplete = useCallback(() => {
    setDeclarationReveal(null);
  }, [setDeclarationReveal]);

  const handleBelotRevealComplete = useCallback(() => {
    setBelotReveal(null);
  }, [setBelotReveal]);

  const handleTrumpRevealComplete = useCallback(() => {
    setTrumpReveal(null);
  }, [setTrumpReveal]);

  const handleCapotComplete = useCallback(() => {
    setOverlayPhase("score_reveal");
  }, []);

  const handleScoreRevealContinue = useCallback(() => {
    if (matchEndData !== null) {
      setScoreRevealData(null);
      setOverlayPhase("match_result");
      return;
    }
    sendMessage(ACTION_CONTINUE, {});
    setHandCompleteAcked(true);
  }, [matchEndData, sendMessage, setScoreRevealData]);

  const handleReturnToLobby = useCallback(() => {
    if (roomIdNum !== null) {
      void leaveRoom(roomIdNum).catch(() => {});
    }
    setMatchEndData(null);
    clearGame();
    returnToLobby();
  }, [roomIdNum, clearGame, returnToLobby, setMatchEndData]);

  const handleReturnToRoom = useCallback(async () => {
    if (roomIdNum === null) {
      setMatchEndData(null);
      clearGame();
      returnToLobby();
      return;
    }
    try {
      await returnToRoom(roomIdNum);
      setMatchEndData(null);
      clearGame();
      navigate(`/rooms/${roomIdNum}`, { replace: true });
    } catch (err) {
      if (errorToastTimerRef.current !== null) {
        clearTimeout(errorToastTimerRef.current);
      }
      const code = err instanceof FetchError ? err.code : null;
      if (code === "INSUFFICIENT_COINS") {
        if (
          roomBuyInRef.current !== null &&
          user !== null &&
          useRoomStore.getState().roomEjection === null
        ) {
          setRoomEjection({
            roomId: roomIdNum,
            buyIn: roomBuyInRef.current,
            balance: user.walletBalance,
            reason: "insolvent",
          });
        }
        setMatchEndData(null);
        clearGame();
        returnToLobby();
        return;
      }
      if (code === "HONOR_TOO_LOW" || code === "NEW_PLAYER_NOT_ALLOWED") {
        if (
          roomMinHonorRef.current !== null &&
          user !== null &&
          useRoomStore.getState().roomEjection === null
        ) {
          setRoomEjection({
            roomId: roomIdNum,
            buyIn: 0,
            balance: 0,
            minHonor: roomMinHonorRef.current,
            honor: honorScoreOrPrior(user.honorScore),
            reason: "honor",
          });
        }
        setMatchEndData(null);
        clearGame();
        returnToLobby();
        return;
      }
      const messageKey =
        code === "MATCH_ALREADY_STARTED"
          ? "match.matchResult.returnToRoomAlreadyStarted"
          : code === "NOT_IN_ROOM"
            ? "match.matchResult.returnToRoomNotInRoom"
            : "match.matchResult.returnToRoomError";
      setErrorToast(t(messageKey));
      errorToastTimerRef.current = window.setTimeout(() => {
        setErrorToast(null);
        errorToastTimerRef.current = null;
      }, MOTION.TOAST_ERROR);
    }
  }, [roomIdNum, clearGame, navigate, returnToLobby, setMatchEndData, setRoomEjection, user, t]);

  const handleAbandonReturnToLobby = useCallback(() => {
    setMatchAbandonedData(null);
    clearGame();
    returnToLobby();
  }, [clearGame, returnToLobby, setMatchAbandonedData]);

  const promptPlayer = matchState?.players.find((p) => p.seat === myPlayerSeat);
  const declarationOverlap = rulesForVariant(matchState?.variant).declarationOverlap;
  const promptHandKey = (promptPlayer?.hand ?? []).map((c) => `${c.rank}${c.suit}`).join(",");
  const promptDeclarations = useMemo(
    () =>
      promptPlayer !== undefined && promptPlayer.declarations.length > 0
        ? promptPlayer.declarations
        : detectDeclarations(promptPlayer?.hand ?? [], declarationOverlap),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- promptHandKey stands in for the hand array, whose identity churns every snapshot
    [promptPlayer?.declarations, promptHandKey, declarationOverlap],
  );

  if (!splashElapsed || !matchState || myPlayerSeat === null) {
    return (
      <div
        className="game-table h-screen w-screen overflow-hidden flex items-center justify-center"
        data-testid="match-page"
        data-splash-active="true"
        style={{
          background:
            "radial-gradient(ellipse at 50% 50%, var(--felt-dark) 0%, var(--felt-deep) 60%, var(--felt-bg) 100%)",
        }}
      >
        <div className="flex flex-col items-center gap-4" data-testid="match-starting-splash">
          <span
            aria-hidden
            className="inline-block rounded-full motion-safe:animate-spin"
            style={{
              width: 36,
              height: 36,
              border: "2px solid var(--brass-soft)",
              borderTopColor: "var(--brass)",
              animationDuration: "0.9s",
            }}
          />
          <span
            className="font-body text-base"
            style={{
              color: "var(--ink-light)",
              fontFamily: "var(--font-body)",
              letterSpacing: 0.3,
              opacity: 0.85,
            }}
            data-testid="splash-text"
          >
            {splashIssue === "invalid"
              ? t("match.roomNotFound")
              : splashIssue === "notMember"
                ? t("match.notMember")
                : splashIssue === "noActiveSession"
                  ? t("match.noActiveMatch")
                  : cameFromRoom
                    ? t("match.starting")
                    : t("match.reconnecting")}
          </span>
        </div>
      </div>
    );
  }

  const viewerTeam: TeamString = myPlayerSeat % 2 === 0 ? "teamA" : "teamB";

  const matchTarget = matchTargetFor(matchState.matchMode);

  const matchStake =
    roomBuyIn !== null ? matchState.players.filter((p) => !p.isBot).length * roomBuyIn : 0;

  const isRoomOwner = myPlayerSeat !== null && matchState.ownerSeat === myPlayerSeat;
  const isPaused = matchState.phase === "paused";

  const isTurnPhase =
    matchState.phase === "playing" ||
    matchState.phase === "bidding" ||
    matchState.phase === "declaring";

  const isOverlayActive =
    isPaused ||
    matchEndData !== null ||
    matchAbandonedData !== null ||
    matchState.phase === "disconnected";
  const isPausablePhase = isTurnPhase && matchState.phase !== "declaring";
  const canPause =
    !isPaused && isPausablePhase && myPlayerSeat !== null && !matchState.pauseUsed?.[myPlayerSeat];

  const surrenderProposerSeat = matchState.surrenderProposerSeat;
  const isProposer = surrenderProposerSeat !== null && surrenderProposerSeat === myPlayerSeat;
  const isPartnerOfProposer =
    surrenderProposerSeat !== null &&
    myPlayerSeat !== null &&
    (surrenderProposerSeat + 2) % 4 === myPlayerSeat;
  const isOpponentOfProposer =
    surrenderProposerSeat !== null && myPlayerSeat !== null && !isProposer && !isPartnerOfProposer;
  const showSurrenderControls = myPlayerSeat !== null && isTurnPhase;
  const canSurrenderRequest =
    showSurrenderControls &&
    surrenderProposerSeat === null &&
    myPlayerSeat !== null &&
    !matchState.surrenderUsed?.[myPlayerSeat];
  const proposerPlayer =
    surrenderProposerSeat !== null
      ? matchState.players.find((p) => p.seat === surrenderProposerSeat)
      : undefined;
  const proposerUsername =
    playerDisplayName(t, proposerPlayer) ?? t("match.surrender.unknownProposer");

  const surrenderedByUsername =
    matchEndData?.outcomeReason === "surrender" &&
    typeof matchEndData.surrenderedBySeat === "number"
      ? (playerDisplayName(
          t,
          matchState.players.find((p) => p.seat === matchEndData.surrenderedBySeat),
        ) ?? undefined)
      : undefined;

  const isMyTurn =
    matchState.activePlayerSeat === myPlayerSeat &&
    matchState.phase === "playing" &&
    !matchState.awaitingDeclaration &&
    matchState.pendingBelotSeat !== myPlayerSeat;
  const canPlayNow = isMyTurn && pendingResolvedTrick === null && !dealing;
  const myPlayer = matchState.players.find((p) => p.seat === myPlayerSeat);
  const shownHand = dealView.visibleHand(myPlayer?.hand ?? [], myPlayer?.faceDownCount ?? 0);
  const myHand = shownHand.hand;
  const myFaceDownCount = shownHand.faceDownCount;
  const playableCardIds =
    canPlayNow && myPlayerSeat !== null ? legalCardIds(matchState, myPlayerSeat) : [];

  const isBiddingPhase = matchState.phase === "bidding";
  const showTrumpPrompt = isBiddingPhase && !dealing;
  const isActiveBidder = showTrumpPrompt && matchState.activePlayerSeat === myPlayerSeat;

  const isDeclarationPhase = matchState.phase === "declaring";
  const declarationAnsweredCount = matchState.players.filter((p) => p.declarationAnswered).length;
  const showDeclarationPrompt =
    trumpReveal === null &&
    !dealing &&
    (isDeclarationPhase
      ? myPlayer !== undefined && declarationReveal === null
      : matchState.awaitingDeclaration === true && matchState.activePlayerSeat === myPlayerSeat);

  const ownTurnDeadline =
    !dealing &&
    (matchState.phase === "bidding" || matchState.phase === "playing") &&
    matchState.activePlayerSeat === myPlayerSeat &&
    matchState.turnExpiresAt !== answeredDeadline
      ? matchState.turnExpiresAt
      : null;
  const ownWindowOpen =
    isDeclarationPhase &&
    showDeclarationPrompt &&
    myPlayer !== undefined &&
    !myPlayer.declarationAnswered &&
    answeredWindowHand !== matchState.handNumber;

  const showLocalBelotPrompt = belotPromptCardId !== null;
  const localBelotIsKing = belotPromptCardId?.startsWith("K") ?? false;
  const showBelotPrompt =
    matchState.pendingBelotSeat === myPlayerSeat &&
    pendingLocalBelotChoice === null &&
    !showLocalBelotPrompt;
  const belotPromptLastTrickCard = showBelotPrompt
    ? (matchState.currentTrick[matchState.currentTrick.length - 1] ?? null)
    : null;
  const belotPromptIsKing = belotPromptLastTrickCard?.card.rank === "K";

  const emotesVisible =
    matchEndData === null &&
    matchAbandonedData === null &&
    matchState.phase !== "paused" &&
    matchState.phase !== "disconnected";

  const showSurrenderBanner =
    isOpponentOfProposer &&
    myPlayerSeat !== null &&
    surrenderProposerSeat !== null &&
    matchEndData === null &&
    matchAbandonedData === null;

  return (
    <div
      className="game-table h-screen w-screen overflow-hidden relative bg-background"
      data-testid="match-page"
    >
      <TableBackdrop compact={isCompactTable} />
      <TableAmbience intensity={0.9} tint="#ffe9b0" />
      <Wordmark />

      <ScorePanel
        viewerTeam={viewerTeam}
        teamAScore={matchState.teamScores[0]}
        teamBScore={matchState.teamScores[1]}
        teamATricks={matchState.tricksWon[0]}
        teamBTricks={matchState.tricksWon[1]}
        teamAHandPotential={
          matchState.handPoints[0] + matchState.declarationPoints[0] + matchState.belotPoints[0]
        }
        teamBHandPotential={
          matchState.handPoints[1] + matchState.declarationPoints[1] + matchState.belotPoints[1]
        }
        lastTrickBonus={scoreRevealData?.lastTrickBonus}
        lastTrickTeam={scoreRevealData?.lastTrickTeam}
        handNumber={matchState.handNumber}
        variantLabel={variantLabel(t, matchState.variant)}
        matchTarget={matchTarget}
      />

      <div
        className="absolute top-4 right-4 hidden flex-col items-end gap-2 md:flex"
        style={{ zIndex: Z.HUD }}
      >
        {!isOverlayActive &&
          matchState.trumpSuit &&
          matchState.phase !== "dealing" &&
          matchState.phase !== "bidding" && (
            <TrumpIndicator
              trumpSuit={matchState.trumpSuit}
              trumpCallerSeat={matchState.trumpCallerSeat}
              trumpCallerName={
                matchState.trumpCallerSeat !== null
                  ? playerDisplayName(
                      t,
                      matchState.players.find((p) => p.seat === matchState.trumpCallerSeat),
                    )
                  : null
              }
              viewerTeam={viewerTeam}
            />
          )}
        {matchStake > 0 && <StakePill amount={matchStake} />}
      </div>

      {matchState.players.map((player) => {
        const compass = compassOffset(player.seat, myPlayerSeat);
        const isSelf = player.seat === myPlayerSeat;
        const isActive =
          !dealing && !isDeclarationPhase && matchState.activePlayerSeat === player.seat;
        const isCaller =
          matchState.trumpCallerSeat !== null &&
          matchState.trumpCallerSeat === player.seat &&
          matchState.trumpSuit !== null;

        const emoteSlot =
          isCompactTable && emotesVisible ? activeEmotes[player.seat as 0 | 1 | 2 | 3] : null;
        const declareSlot = isCompactTable ? activeDeclares[player.seat as 0 | 1 | 2 | 3] : null;

        return (
          <div
            key={player.seat}
            className={`absolute ${SEAT_POSITIONS[compass]}`}
            style={{ zIndex: Z.SEATS }}
            data-testid={`player-seat-${compass}-wrapper`}
          >
            <PlayerSeat
              player={player}
              isSelf={isSelf}
              isActive={isActive}
              seatTeam={seatTeam(player.seat, myPlayerSeat)}
              cardCount={
                isSelf
                  ? undefined
                  : (dealView.seatCardCount(player.seat) ??
                    (player.handCount ?? player.hand.length) + (player.faceDownCount ?? 0))
              }
              turnExpiresAt={isActive && isTurnPhase ? matchState.turnExpiresAt : null}
              timerDuration={matchState.timerDurationSec}
              isDealer={matchState.dealerSeat === player.seat}
              trumpCallerSuit={isCaller ? matchState.trumpSuit : null}
              orientation={SEAT_ORIENTATIONS[compass]}
              compact={isCompactTable}
            />
            {emoteSlot && (
              <EmoteBubble
                key={`emote-${player.seat}-${emoteSlot.receivedAt}`}
                emote={emoteSlot.emote}
                compassPosition={compass as 0 | 1 | 2 | 3}
                compact
                onDismiss={() => setActiveEmote(player.seat, null)}
              />
            )}
            {isCompactTable && showSurrenderBanner && player.seat === surrenderProposerSeat && (
              <SurrenderOpponentBanner
                proposerUsername={proposerUsername}
                compassPosition={compass as 0 | 1 | 2 | 3}
                compact
              />
            )}
            {declareSlot && (
              <DeclareBanner
                key={`declare-${player.seat}-${declareSlot.receivedAt}`}
                declarerUsername={playerDisplayName(t, player) ?? player.username}
                compassPosition={compass as 0 | 1 | 2 | 3}
                compact
                onDismiss={() => setActiveDeclare(player.seat, false)}
              />
            )}
          </div>
        );
      })}

      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2">
        <TrickArea
          trick={matchState.currentTrick}
          winnerSeat={matchState.trickWinnerSeat}
          myPlayerSeat={myPlayerSeat}
          pendingResolvedTrick={pendingResolvedTrick}
          suppressedCardIds={suppressedCardIds}
          compact={isCompactTable}
        />
      </div>

      <div
        className="absolute bottom-4 left-1/2 -translate-x-1/2"
        style={isActiveBidder ? { zIndex: Z.BIDDER_HAND } : undefined}
      >
        <HandCards
          hand={myHand}
          faceDownCount={myFaceDownCount}
          isMyTurn={canPlayNow}
          playableCardIds={playableCardIds}
          onPlayCard={handlePlayCard}
          flyingId={flyingCardId}
          compact={isCompactTable}
        />
      </div>

      {isTurnPhase && (
        <div className="absolute bottom-4 left-4 z-10 hidden items-center gap-2 md:flex">
          {isPausablePhase && (
            <HUDButton
              icon={<Pause className="h-4 w-4" aria-hidden="true" />}
              label={
                matchState.pauseUsed?.[myPlayerSeat]
                  ? t("match.pause.pauseUsed")
                  : t("match.hud.pause")
              }
              onClick={handlePause}
              disabled={!canPause}
              data-testid="pause-button"
            />
          )}

          <SurrenderButton
            canRequest={canSurrenderRequest}
            isExhausted={!!matchState.surrenderUsed?.[myPlayerSeat]}
            isPending={surrenderProposerSeat !== null}
            onConfirm={handleSurrenderRequest}
          />
        </div>
      )}

      {overlayPhase === "normal" && !isChatOpen && (
        <div
          className="absolute bottom-4 right-24 hidden items-center gap-2 md:flex"
          style={{ zIndex: Z.CHROME_TOP }}
        >
          <HUDButton
            icon={<HelpCircle className="h-4 w-4" aria-hidden="true" />}
            aria-label={t("match.hud.rules")}
            title={t("match.hud.rules")}
            onClick={() => setRulesOpen(true)}
            data-testid="rules-button"
          />
          <HUDButton
            icon={<SettingsIcon className="h-4 w-4" aria-hidden="true" />}
            aria-label={t("match.hud.settings")}
            title={t("match.hud.settings")}
            onClick={() => setSettingsOpen(true)}
            data-testid="settings-button"
          />
          <HUDButton
            icon={<MuteIcon className="h-4 w-4" aria-hidden="true" />}
            aria-label={muteLabel}
            title={muteLabel}
            onClick={handleToggleMute}
            data-testid="mute-button"
          />
          {(matchState.phase === "dealing" || isTurnPhase) &&
            matchEndData === null &&
            matchAbandonedData === null && <EmotePickerButton onSend={handleSendEmote} />}
        </div>
      )}

      {matchStake > 0 && (
        <div className="absolute top-3 right-14 md:hidden" style={{ zIndex: Z.HUD }}>
          <StakePill amount={matchStake} />
        </div>
      )}

      {overlayPhase === "normal" && (
        <div className="md:hidden">
          <button
            type="button"
            onClick={() => setHudMenuOpen((v) => !v)}
            aria-label={t("nav.menu")}
            aria-expanded={hudMenuOpen}
            data-testid="hud-menu-button"
            className="absolute top-3 right-3 inline-flex size-9 items-center justify-center rounded-md"
            style={{
              zIndex: Z.CHROME_TOP,
              background: "var(--panel-deeper)",
              border: "1px solid var(--brass-40)",
              color: "var(--ink-light)",
              backdropFilter: "blur(6px)",
              WebkitBackdropFilter: "blur(6px)",
            }}
          >
            {hudMenuOpen ? (
              <X className="h-5 w-5" aria-hidden="true" />
            ) : (
              <MenuIcon className="h-5 w-5" aria-hidden="true" />
            )}
          </button>

          {hudMenuOpen && (
            <>
              <button
                type="button"
                aria-hidden="true"
                tabIndex={-1}
                onClick={() => setHudMenuOpen(false)}
                className="fixed inset-0 cursor-default"
                style={{ zIndex: Z.CHROME_TOP }}
              />
              <div
                className="absolute top-14 right-3 flex w-44 flex-col gap-1.5 rounded-xl p-2"
                style={{
                  zIndex: Z.CHROME_TOP,
                  background: "var(--panel-deeper)",
                  border: "1px solid var(--brass-40)",
                  boxShadow: "0 14px 36px rgba(0,0,0,0.5)",
                  backdropFilter: "blur(10px)",
                  WebkitBackdropFilter: "blur(10px)",
                }}
                data-testid="hud-menu"
              >
                {isTurnPhase && (
                  <>
                    {isPausablePhase && (
                      <HUDButton
                        icon={<Pause className="h-4 w-4" aria-hidden="true" />}
                        label={
                          matchState.pauseUsed?.[myPlayerSeat]
                            ? t("match.pause.pauseUsed")
                            : t("match.hud.pause")
                        }
                        onClick={() => {
                          handlePause();
                          setHudMenuOpen(false);
                        }}
                        disabled={!canPause}
                        style={{ width: "100%", justifyContent: "flex-start" }}
                        data-testid="hud-menu-pause"
                      />
                    )}
                    <HUDButton
                      variant="danger"
                      icon={<Flag className="h-4 w-4" aria-hidden="true" />}
                      label={t("match.surrender.requestButton")}
                      onClick={() => {
                        setSurrenderConfirmOpen(true);
                        setHudMenuOpen(false);
                      }}
                      disabled={!canSurrenderRequest}
                      style={{ width: "100%", justifyContent: "flex-start" }}
                      data-testid="hud-menu-surrender"
                    />
                  </>
                )}
                <HUDButton
                  icon={<HelpCircle className="h-4 w-4" aria-hidden="true" />}
                  label={t("match.hud.rules")}
                  onClick={() => {
                    setRulesOpen(true);
                    setHudMenuOpen(false);
                  }}
                  style={{ width: "100%", justifyContent: "flex-start" }}
                  data-testid="hud-menu-rules"
                />
                <HUDButton
                  icon={<SettingsIcon className="h-4 w-4" aria-hidden="true" />}
                  label={t("match.hud.settings")}
                  onClick={() => {
                    setSettingsOpen(true);
                    setHudMenuOpen(false);
                  }}
                  style={{ width: "100%", justifyContent: "flex-start" }}
                  data-testid="hud-menu-settings"
                />
                <HUDButton
                  icon={<MuteIcon className="h-4 w-4" aria-hidden="true" />}
                  label={muteLabel}
                  onClick={() => {
                    handleToggleMute();
                    setHudMenuOpen(false);
                  }}
                  style={{ width: "100%", justifyContent: "flex-start" }}
                  data-testid="hud-menu-mute"
                />
                {(matchState.phase === "dealing" || isTurnPhase) &&
                  matchEndData === null &&
                  matchAbandonedData === null && (
                    <EmotePickerButton
                      onSend={handleSendEmote}
                      onSent={() => setHudMenuOpen(false)}
                      label={t("match.emote.button")}
                      openDirection="down"
                    />
                  )}
              </div>
            </>
          )}
        </div>
      )}

      {surrenderConfirmOpen && canSurrenderRequest && (
        <SurrenderConfirmDialog
          onCancel={() => setSurrenderConfirmOpen(false)}
          onConfirm={() => {
            setSurrenderConfirmOpen(false);
            handleSurrenderRequest();
          }}
        />
      )}

      {isPartnerOfProposer && matchEndData === null && matchAbandonedData === null && (
        <SurrenderPrompt
          proposerUsername={proposerUsername}
          onAccept={handleSurrenderAccept}
          onDecline={handleSurrenderDecline}
        />
      )}

      {!isCompactTable && showSurrenderBanner && surrenderProposerSeat !== null && (
        <SurrenderOpponentBanner
          proposerUsername={proposerUsername}
          compassPosition={compassOffset(surrenderProposerSeat, myPlayerSeat) as 0 | 1 | 2 | 3}
        />
      )}

      {isPaused && (
        <PauseOverlay
          pausedPlayers={matchState.pausedPlayers}
          pauseUsed={matchState.pauseUsed}
          players={matchState.players}
          myPlayerSeat={myPlayerSeat}
          isRoomOwner={isRoomOwner}
          onResume={handleUnpause}
          onPause={handlePause}
          onOwnerResume={handleOwnerUnpause}
        />
      )}

      {((matchState.phase === "disconnected" &&
        matchState.disconnectedSeat !== -1 &&
        matchState.reconnectExpiresAt) ||
        matchAbandonedData) && (
        <ReconnectOverlay
          disconnectedPlayerName={
            matchAbandonedData
              ? (playerDisplayName(t, matchState.players[matchAbandonedData.abandonedByPlayer]) ??
                `Player ${matchAbandonedData.abandonedByPlayer + 1}`)
              : (playerDisplayName(t, matchState.players[matchState.disconnectedSeat]) ??
                `Player ${matchState.disconnectedSeat + 1}`)
          }
          disconnectedPlayers={matchState.players
            .map((p) => {
              const expiry = matchState.playerReconnectExpiresAt[p.seat];
              if (p.connected || !expiry) return null;
              return {
                name: playerDisplayName(t, p) ?? `Player ${p.seat + 1}`,
                expiresAt: expiry,
              };
            })
            .filter((entry): entry is { name: string; expiresAt: string } => entry !== null)}
          reconnectExpiresAt={matchState.reconnectExpiresAt ?? ""}
          abandonedData={matchAbandonedData}
          viewerTeam={viewerTeam}
          viewerSeat={myPlayerSeat}
          seasonSettlement={seasonSettlement}
          onReturnToLobby={handleAbandonReturnToLobby}
          soundKey={matchAbandonedData ? payloadSoundKey(matchAbandonedData) : null}
        />
      )}

      {overlayPhase === "normal" && (
        <>
          {dealing && dealView.deal !== null && (
            <DealAnimation
              flippedCandidate={
                dealView.deal.kind === "candidateFirst" && dealView.deal.flipped
                  ? dealView.deal.candidate
                  : null
              }
              isReshuffle={dealView.deal.isReshuffle}
              prefersReducedMotion={prefersReducedMotion}
            />
          )}

          {showTrumpPrompt && (
            <TrumpPrompt
              trumpCandidate={matchState.trumpCandidate}
              biddingRound={matchState.biddingRound}
              isActiveBidder={isActiveBidder}
              activePlayerName={
                playerDisplayName(
                  t,
                  matchState.players.find((p) => p.seat === matchState.activePlayerSeat),
                ) ?? null
              }
              activePlayerTeam={
                matchState.activePlayerSeat !== null
                  ? seatTeam(matchState.activePlayerSeat, myPlayerSeat)
                  : null
              }
              onPick={handlePickTrump}
              onPass={handlePassTrump}
              canPass={!matchState.mustPickTrump}
              turnExpiresAt={matchState.turnExpiresAt}
              timerDurationSec={matchState.timerDurationSec}
            />
          )}
        </>
      )}

      {showDeclarationPrompt && myPlayer && (
        <DeclarationPrompt
          declarations={promptDeclarations}
          onDeclare={handleDeclare}
          onSkip={handleSkipDeclare}
          turnExpiresAt={matchState.turnExpiresAt}
          timerDurationSec={matchState.timerDurationSec}
          simultaneous={isDeclarationPhase}
          answered={isDeclarationPhase && myPlayer.declarationAnswered}
          answeredCount={declarationAnsweredCount}
        />
      )}

      {showLocalBelotPrompt && matchState.trumpSuit && (
        <BelotPrompt
          isKing={localBelotIsKing}
          trumpSuit={matchState.trumpSuit}
          onAnnounce={() => handleLocalBelotDecision(true)}
          onDecline={() => handleLocalBelotDecision(false)}
          turnExpiresAt={matchState.turnExpiresAt}
          timerDurationSec={matchState.timerDurationSec}
        />
      )}

      {showBelotPrompt && matchState.trumpSuit && (
        <BelotPrompt
          isKing={belotPromptIsKing}
          trumpSuit={matchState.trumpSuit}
          onAnnounce={handleAnnounceBelot}
          onDecline={handleDeclineBelot}
          turnExpiresAt={matchState.turnExpiresAt}
          timerDurationSec={matchState.timerDurationSec}
        />
      )}

      {declarationReveal && !isOverlayActive && declRevealReady && (
        <DeclarationReveal
          payload={declarationReveal}
          players={matchState.players}
          viewerTeam={viewerTeam}
          onComplete={handleDeclarationRevealComplete}
          soundKey={payloadSoundKey(declarationReveal)}
        />
      )}

      {belotReveal && !isOverlayActive && (
        <BelotReveal
          key={`${belotReveal.playerSeat}-${belotReveal.cardId}`}
          playerSeat={belotReveal.playerSeat}
          myPlayerSeat={myPlayerSeat}
          cardId={belotReveal.cardId}
          isKing={belotReveal.cardId.startsWith("K")}
          players={matchState.players}
          onComplete={handleBelotRevealComplete}
          soundKey={payloadSoundKey(belotReveal)}
        />
      )}

      {trumpReveal && (
        <TrumpReveal
          key={`${trumpReveal.playerSeat}-${trumpReveal.cardId}`}
          playerSeat={trumpReveal.playerSeat}
          myPlayerSeat={myPlayerSeat}
          cardId={trumpReveal.cardId}
          trumpSuit={trumpReveal.trumpSuit as Suit}
          players={matchState.players}
          onComplete={handleTrumpRevealComplete}
        />
      )}

      <MatchChatDock isOpen={isChatOpen} onOpenChange={setIsChatOpen} />

      {!isCompactTable &&
        emotesVisible &&
        matchState.players.map((player) => {
          const slot = activeEmotes[player.seat as 0 | 1 | 2 | 3];
          if (slot === null) return null;
          const compass = compassOffset(player.seat, myPlayerSeat) as 0 | 1 | 2 | 3;
          return (
            <EmoteBubble
              key={`${player.seat}-${slot.receivedAt}`}
              emote={slot.emote}
              compassPosition={compass}
              onDismiss={() => setActiveEmote(player.seat, null)}
            />
          );
        })}

      {!isCompactTable &&
        matchState.players.map((player) => {
          const slot = activeDeclares[player.seat as 0 | 1 | 2 | 3];
          if (slot === null) return null;
          const compass = compassOffset(player.seat, myPlayerSeat) as 0 | 1 | 2 | 3;
          return (
            <DeclareBanner
              key={`declare-${player.seat}-${slot.receivedAt}`}
              declarerUsername={playerDisplayName(t, player) ?? player.username}
              compassPosition={compass}
              onDismiss={() => setActiveDeclare(player.seat, false)}
            />
          );
        })}

      {overlayPhase === "capot_animation" &&
        scoreRevealData?.capotTeam !== null &&
        scoreRevealData?.capotTeam !== undefined && (
          <CapotAnimation
            capotTeam={scoreRevealData.capotTeam}
            viewerSeat={myPlayerSeat}
            capotBonus={scoreRevealData.capotBonus}
            onComplete={handleCapotComplete}
            soundKey={
              scoreRevealData === rebuiltScoreRevealRef.current
                ? null
                : payloadSoundKey(scoreRevealData)
            }
          />
        )}

      {overlayPhase === "score_reveal" && scoreRevealData !== null && (
        <ScoreReveal
          data={scoreRevealData}
          viewerTeam={viewerTeam}
          onContinue={handleScoreRevealContinue}
          handNumber={lastTrumpRef.current.handNumber ?? matchState.handNumber}
          trumpSuit={matchState.trumpSuit ?? lastTrumpRef.current.suit}
          trumpCallerSeat={matchState.trumpCallerSeat ?? lastTrumpRef.current.callerSeat}
          acknowledged={handCompleteAcked}
          matchTarget={matchTarget}
          declarationsTransfer={matchState.declarationsTransfer}
        />
      )}

      {overlayPhase === "match_result" && matchEndData !== null && (
        <MatchResult
          data={matchEndData}
          viewerTeam={viewerTeam}
          onReturnToLobby={handleReturnToLobby}
          onReturnToRoom={handleReturnToRoom}
          surrenderedByUsername={surrenderedByUsername}
          coinDelta={coinSettlement?.coinDelta}
          honorSettlement={honorSettlement}
          seasonSettlement={seasonSettlement}
          roomId={roomIdNum ?? undefined}
          soundKey={payloadSoundKey(matchEndData)}
        />
      )}

      <UrgentTick
        turnDeadline={ownTurnDeadline}
        turnTotalSec={matchState.timerDurationSec}
        windowOpen={ownWindowOpen}
        windowTotalSec={MOTION.DECLARATION_PHASE_AUTO_SKIP / 1000}
      />

      <CardFlight flights={activeFlights} onComplete={handleFlightComplete} />

      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
      <RulesDialog
        open={rulesOpen}
        onOpenChange={setRulesOpen}
        variant={matchState.variant}
        stopAtTarget={matchState.stopAtTarget}
        declarationsTransfer={matchState.declarationsTransfer}
        clockwise={matchState.clockwise}
      />

      {errorToast && (
        <div
          className="absolute bottom-20 left-1/2 -translate-x-1/2 border border-danger/45 bg-(--panel-deeper) text-danger font-body text-sm px-4 py-2 rounded-lg flex items-center gap-3"
          style={{ zIndex: Z.TOAST }}
          role="alert"
          data-testid="error-toast"
        >
          <span>{errorToast}</span>
          <button
            type="button"
            onClick={dismissErrorToast}
            aria-label={t("common.close")}
            data-testid="error-toast-close"
            className="text-danger/80 hover:text-danger text-lg leading-none px-1 focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1 focus-visible:ring-offset-background rounded"
          >
            ×
          </button>
        </div>
      )}
    </div>
  );
}
