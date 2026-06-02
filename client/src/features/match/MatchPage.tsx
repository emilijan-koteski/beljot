import {
  Flag,
  HelpCircle,
  Menu as MenuIcon,
  Pause,
  Settings as SettingsIcon,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate, useParams } from "react-router";

import { getRoom } from "@/shared/api/rooms";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { FLAG_LIFETIME, MOTION } from "@/shared/lib/motion";
import { useWsConnectionState, useWsSendMessage } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useMatchStore } from "@/shared/stores/matchStore";
import type { Suit, TeamString } from "@/shared/types/matchTypes";
import {
  ACTION_ANNOUNCE_BELOT,
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
} from "@/shared/types/wsEvents";

import { BelotPrompt } from "./components/BelotPrompt";
import { BelotReveal } from "./components/BelotReveal";
import { CapotAnimation } from "./components/CapotAnimation";
import { CardFlight, type CardFlightDescriptor, type FlightRect } from "./components/CardFlight";
import { DealAnimation } from "./components/DealAnimation";
import { DeclarationPrompt } from "./components/DeclarationPrompt";
import { DeclarationReveal } from "./components/DeclarationReveal";
import { EmoteBubble } from "./components/EmoteBubble";
import { EmotePickerButton } from "./components/EmotePickerButton";
import { HandCards } from "./components/HandCards";
import { HUDButton } from "./components/HUDButton";
import { MatchChatDock } from "./components/MatchChatDock";
import { MatchResult } from "./components/MatchResult";
import { PauseOverlay } from "./components/PauseOverlay";
import { PlayerSeat, type SeatOrientation } from "./components/PlayerSeat";
import { ReconnectOverlay } from "./components/ReconnectOverlay";
import { ReshuffleAnimation } from "./components/ReshuffleAnimation";
import { RulesDialog } from "./components/RulesDialog";
import { ScorePanel } from "./components/ScorePanel";
import { ScoreReveal } from "./components/ScoreReveal";
import { SettingsDialog } from "./components/SettingsDialog";
import { SurrenderButton, SurrenderConfirmDialog } from "./components/SurrenderButton";
import { SurrenderOpponentBanner } from "./components/SurrenderOpponentBanner";
import { SurrenderPrompt } from "./components/SurrenderPrompt";
import { TableAmbience } from "./components/TableAmbience";
import { TableBackdrop } from "./components/TableBackdrop";
import { TRICK_SLOT_H, TRICK_SLOT_W, TrickArea } from "./components/TrickArea";
import { TrumpIndicator } from "./components/TrumpIndicator";
import { TrumpPrompt } from "./components/TrumpPrompt";
import { TrumpReveal } from "./components/TrumpReveal";
import { Wordmark } from "./components/Wordmark";
import { detectDeclarations } from "./lib/declarations";
import { legalCardIds } from "./lib/legalCards";
import { seatTeam } from "./lib/tableTheme";
import { compassOffset, SLOT_POSITIONS } from "./lib/trickLayout";

function rectFrom(el: Element | null): FlightRect | null {
  if (!el) return null;
  const r = el.getBoundingClientRect();
  if (r.width === 0 && r.height === 0) return null;
  return { left: r.left, top: r.top, width: r.width, height: r.height };
}

function measureTrickSlotRect(compass: 0 | 1 | 2 | 3): FlightRect | null {
  // Prefer the dedicated slot anchor — TrickArea always renders a
  // `data-testid="trick-slot-{compass}"` element at each slot position so
  // the overlay can measure the real DOM rect (the spec's "Always" rule).
  const slotEl = document.querySelector(`[data-testid="trick-slot-${compass}"]`);
  const slotRect = rectFrom(slotEl);
  if (slotRect) return slotRect;

  // Fallback: trick-area rect + relative slot offsets. Used when the slot
  // anchor isn't laid out yet (initial render race) or in tests where
  // jsdom doesn't compute layout.
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

function viewportBottomCenterRect(): FlightRect {
  // Mid-flight waypoint for the self throw — the card visibly arcs through
  // viewport bottom-center before continuing up to the south slot. Width
  // matches `md` so the size morph stays continuous through the waypoint.
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
 * Final rect for a trick-collect flight. Returns a TRICK_SLOT-sized rect
 * (no scaling — pure translate-and-fade) anchored to the winner's compass
 * position in viewport coords.
 *
 * Why compass + viewport, not DOM measurement: DOM measurement here turned
 * out to be flaky (deck stack visibility-toggling on the last trick of a
 * hand, and a recurring "always lands at the same seat" race when the
 * effect captured a stale rect). The seat positions in `SEAT_POSITIONS`
 * are hard-anchored to the four edges of the viewport via CSS
 * `bottom-44 / right-16 / top-16 / left-16`, so we can compute the
 * destination directly from the compass without consulting the DOM at
 * collect time.
 */
function winnerCollectRect(winner: number, myPlayerSeat: number): FlightRect {
  const compass = compassOffset(winner, myPlayerSeat);
  const vw = window.innerWidth;
  const vh = window.innerHeight;

  let cx: number;
  let cy: number;
  switch (compass) {
    case 0:
      // Self → bottom-center landing zone above the viewport edge. Cards
      // slide down from the trick area and fade — the gesture the user
      // asked for: "simple slide to the bottom center and fade them away".
      cx = vw / 2;
      cy = vh - TRICK_SLOT_H / 2 - 24;
      break;
    case 1:
      // East opponent. Tucked in from the wood rim (right-16 = 64 px) by
      // about a card width.
      cx = vw - 120;
      cy = vh / 2;
      break;
    case 2:
      // Teammate / north. Just below the top wood rim (top-16 = 64 px).
      cx = vw / 2;
      cy = 120;
      break;
    case 3:
      // West opponent. Mirror of east.
      cx = 120;
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

const SEAT_POSITIONS: Record<number, string> = {
  // Phones hug the screen edges (matching the scoreboard inset); md+ insets off
  // the wood rim as before.
  0: "bottom-48 left-1/2 -translate-x-1/2 md:bottom-44", // South (self) - above the fanned hand
  1: "right-3 top-1/2 -translate-y-1/2 md:right-16", // East (next player counter-clockwise)
  2: "top-6 left-1/2 -translate-x-1/2 md:top-16", // North (partner) - clears the wordmark on md+
  3: "left-3 top-1/2 -translate-y-1/2 md:left-16", // West (third player)
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
  const { roomId: roomIdParam } = useParams<{ roomId: string }>();
  const parsedRoomId = roomIdParam ? Number(roomIdParam) : null;
  const roomIdNum =
    parsedRoomId !== null && Number.isFinite(parsedRoomId) && parsedRoomId > 0
      ? parsedRoomId
      : null;

  const sendMessage = useWsSendMessage();

  const user = useAuthStore((s) => s.user);
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
  const matchAbandonedData = useMatchStore((s) => s.matchAbandonedData);
  const setMatchAbandonedData = useMatchStore((s) => s.setMatchAbandonedData);
  const activeEmotes = useMatchStore((s) => s.activeEmotes);
  const setActiveEmote = useMatchStore((s) => s.setActiveEmote);
  const pendingAutoPlayedCard = useMatchStore((s) => s.pendingAutoPlayedCard);
  const setPendingAutoPlayedCard = useMatchStore((s) => s.setPendingAutoPlayedCard);
  const pendingResolvedTrick = useMatchStore((s) => s.pendingResolvedTrick);
  const setPendingResolvedTrick = useMatchStore((s) => s.setPendingResolvedTrick);

  // "Game is starting…" splash gate — holds the themed loading screen for a
  // deliberate minimum duration when arriving from RoomPage (or LobbyPage
  // quick-play auto-start). The triggering navigation passes
  // `state: { fromRoom: true }`; mounts without that flag (page reload, WS
  // reconnect, deep-link) skip the artificial hold so reload-to-recover stays
  // snappy. Reduced-motion users get a shorter beat. Duration is captured at
  // mount — mid-splash OS motion-preference flips do NOT reset the timer.
  const location = useLocation();
  const cameFromRoom = (location.state as { fromRoom?: boolean } | null)?.fromRoom === true;
  const prefersReducedMotion = useReducedMotion();
  // Phone layout: compact seats (vertical, smaller avatars, name-only) and a
  // tighter center trick area. Matches Tailwind's `md` breakpoint (768px).
  const isCompactTable = useMediaQuery("(max-width: 767px)");
  const [splashElapsed, setSplashElapsed] = useState(!cameFromRoom);
  useEffect(() => {
    if (!cameFromRoom) return;
    const duration = prefersReducedMotion
      ? MOTION.GAME_STARTING_SPLASH_REDUCED
      : MOTION.GAME_STARTING_SPLASH;
    const timer = window.setTimeout(() => setSplashElapsed(true), duration);
    return () => clearTimeout(timer);
    // Intentionally exclude `prefersReducedMotion` from deps — the duration
    // is locked at mount; a mid-splash OS preference flip must not restart
    // the timer (could double the wait the user experiences).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cameFromRoom]);

  const [showReshuffle, setShowReshuffle] = useState(false);
  const [errorToast, setErrorToast] = useState<string | null>(null);
  const errorToastTimerRef = useRef<number | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [rulesOpen, setRulesOpen] = useState(false);
  // Mobile HUD: the bottom action buttons collapse into a top-right hamburger.
  const [hudMenuOpen, setHudMenuOpen] = useState(false);
  // Surrender confirm, driven from the mobile menu (the menu closes on tap, so
  // it can't host the SurrenderButton's own confirm — see SurrenderConfirmDialog).
  const [surrenderConfirmOpen, setSurrenderConfirmOpen] = useState(false);
  // Local card-throw animation: set on play_card dispatch, cleared shortly
  // after so HandCards hides the played card via visibility:hidden while the
  // CardFlight overlay paints the motion. The actual hand removal arrives via
  // the next matchState push, which keys the remaining cards by id so HandCards
  // stays stable across the hand-off.
  const [flyingCardId, setFlyingCardId] = useState<string | null>(null);
  const flyingClearTimerRef = useRef<number | null>(null);

  // CardFlight overlay state — viewport-fixed flying cards that handle all
  // throw and collect motion in one continuous element per card. While a
  // cardId is in flight, both HandCards (source) and TrickArea (slot) hide
  // their static rendering of that card so the overlay is the only painter.
  const [activeFlights, setActiveFlights] = useState<CardFlightDescriptor[]>([]);
  const flightingCardIds = useMemo(
    () => new Set(activeFlights.map((f) => `${f.card.rank}${f.card.suit}`)),
    [activeFlights],
  );
  // Tracks the last currentTrick length we processed so opponent-throw flights
  // fire exactly once per growth event (and not on, e.g., a reconnect that
  // shrinks the trick).
  const prevTrickLenRef = useRef(0);

  // When the server auto-plays a card for the local player (per-move timer
  // expired), the dispatcher writes pendingAutoPlayedCard. Mirror it into
  // flyingCardId so HandCards animates the throw the same way it does for a
  // manual click — without this, the card just vanishes from the hand.
  useEffect(() => {
    if (!pendingAutoPlayedCard) return;
    setFlyingCardId(pendingAutoPlayedCard.cardId);
    if (flyingClearTimerRef.current !== null) {
      clearTimeout(flyingClearTimerRef.current);
    }
    flyingClearTimerRef.current = window.setTimeout(() => {
      setFlyingCardId(null);
      flyingClearTimerRef.current = null;
    }, FLAG_LIFETIME.FLYING_CARD);
    // Auto-played throw flight: measure the hand card + south slot rects and
    // push a self-throw flight, mirroring handlePlayCard's overlay wiring.
    // Defer rect reads via rAF so React's commit + the browser's layout pass
    // settle before we measure (the hand card may be in a freshly-committed
    // render).
    if (myPlayerSeat !== null && !prefersReducedMotion) {
      const cardId = pendingAutoPlayedCard.cardId;
      const receivedAt = pendingAutoPlayedCard.receivedAt;
      const rafId = requestAnimationFrame(() => {
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
      });
      setPendingAutoPlayedCard(null);
      return () => cancelAnimationFrame(rafId);
    }
    setPendingAutoPlayedCard(null);
  }, [pendingAutoPlayedCard, setPendingAutoPlayedCard, myPlayerSeat, prefersReducedMotion]);

  // Opponent throw — when currentTrick grows by one card and the newest card
  // was played by a non-local seat, push a CardFlight from that seat's deck
  // stack to their compass slot. The legacy trickLand keyframe in TrickArea
  // is gone, so without this the opponent's card would simply pop into the
  // slot with no flight.
  const currentTrick = matchState?.currentTrick;
  useEffect(() => {
    const trick = currentTrick ?? [];
    const prev = prevTrickLenRef.current;
    if (prefersReducedMotion) {
      prevTrickLenRef.current = trick.length;
      return;
    }
    if (myPlayerSeat === null) {
      prevTrickLenRef.current = trick.length;
      return;
    }
    let rafId: number | null = null;
    if (trick.length > prev && trick.length > 0) {
      const newest = trick[trick.length - 1];
      if (newest && newest.playerSeat !== myPlayerSeat) {
        const compass = compassOffset(newest.playerSeat, myPlayerSeat);
        const card = newest.card;
        const playerSeat = newest.playerSeat;
        // rAF-defer the rect reads: the new currentTrick state was just
        // committed; we measure after the layout pass that the commit
        // schedules so the trick-slot anchor reflects the post-commit DOM.
        rafId = requestAnimationFrame(() => {
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
        });
      }
    }
    prevTrickLenRef.current = trick.length;
    return () => {
      if (rafId !== null) cancelAnimationFrame(rafId);
    };
  }, [currentTrick, myPlayerSeat, prefersReducedMotion]);

  // Trick-collect orchestration. When pendingResolvedTrick is captured by
  // the WS dispatcher, hold it for TRICK_RESOLVE_PAUSE so the winner glow
  // reads, then measure the four slot rects + the winner destination rect
  // and push four collect flights. Reduced-motion users skip the flights and
  // see the snapshot vanish after a short hold (preserving the glow).
  useEffect(() => {
    if (!pendingResolvedTrick) return;
    if (myPlayerSeat === null) return;

    if (prefersReducedMotion) {
      // Hold the snapshot long enough for the user to register the winner
      // glow — match the full-motion glow phase so reduced-motion users
      // don't get a sub-second flash they can't process. No flights run.
      const reducedTimer = window.setTimeout(() => {
        setPendingResolvedTrick(null);
      }, MOTION.TRICK_RESOLVE_PAUSE);
      return () => clearTimeout(reducedTimer);
    }

    // Fallback safety clear: if the overlay's `animationend` doesn't fire
    // (component unmount / WS reconnect mid-collect / browser timer
    // throttling) the snapshot would otherwise stick and superimpose the
    // resolved trick over the next hand. Schedule a guaranteed clear
    // slightly past the natural collect animation end. handleFlightComplete
    // clears via the same setter on the happy path, and clearing twice is
    // a no-op.
    const FALLBACK_CLEAR_MS = MOTION.TRICK_RESOLVE_PAUSE + MOTION.CARD_FLIGHT_COLLECT + 400;
    const fallbackClearTimer = window.setTimeout(() => {
      setPendingResolvedTrick(null);
    }, FALLBACK_CLEAR_MS);

    const winner = pendingResolvedTrick.winnerSeat;
    const snapshot = pendingResolvedTrick.trick;
    const receivedAt = pendingResolvedTrick.receivedAt;
    // Distinguish back-to-back tricks that contain the same card id: the
    // trickNumber from the live state at capture time is a stable, unique
    // suffix (server increments it on each resolve).
    const trickKey = matchState?.trickNumber ?? 0;

    const glowTimer = window.setTimeout(() => {
      // Winner destination — compass-anchored to the viewport, no DOM read.
      // (See `winnerCollectRect` for the rationale.)
      const destRect = winnerCollectRect(winner, myPlayerSeat);

      const newFlights: CardFlightDescriptor[] = [];
      for (const tc of snapshot) {
        const compass = compassOffset(tc.playerSeat, myPlayerSeat);
        const fromRect = measureTrickSlotRect(compass);
        if (!fromRect) continue;
        const cardId = `${tc.card.rank}${tc.card.suit}`;
        newFlights.push({
          id: `collect-${trickKey}-${cardId}-${receivedAt}`,
          card: tc.card,
          fromRect,
          toRect: destRect,
          durationMs: MOTION.CARD_FLIGHT_COLLECT,
          // Cards fade as they reach the winner's pile.
          endOpacity: 0,
        });
      }

      if (newFlights.length === 0) {
        setPendingResolvedTrick(null);
        return;
      }
      setActiveFlights((prev) => [...prev, ...newFlights]);
    }, MOTION.TRICK_RESOLVE_PAUSE);

    return () => {
      clearTimeout(glowTimer);
      clearTimeout(fallbackClearTimer);
    };
  }, [
    pendingResolvedTrick,
    myPlayerSeat,
    prefersReducedMotion,
    setPendingResolvedTrick,
    matchState?.trickNumber,
  ]);

  // Flight completion — remove the flight; if it was the last collect flight,
  // also clear pendingResolvedTrick so the snapshot disappears in the same
  // render as the last flight unmounts (no slot-card flicker). React 18+
  // automatically batches the two state setters when both run inside the
  // same handler, so we don't need queueMicrotask trickery.
  const handleFlightComplete = useCallback(
    (flightId: string) => {
      let clearSnapshot = false;
      setActiveFlights((prev) => {
        const next = prev.filter((f) => f.id !== flightId);
        const wasCollect = flightId.startsWith("collect-");
        const stillCollecting = next.some((f) => f.id.startsWith("collect-"));
        if (wasCollect && !stillCollecting) clearSnapshot = true;
        return next;
      });
      if (clearSnapshot) setPendingResolvedTrick(null);
    },
    [setPendingResolvedTrick],
  );

  // ScoreReveal needs the trump suit / caller seat AND the hand number from
  // the just-finished hand (for its title + contract-held subtitle). The
  // server pushes the next-hand matchState (trumpSuit=null, handNumber already
  // incremented) close to the hand_scored payload, so by the time the reveal
  // mounts those live fields describe the NEXT hand — which is why the title
  // showed "Hand 2 — results" right after hand 1 finished. Snapshot them
  // whenever a hand is live (trumpSuit set); the snapshot survives the
  // next-hand reset and feeds ScoreReveal the finished hand's values.
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

  // Overlay flow state: normal → capot_animation → score_reveal → normal/match_result
  type OverlayPhase = "normal" | "capot_animation" | "score_reveal" | "match_result";
  const [overlayPhase, setOverlayPhase] = useState<OverlayPhase>("normal");

  // Chat sidebar open/closed — lifted here so the rules/settings cluster can
  // hide the emote button while the chat sidebar is occupying the right rail.
  const [isChatOpen, setIsChatOpen] = useState(false);

  // Match chat history is tied to the MatchPage lifecycle — clear it on
  // unmount (navigation away, abandonment, match-end → lobby) so the next
  // match starts with an empty panel. Ephemeral by design (AC #6).
  useEffect(() => {
    return () => {
      useChatStore.getState().clearMatch();
    };
  }, []);

  // Trigger overlay flow when score reveal data arrives
  useEffect(() => {
    if (scoreRevealData !== null && overlayPhase === "normal") {
      if (scoreRevealData.capot) {
        setOverlayPhase("capot_animation");
      } else {
        setOverlayPhase("score_reveal");
      }
    }
  }, [scoreRevealData, overlayPhase]);

  // Track previous phase to detect bidding→dealing transition (reshuffle)
  const prevPhaseRef = useRef<string | null>(null);

  // Derive myPlayerSeat on first game state
  useEffect(() => {
    if (matchState && user && myPlayerSeat === null) {
      const myPlayer = matchState.players.find((p) => p.userId === user.id);
      if (myPlayer !== undefined) {
        setMyPlayerSeat(myPlayer.seat);
      }
    }
  }, [matchState, user, myPlayerSeat, setMyPlayerSeat]);

  // Redirect to lobby on stale match_end state (e.g. page refresh after abandonment
  // when matchAbandonedData/matchEndData are lost from in-memory store)
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

  // Splash-state classifier — when the user lands on /game/:id we need to
  // distinguish between (a) they belong here and we're just waiting for the
  // WS push, (b) the room exists but the match already finished, (c) the
  // room exists but they were never seated in it, (d) the room id doesn't
  // resolve at all. Without this guard, deep-linking to a stale/abandoned
  // room left the splash showing "Reconnecting to the game…" forever.
  //
  // Source priority:
  //   1. HTTP GET /api/rooms/:id — authoritative source of room status +
  //      seat-membership. Fires on mount.
  //   2. WS-fallback — if the HTTP check passes but the server still doesn't
  //      push match_state within a short window (e.g. session torn down after
  //      the room status was checked), classify as "noActiveSession" too.
  type SplashIssue = "noActiveSession" | "notMember" | "invalid";
  const wsConnectionState = useWsConnectionState();
  const [splashIssue, setSplashIssue] = useState<SplashIssue | null>(null);

  useEffect(() => {
    if (!roomIdNum || !user) return;
    let cancelled = false;
    getRoom(roomIdNum)
      .then((detail) => {
        if (cancelled) return;
        const isPlayer = detail.players.some((p) => p.userId === user.id);
        if (!isPlayer) {
          setSplashIssue("notMember");
          return;
        }
        // Server room status is one of "waiting" | "in_progress" | "completed"
        // | "cancelled". Only "waiting" / "in_progress" expect a live session;
        // anything else means the match wrapped up and the session is gone.
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
      // Game state arrived after we declared an issue (rare race: HTTP check
      // returned "completed" but a fresh session started a moment later).
      // Cancel the redirect so we don't yank the user out of a real game.
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

  // Transition to match result after score reveal is dismissed (if match ended)
  useEffect(() => {
    if (matchEndData !== null && overlayPhase === "normal") {
      setOverlayPhase("match_result");
    }
  }, [matchEndData, overlayPhase]);

  // Detect reshuffle: bidding → dealing transition within same match
  const currentPhase = matchState?.phase;
  useEffect(() => {
    if (currentPhase) {
      if (prevPhaseRef.current === "bidding" && currentPhase === "dealing") {
        setShowReshuffle(true);
      }
      prevPhaseRef.current = currentPhase;
    }
  }, [currentPhase]);

  // Track whether the most recent client action was a surrender so the next
  // ErrInvalidAction / ErrWrongPhase rejection can route to the
  // surrender-specific i18n strings instead of the generic ones.
  const surrenderActionInFlightRef = useRef(false);

  // Error toast display — uses same mapping as useWsDispatch error routing.
  // Timer is tracked via ref (not effect cleanup) so the 3 s auto-dismiss isn't
  // cancelled by the re-run triggered by setLastError(null).
  useEffect(() => {
    if (!lastError) return;
    const ERROR_I18N: Record<string, string> = {
      "error:wrong_phase": "match.errors.wrongPhase",
      "error:not_your_turn": "match.errors.notYourTurn",
      "error:invalid_action": "match.errors.invalidAction",
      "error:illegal_play": "match.errors.illegalPlay",
      "error:pause_exhausted": "match.errors.pauseExhausted",
      "error:no_active_pause": "match.errors.noActivePause",
      "error:not_room_owner": "match.errors.notRoomOwner",
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

  // Browser back-button interception — push sentinel entry only once
  const historyPushedRef = useRef(false);
  useEffect(() => {
    if (!matchState) return;

    const handlePopState = () => {
      const leave = window.confirm(t("match.leaveConfirm"));
      if (leave) {
        clearGame();
        navigate("/lobby");
      } else {
        window.history.pushState(null, "", window.location.href);
      }
    };

    if (!historyPushedRef.current) {
      window.history.pushState(null, "", window.location.href);
      historyPushedRef.current = true;
    }
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, [matchState, clearGame, navigate, t]);

  // --- Action handlers ---
  const handlePlayCard = useCallback(
    (cardId: string) => {
      // Measure the source (hand card) and destination (south trick slot)
      // BEFORE we hide the card via flyingCardId. Read the hand from the
      // live store (not the closed-over `matchState`) so a concurrent WS
      // update between render and click can't leave us with a stale lookup.
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
            // Self throw arcs through viewport bottom-center so the card
            // visibly travels down to the screen lip and back up to the
            // table — matches the motion the user described.
            waypointRect: viewportBottomCenterRect(),
            durationMs: MOTION.CARD_FLIGHT_SELF_THROW,
          };
        }
      }

      // Trigger the hand-throw flight immediately so the gesture starts
      // before the WS round-trip completes. The card is cleared from the
      // local hand by the next matchState push (server removes it from
      // `players[seat].hand`); flyingCardId controls the source-side
      // visibility:hidden so the overlay's flying card is the only painter.
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
      sendMessage(ACTION_PLAY_CARD, { cardId });
    },
    [sendMessage, prefersReducedMotion, myPlayerSeat],
  );

  const handlePickTrump = useCallback(
    (suit?: Suit) => {
      sendMessage(ACTION_PICK_TRUMP, suit ? { suit } : {});
    },
    [sendMessage],
  );

  const handlePassTrump = useCallback(() => {
    sendMessage(ACTION_PASS_TRUMP, {});
  }, [sendMessage]);

  const handleDeclare = useCallback(() => {
    sendMessage(ACTION_DECLARE, {});
  }, [sendMessage]);

  const handleSkipDeclare = useCallback(() => {
    sendMessage(ACTION_SKIP_DECLARE, {});
  }, [sendMessage]);

  const handleAnnounceBelot = useCallback(() => {
    sendMessage(ACTION_ANNOUNCE_BELOT, {});
  }, [sendMessage]);

  const handleDeclineBelot = useCallback(() => {
    sendMessage(ACTION_DECLINE_BELOT, {});
  }, [sendMessage]);

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

  const handleReshuffleComplete = useCallback(() => {
    setShowReshuffle(false);
  }, []);

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
    setScoreRevealData(null);
    if (matchEndData !== null) {
      setOverlayPhase("match_result");
    } else {
      setOverlayPhase("normal");
    }
  }, [matchEndData, setScoreRevealData]);

  const handleReturnToLobby = useCallback(() => {
    setMatchEndData(null);
    clearGame();
    navigate("/lobby");
  }, [clearGame, navigate, setMatchEndData]);

  const handleAbandonReturnToLobby = useCallback(() => {
    setMatchAbandonedData(null);
    clearGame();
    navigate("/lobby");
  }, [clearGame, navigate, setMatchAbandonedData]);

  // Loading state — themed with the in-game felt + brass palette so the
  // transition into the table doesn't flash a generic dark splash. We can't
  // render the full TableBackdrop (no matchState yet), so this is a slimmed-
  // down felt gradient + brass spinner using the same tokens. Two copy paths:
  //   • From room lobby (`cameFromRoom`): "Game is starting…", held for
  //     `MOTION.GAME_STARTING_SPLASH` even if state is already in hand — the
  //     deliberate beat masks the room→game transition so dealing / trump
  //     prompt don't appear in the same instant the page enters.
  //   • Reload / WS reconnect / deep-link: "Reconnecting to the game…",
  //     shown only until state arrives, with no artificial floor.
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
              border: "2px solid rgba(201,168,118,0.18)",
              borderTopColor: "var(--brass, #c9a876)",
              animationDuration: "0.9s",
            }}
          />
          <span
            className="font-body text-base"
            style={{
              color: "var(--ink-light, #f5f2e8)",
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

  // Viewer team derivation — single rule, frontend only. Components rendered
  // below this guard always have a non-null myPlayerSeat, so viewerTeam is
  // always a real TeamString when we're inside an active match.
  const viewerTeam: TeamString = myPlayerSeat % 2 === 0 ? "teamA" : "teamB";

  // Pause state
  const isRoomOwner = myPlayerSeat !== null && matchState.ownerSeat === myPlayerSeat;
  const isPaused = matchState.phase === "paused";

  // Single source of truth for "is an overlay covering the table". Used to
  // gate reveals (D112) and the dealer/trump pill (dealer-indicator D97).
  const isOverlayActive =
    isPaused ||
    matchEndData !== null ||
    matchAbandonedData !== null ||
    matchState.phase === "disconnected";
  const canPause =
    !isPaused &&
    (matchState.phase === "playing" || matchState.phase === "bidding") &&
    myPlayerSeat !== null &&
    !matchState.pauseUsed?.[myPlayerSeat];

  // Surrender state (Story 8.2)
  const surrenderProposerSeat = matchState.surrenderProposerSeat;
  const isProposer = surrenderProposerSeat !== null && surrenderProposerSeat === myPlayerSeat;
  const isPartnerOfProposer =
    surrenderProposerSeat !== null &&
    myPlayerSeat !== null &&
    (surrenderProposerSeat + 2) % 4 === myPlayerSeat;
  const isOpponentOfProposer =
    surrenderProposerSeat !== null && myPlayerSeat !== null && !isProposer && !isPartnerOfProposer;
  const showSurrenderControls =
    myPlayerSeat !== null && (matchState.phase === "playing" || matchState.phase === "bidding");
  const canSurrenderRequest =
    showSurrenderControls &&
    surrenderProposerSeat === null &&
    myPlayerSeat !== null &&
    !matchState.surrenderUsed?.[myPlayerSeat];
  const proposerPlayer =
    surrenderProposerSeat !== null
      ? matchState.players.find((p) => p.seat === surrenderProposerSeat)
      : undefined;
  const proposerUsername = proposerPlayer?.username ?? t("match.surrender.unknownProposer");

  const surrenderedByUsername =
    matchEndData?.outcomeReason === "surrender" &&
    typeof matchEndData.surrenderedBySeat === "number"
      ? matchState.players.find((p) => p.seat === matchEndData.surrenderedBySeat)?.username
      : undefined;

  // Compute playable card IDs — block during prompts and pause
  const isMyTurn =
    matchState.activePlayerSeat === myPlayerSeat &&
    matchState.phase === "playing" &&
    !matchState.awaitingDeclaration &&
    matchState.pendingBelotSeat !== myPlayerSeat;
  const myPlayer = matchState.players.find((p) => p.seat === myPlayerSeat);
  const myHand = myPlayer?.hand ?? [];
  const playableCardIds =
    isMyTurn && myPlayerSeat !== null ? legalCardIds(matchState, myPlayerSeat) : [];

  // Bidding state
  const isBiddingPhase = matchState.phase === "bidding";
  const isActiveBidder = isBiddingPhase && matchState.activePlayerSeat === myPlayerSeat;

  // Declaration state
  const showDeclarationPrompt =
    matchState.awaitingDeclaration === true && matchState.activePlayerSeat === myPlayerSeat;

  // Belot state
  const showBelotPrompt = matchState.pendingBelotSeat === myPlayerSeat;
  // The triggering K/Q is the last card of the current trick at prompt time.
  const belotPromptLastTrickCard = showBelotPrompt
    ? (matchState.currentTrick[matchState.currentTrick.length - 1] ?? null)
    : null;
  const belotPromptIsKing = belotPromptLastTrickCard?.card.rank === "K";

  // Deal animation state
  const isDealingPhase = matchState.phase === "dealing";

  return (
    <div
      className="game-table h-screen w-screen overflow-hidden relative bg-background"
      data-testid="match-page"
    >
      {/* Static table chrome — felt + wood rim + brass oval + filigree */}
      <TableBackdrop compact={isCompactTable} />
      {/* Floating particles + soft top beam (decorative, reduced-motion aware) */}
      <TableAmbience intensity={0.9} tint="#ffe9b0" />
      {/* "Beljot.online" wordmark — top center */}
      <Wordmark />

      {/* Score panel - top left */}
      <ScorePanel
        viewerTeam={viewerTeam}
        teamAScore={matchState.teamScores[0]}
        teamBScore={matchState.teamScores[1]}
        teamATricks={matchState.tricksWon[0]}
        teamBTricks={matchState.tricksWon[1]}
        teamAHandPotential={matchState.handPoints[0] + matchState.declarationPoints[0]}
        teamBHandPotential={matchState.handPoints[1] + matchState.declarationPoints[1]}
        lastTrickBonus={scoreRevealData?.lastTrickBonus}
        lastTrickTeam={scoreRevealData?.lastTrickTeam}
        handNumber={matchState.handNumber}
        variantLabel={t(`match.variants.${matchState.variant}`, {
          defaultValue: matchState.variant,
        })}
      />

      {/* Trump indicator - top right. Gated to play phases (AC 4.4.5) and
          hidden behind any active overlay. The dealer is now indicated by a
          chip on the dealer's avatar (Stage 2), so the standalone dealer pill
          is no longer rendered here. */}
      {!isOverlayActive &&
        matchState.trumpSuit &&
        matchState.phase !== "dealing" &&
        matchState.phase !== "bidding" && (
          <div className="absolute top-4 right-4 z-10 hidden md:block">
            <TrumpIndicator
              trumpSuit={matchState.trumpSuit}
              trumpCallerSeat={matchState.trumpCallerSeat}
              trumpCallerName={
                matchState.trumpCallerSeat !== null
                  ? (matchState.players.find((p) => p.seat === matchState.trumpCallerSeat)
                      ?.username ?? null)
                  : null
              }
              viewerTeam={viewerTeam}
            />
          </div>
        )}

      {/* Player seats at compass positions */}
      {matchState.players.map((player) => {
        const compass = compassOffset(player.seat, myPlayerSeat);
        const isSelf = player.seat === myPlayerSeat;
        const isActive = matchState.activePlayerSeat === player.seat;
        // Caller chip only shows the suit when this seat IS the trump caller.
        const isCaller =
          matchState.trumpCallerSeat !== null &&
          matchState.trumpCallerSeat === player.seat &&
          matchState.trumpSuit !== null;

        return (
          <div
            key={player.seat}
            className={`absolute ${SEAT_POSITIONS[compass]}`}
            data-testid={`player-seat-${compass}-wrapper`}
          >
            <PlayerSeat
              player={player}
              isSelf={isSelf}
              isActive={isActive}
              seatTeam={seatTeam(player.seat, myPlayerSeat)}
              cardCount={isSelf ? undefined : player.hand.length}
              turnExpiresAt={
                isActive && (matchState.phase === "playing" || matchState.phase === "bidding")
                  ? matchState.turnExpiresAt
                  : null
              }
              timerDuration={matchState.timerDurationSec}
              isDealer={matchState.dealerSeat === player.seat}
              trumpCallerSuit={isCaller ? matchState.trumpSuit : null}
              orientation={SEAT_ORIENTATIONS[compass]}
              compact={isCompactTable}
            />
          </div>
        );
      })}

      {/* Trick area - center */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2">
        <TrickArea
          trick={matchState.currentTrick}
          winnerSeat={matchState.trickWinnerSeat}
          myPlayerSeat={myPlayerSeat}
          pendingResolvedTrick={pendingResolvedTrick}
          suppressedCardIds={flightingCardIds}
          compact={isCompactTable}
        />
      </div>

      {/* Hand cards - bottom center. While a personal action prompt is up
          (trump bidding, belot announcement, declaration) the hand is
          elevated to z-50 so it sits between the OverlayBackdrop dim (z-40)
          and the panel itself (z-60) — the player can read their cards
          unblurred while everything else stays dimmed. */}
      <div
        className={`absolute bottom-4 left-1/2 -translate-x-1/2 ${
          isActiveBidder || showBelotPrompt || showDeclarationPrompt ? "z-50" : ""
        }`}
      >
        <HandCards
          hand={myHand}
          isMyTurn={isMyTurn}
          playableCardIds={playableCardIds}
          onPlayCard={handlePlayCard}
          flyingId={flyingCardId}
          compact={isCompactTable}
        />
      </div>

      {/* Pause + surrender controls — bottom-left HUD cluster (desktop only;
          phones fold these into the top-right hamburger menu below). */}
      {(matchState.phase === "playing" || matchState.phase === "bidding") && (
        <div className="absolute bottom-4 left-4 z-10 hidden items-center gap-2 md:flex">
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

          <SurrenderButton
            canRequest={canSurrenderRequest}
            isExhausted={!!matchState.surrenderUsed?.[myPlayerSeat]}
            isPending={surrenderProposerSeat !== null}
            onConfirm={handleSurrenderRequest}
          />
        </div>
      )}

      {/* Rules + settings + emote — bottom-right HUD cluster sitting LEFT of
          the chat FAB. The whole cluster is hidden while the chat dock is open,
          since the floating panel covers this corner; it returns on close. The
          Sound button is intentionally omitted until audio ships. */}
      {!isOverlayActive && !isChatOpen && (
        <div className="absolute bottom-4 right-24 z-10 hidden items-center gap-2 md:flex">
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
          {(matchState.phase === "dealing" ||
            matchState.phase === "bidding" ||
            matchState.phase === "playing") &&
            matchEndData === null &&
            matchAbandonedData === null && <EmotePickerButton onSend={handleSendEmote} />}
        </div>
      )}

      {/* Mobile HUD menu — the bottom action clusters fold into a top-right
          hamburger; only the chat FAB stays at the bottom-right on phones. */}
      {!isOverlayActive && (
        <div className="md:hidden">
          <button
            type="button"
            onClick={() => setHudMenuOpen((v) => !v)}
            aria-label={t("nav.menu")}
            aria-expanded={hudMenuOpen}
            data-testid="hud-menu-button"
            className="absolute top-3 right-3 z-30 inline-flex size-9 items-center justify-center rounded-[10px]"
            style={{
              background: "var(--panel-hud, rgba(18,32,22,0.85))",
              border: "1px solid rgba(201,168,118,0.4)",
              color: "var(--ink-light, #f5f2e8)",
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
              {/* tap-away backdrop */}
              <button
                type="button"
                aria-hidden="true"
                tabIndex={-1}
                onClick={() => setHudMenuOpen(false)}
                className="fixed inset-0 z-20 cursor-default"
              />
              <div
                className="absolute top-14 right-3 z-30 flex w-44 flex-col gap-1.5 rounded-xl p-2"
                style={{
                  background: "var(--panel-deeper, rgba(18,32,22,0.94))",
                  border: "1px solid rgba(201,168,118,0.4)",
                  boxShadow: "0 14px 36px rgba(0,0,0,0.5)",
                  backdropFilter: "blur(10px)",
                  WebkitBackdropFilter: "blur(10px)",
                }}
                data-testid="hud-menu"
              >
                {(matchState.phase === "playing" || matchState.phase === "bidding") && (
                  <>
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
                {(matchState.phase === "dealing" ||
                  matchState.phase === "bidding" ||
                  matchState.phase === "playing") &&
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

      {/* Surrender confirm (mobile HUD menu path). Owned here, not by the menu,
          so closing the menu doesn't unmount the modal. Auto-dismisses if
          surrender stops being available (partner proposed / attempt spent). */}
      {surrenderConfirmOpen && canSurrenderRequest && (
        <SurrenderConfirmDialog
          onCancel={() => setSurrenderConfirmOpen(false)}
          onConfirm={() => {
            setSurrenderConfirmOpen(false);
            handleSurrenderRequest();
          }}
        />
      )}

      {/* Surrender prompt — partner-only, modal. Suppress while a match-end
          / match-abandoned overlay is up so the prompt's focus trap doesn't
          swallow Tab navigation to the Return-to-Lobby button. */}
      {isPartnerOfProposer && matchEndData === null && matchAbandonedData === null && (
        <SurrenderPrompt
          proposerUsername={proposerUsername}
          onAccept={handleSurrenderAccept}
          onDecline={handleSurrenderDecline}
        />
      )}

      {/* Surrender opponent banner — non-modal status strip anchored to the
          proposer's seat (same per-seat pattern as EmoteBubble). Same overlay
          gate as the prompt. */}
      {isOpponentOfProposer &&
        myPlayerSeat !== null &&
        surrenderProposerSeat !== null &&
        matchEndData === null &&
        matchAbandonedData === null && (
          <SurrenderOpponentBanner
            proposerUsername={proposerUsername}
            compassPosition={compassOffset(surrenderProposerSeat, myPlayerSeat) as 0 | 1 | 2 | 3}
          />
        )}

      {/* Pause overlay */}
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

      {/* Reconnect overlay — shown during disconnect countdown OR abandonment.
          `disconnectedPlayers` carries each offline seat's individual
          reconnect expiry so the per-row countdowns can run independently of
          the center ring. The center ring counts down to whichever seat
          closes soonest (the same value the server publishes via
          `matchState.reconnectExpiresAt`). */}
      {((matchState.phase === "disconnected" &&
        matchState.disconnectedSeat !== -1 &&
        matchState.reconnectExpiresAt) ||
        matchAbandonedData) && (
        <ReconnectOverlay
          disconnectedPlayerName={
            matchAbandonedData
              ? (matchState.players[matchAbandonedData.abandonedByPlayer]?.username ??
                `Player ${matchAbandonedData.abandonedByPlayer + 1}`)
              : (matchState.players[matchState.disconnectedSeat]?.username ??
                `Player ${matchState.disconnectedSeat + 1}`)
          }
          disconnectedPlayers={matchState.players
            .map((p) => {
              const expiry = matchState.playerReconnectExpiresAt[p.seat];
              if (p.connected || !expiry) return null;
              return {
                name: p.username || `Player ${p.seat + 1}`,
                expiresAt: expiry,
              };
            })
            .filter((entry): entry is { name: string; expiresAt: string } => entry !== null)}
          reconnectExpiresAt={matchState.reconnectExpiresAt ?? ""}
          abandonedData={matchAbandonedData}
          viewerTeam={viewerTeam}
          onReturnToLobby={handleAbandonReturnToLobby}
        />
      )}

      {/* Next-hand UI is suppressed while the end-of-hand overlay sequence
          (capot animation → score reveal → match result) owns the screen.
          Without this gate the new hand's deal animation + first-bidder
          trump prompt paint behind the score reveal because both layers
          live at z-50; their order in the DOM decides the painted result.
          Holding them off until overlayPhase === "normal" makes the
          end-of-hand summary unambiguously the front overlay, then the
          new-hand prompts surface in sequence as the player dismisses it. */}
      {overlayPhase === "normal" && (
        <>
          {/* Deal animation overlay */}
          {isDealingPhase && <DealAnimation trumpCandidate={matchState.trumpCandidate} />}

          {/* Trump bidding prompt overlay */}
          {isBiddingPhase && (
            <TrumpPrompt
              trumpCandidate={matchState.trumpCandidate}
              biddingRound={matchState.biddingRound}
              isActiveBidder={isActiveBidder}
              activePlayerName={
                matchState.players.find((p) => p.seat === matchState.activePlayerSeat)?.username ??
                null
              }
              onPick={handlePickTrump}
              onPass={handlePassTrump}
              turnExpiresAt={matchState.turnExpiresAt}
              timerDurationSec={matchState.timerDurationSec}
            />
          )}
        </>
      )}

      {/* Reshuffle animation overlay */}
      {showReshuffle && <ReshuffleAnimation onComplete={handleReshuffleComplete} />}

      {/* Declaration prompt overlay */}
      {showDeclarationPrompt && myPlayer && (
        <DeclarationPrompt
          declarations={
            myPlayer.declarations.length > 0
              ? myPlayer.declarations
              : detectDeclarations(myPlayer.hand)
          }
          onDeclare={handleDeclare}
          onSkip={handleSkipDeclare}
          turnExpiresAt={matchState.turnExpiresAt}
          timerDurationSec={matchState.timerDurationSec}
        />
      )}

      {/* Belot prompt overlay */}
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

      {/* Declaration resolution reveal — silently consumed while an overlay
          covers the table (D112). The reveal's internal setTimeout still ticks
          via useEffect; gating is render-only by design. */}
      {declarationReveal && !isOverlayActive && (
        <DeclarationReveal
          payload={declarationReveal}
          players={matchState.players}
          viewerTeam={viewerTeam}
          onComplete={handleDeclarationRevealComplete}
        />
      )}

      {/* Belot / Re-belot reveal — same overlay-active gate as the declaration
          reveal. Keyed on payload so back-to-back reveals remount cleanly. */}
      {belotReveal && !isOverlayActive && (
        <BelotReveal
          key={`${belotReveal.playerSeat}-${belotReveal.cardId}`}
          playerSeat={belotReveal.playerSeat}
          myPlayerSeat={myPlayerSeat}
          cardId={belotReveal.cardId}
          isKing={belotReveal.cardId.startsWith("K")}
          players={matchState.players}
          onComplete={handleBelotRevealComplete}
        />
      )}

      {/* Trump-take reveal — center-of-table announcement toast that glows
          in the caller's viewer-relative team color (Gold = Us, Silver = Them)
          and auto-closes after 8 s with an X-with-countdown-ring escape. */}
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

      {/* Match chat dock — bottom-right floating window broadcasting to the 4 participants */}
      <MatchChatDock isOpen={isChatOpen} onOpenChange={setIsChatOpen} />

      {/* Emote bubbles — one per seat that has an active emote. Suppressed
          when an overlay or pause owns the screen; the store still records
          the latest emote so that re-emergence renders the next live one. */}
      {matchEndData === null &&
        matchAbandonedData === null &&
        matchState.phase !== "paused" &&
        matchState.phase !== "disconnected" &&
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

      {/* Capot animation overlay */}
      {overlayPhase === "capot_animation" &&
        scoreRevealData?.capotTeam !== null &&
        scoreRevealData?.capotTeam !== undefined && (
          <CapotAnimation
            capotTeam={scoreRevealData.capotTeam}
            viewerSeat={myPlayerSeat}
            capotBonus={scoreRevealData.capotBonus}
            onComplete={handleCapotComplete}
          />
        )}

      {/* Score reveal overlay */}
      {overlayPhase === "score_reveal" && scoreRevealData !== null && (
        <ScoreReveal
          data={scoreRevealData}
          viewerTeam={viewerTeam}
          onContinue={handleScoreRevealContinue}
          handNumber={lastTrumpRef.current.handNumber ?? matchState.handNumber}
          trumpSuit={matchState.trumpSuit ?? lastTrumpRef.current.suit}
          trumpCallerSeat={matchState.trumpCallerSeat ?? lastTrumpRef.current.callerSeat}
        />
      )}

      {/* Match result overlay */}
      {overlayPhase === "match_result" && matchEndData !== null && (
        <MatchResult
          data={matchEndData}
          viewerTeam={viewerTeam}
          onReturnToLobby={handleReturnToLobby}
          surrenderedByUsername={surrenderedByUsername}
        />
      )}

      {/* CardFlight overlay — viewport-fixed layer that paints all in-flight
          cards (self throw, opponent throw, take-collect). Mounted last so
          its z-index sits above the table and any other game chrome but
          below modal dialogs / score reveal (which set their own z-index). */}
      <CardFlight flights={activeFlights} onComplete={handleFlightComplete} />

      {/* Settings + rules dialogs — driven by the bottom-right HUD buttons */}
      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
      <RulesDialog open={rulesOpen} onOpenChange={setRulesOpen} />

      {/* Error toast */}
      {errorToast && (
        <div
          className="absolute bottom-20 left-1/2 -translate-x-1/2 z-40 bg-destructive/90 text-text-primary font-body text-sm px-4 py-2 rounded-lg flex items-center gap-3"
          role="alert"
          data-testid="error-toast"
        >
          <span>{errorToast}</span>
          <button
            type="button"
            onClick={dismissErrorToast}
            aria-label={t("common.close")}
            data-testid="error-toast-close"
            className="text-text-primary/80 hover:text-text-primary text-lg leading-none px-1 focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1 focus-visible:ring-offset-background rounded"
          >
            ×
          </button>
        </div>
      )}
    </div>
  );
}
