import {
  ChevronDown,
  Clock,
  Crown,
  Shuffle,
  Sparkles,
  Trophy,
  Users,
  UserX,
  Zap,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";

import { RoomChatDock } from "@/features/room/components/RoomChatDock";
import { SeatTile } from "@/features/room/components/SeatTile";
import { KickPlayerDialog, TransferOwnershipDialog } from "@/features/room/OwnerConfirmDialogs";
import { FetchError } from "@/shared/api/axiosClient";
import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import { Button } from "@/shared/components/ui/button";
import { CodeChip } from "@/shared/components/ui/code-chip";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/shared/components/ui/dropdown-menu";
import {
  useJoinRoomMutation,
  useKickPlayerMutation,
  useLeaveRoomMutation,
  useLeaveSeatMutation,
  useSelectSeatMutation,
  useStartMatchMutation,
  useSwapSeatsMutation,
  useTransferOwnershipMutation,
} from "@/shared/hooks/mutations/useRooms";
import { useRoomDetailQuery } from "@/shared/hooks/queries/useRooms";
import { cn } from "@/shared/lib/utils";
import { useWsConnectionState } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { RoomPlayer } from "@/shared/types/apiTypes";

const variantKeys: Record<string, string> = {
  bitola: "lobby.card.variantBitola",
};

const matchModeKeys: Record<string, string> = {
  "1001": "lobby.card.matchMode1001",
  "501": "lobby.card.matchMode501",
};

// Cardinal positions for the diamond seat layout. In the waiting room seat
// indices map 1:1 to fixed cardinal positions (no viewer-relative rotation).
// This is intentionally different from the in-game table where the viewer
// rotates to the bottom: in the lobby the player has no hand to anchor, and
// rotating after a click made seat selection feel unpredictable ("I clicked
// the top tile but landed at the bottom"). Fixed positions match the click
// target.
type CardinalPosition = "south" | "east" | "north" | "west";

const SEAT_INDEXES = [0, 1, 2, 3] as const;

const SEAT_TO_CARDINAL: Record<number, CardinalPosition> = {
  0: "south",
  1: "east",
  2: "north",
  3: "west",
};

function getPlayerAtSeat(players: RoomPlayer[], seatIndex: number): RoomPlayer | undefined {
  return players.find((p) => p.seat === seatIndex);
}

// Team-legend indicator — a small rounded square that mirrors the seat tiles:
// neutral = dashed cream border + blinking brass dot (viewer unseated); us/them
// = solid gold/silver border + a static team dot (viewer seated).
function LegendIndicator({ variant }: { variant: "neutral" | "us" | "them" }) {
  const tokens = {
    neutral: { border: "var(--border-2)", dot: "var(--brass)", dashed: true, pulse: true },
    us: { border: "var(--team-a)", dot: "var(--team-a-fill)", dashed: false, pulse: false },
    them: { border: "var(--team-b)", dot: "var(--team-b-fill)", dashed: false, pulse: false },
  }[variant];
  return (
    <span
      aria-hidden
      className="inline-flex h-4.5 w-7 items-center justify-center rounded-[5px]"
      style={{ border: `1.5px ${tokens.dashed ? "dashed" : "solid"} ${tokens.border}` }}
    >
      <span
        className={cn(
          "size-2 rounded-full",
          tokens.pulse && "animate-[pulse-dot_1.6s_ease-in-out_infinite]",
        )}
        style={{ background: tokens.dot }}
      />
    </span>
  );
}

// Leave the room in a way that survives a full document unload (typing a URL in
// the address bar, refresh, tab close). React does NOT run effect cleanups on a
// hard unload, so the unmount-cleanup leave below never fires and an in-flight
// XHR would be aborted — leaving the player a member, which then blocks joining
// any other room (ErrAlreadyInRoom). `keepalive` lets this POST outlive the
// page; auth rides the same Bearer token axios uses (plus cookies, harmless).
function sendLeaveBeacon(roomId: number, token: string) {
  void fetch(`/api/v1/rooms/${roomId}/leave`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    keepalive: true,
    credentials: "include",
  }).catch(() => {});
}

export function RoomPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const navigate = useNavigate();
  const currentUser = useAuthStore((s) => s.user);

  // TanStack Query for initial REST fetch
  const roomQuery = useRoomDetailQuery(id ? Number(id) : undefined);

  // WebSocket connection state — used to refetch after reconnection
  const wsState = useWsConnectionState();
  const prevWsStateRef = useRef(wsState);

  // Zustand store for real-time WS updates (source of truth for rendering)
  const storeRoom = useRoomStore((s) => s.room);
  const storePlayers = useRoomStore((s) => s.players);
  const matchStarted = useRoomStore((s) => s.matchStarted);
  const kickedFromRoomId = useRoomStore((s) => s.kickedFromRoomId);

  const hasLeftRef = useRef(false);
  const hasJoinedRef = useRef(false);
  // Guards the deep-link auto-join below to a single attempt (the query data
  // changes on refetch, and React StrictMode double-invokes effects in dev).
  const joinAttemptedRef = useRef(false);

  // Owner-only transient UI state — kept local instead of pushed into a store
  // because nothing outside RoomPage cares about it.
  const [swapSourceSeat, setSwapSourceSeat] = useState<number | null>(null);
  const [kickConfirm, setKickConfirm] = useState<{
    seat: number;
    userId: number;
    username: string;
  } | null>(null);
  const [transferConfirm, setTransferConfirm] = useState<{
    userId: number;
    username: string;
  } | null>(null);

  // Mutations
  const joinRoomMutation = useJoinRoomMutation();
  const selectSeatMutation = useSelectSeatMutation();
  const startGameMutation = useStartMatchMutation();
  const leaveRoomMutation = useLeaveRoomMutation();
  const kickPlayerMutation = useKickPlayerMutation();
  const swapSeatsMutation = useSwapSeatsMutation();
  const leaveSeatMutation = useLeaveSeatMutation();
  const transferOwnershipMutation = useTransferOwnershipMutation();

  // Seed roomStore from query data
  useEffect(() => {
    if (roomQuery.data) {
      const store = useRoomStore.getState();
      store.setRoom(roomQuery.data.room);
      store.setPlayers(roomQuery.data.players);
      store.setCurrentRoomId(roomQuery.data.room.id);
      const userId = useAuthStore.getState().user?.id;
      if (userId && roomQuery.data.players.some((p) => p.userId === userId)) {
        hasJoinedRef.current = true;
      }
    }
  }, [roomQuery.data]);

  // Deep-link / direct-URL entry: reaching this page without going through the
  // lobby's "Join" means the server has no membership row for us, so seat
  // selection is rejected (ErrNotInRoom) and we receive no room WS broadcasts —
  // the page looks dead, you can only leave. When the fetched room shows we are
  // NOT yet a member of an open (waiting, non-quick-play) room, join it here —
  // mirroring what the lobby's Join does — then refetch to pick up membership.
  useEffect(() => {
    if (!roomQuery.isSuccess || !roomQuery.data || !id) return;
    if (joinAttemptedRef.current) return;
    const { room, players } = roomQuery.data;
    const userId = useAuthStore.getState().user?.id;
    if (!userId) return;
    // Already a member (arrived via lobby / already seated) — nothing to do.
    if (players.some((p) => p.userId === userId)) return;
    // Quick-play rooms redirect to /matchmaking; closed / in-progress rooms are
    // handled by their own guards. Only a joinable waiting room is auto-joined.
    if (room.isQuickPlay || room.status !== "waiting") return;

    joinAttemptedRef.current = true;
    joinRoomMutation
      .mutateAsync(Number(id))
      .then(() => {
        hasJoinedRef.current = true;
        void roomQuery.refetch();
      })
      .catch((err) => {
        const code = err instanceof FetchError ? err.code : null;
        // ALREADY_IN_ROOM: a membership exists after all (e.g. another tab) —
        // resync from the server rather than bouncing to the lobby.
        if (code === "ALREADY_IN_ROOM") {
          hasJoinedRef.current = true;
          void roomQuery.refetch();
          return;
        }
        hasLeftRef.current = true; // suppress the unmount cleanup-leave
        toast.error(
          code === "ROOM_FULL" ? t("lobby.errors.roomFull") : t("lobby.errors.joinFailed"),
        );
        navigate("/lobby", { replace: true });
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [roomQuery.isSuccess, roomQuery.data, id]);

  // Quick-play rooms have their own dedicated matchmaking screen — a deep link
  // or refresh that lands here must redirect to /matchmaking/:id. Gate on the
  // authoritative query result, and set hasLeftRef FIRST so the unmount
  // auto-leave below does not remove the player from the queue we're sending
  // them into.
  useEffect(() => {
    if (roomQuery.isSuccess && roomQuery.data?.room.isQuickPlay && id) {
      hasLeftRef.current = true;
      navigate(`/matchmaking/${id}`, { replace: true });
    }
  }, [roomQuery.isSuccess, roomQuery.data, id, navigate]);

  // Refetch room state after WebSocket reconnects to catch events missed during disconnect
  useEffect(() => {
    const prev = prevWsStateRef.current;
    prevWsStateRef.current = wsState;
    if (wsState === "connected" && prev !== "connected") {
      roomQuery.refetch();
    }
  }, [wsState, roomQuery]);

  // Handle match_started from WebSocket — navigate all players to the game page.
  // Pass `fromRoom: true` so MatchPage's splash gate triggers (deliberate beat
  // masking the room→game transition).
  useEffect(() => {
    if (matchStarted && id) {
      hasLeftRef.current = true; // Prevent cleanup leave on navigation
      navigate(`/match/${id}`, { state: { fromRoom: true } });
    }
  }, [matchStarted, id, navigate]);

  // Closed-room auto-redirect — once a match ends (or the room is cancelled)
  // the row flips to a terminal status but the URL still resolves. Without
  // this guard, deep-linking to a finished room rendered the seat grid and
  // "waiting for owner to start" copy as if the room were live. We surface a
  // "Room is closed" message for a beat, then navigate to /lobby. Note that
  // "playing" is *not* terminal — the room is live and existing in-room logic
  // (kick gating, matchStarted handler) already covers that path.
  const currentRoomStatus = storeRoom?.status ?? roomQuery.data?.room.status ?? null;
  const isRoomClosed =
    currentRoomStatus === "completed" ||
    currentRoomStatus === "cancelled" ||
    currentRoomStatus === "finished";
  useEffect(() => {
    if (!isRoomClosed || matchStarted) return;
    hasLeftRef.current = true; // Don't fire the cleanup-leave on the way out
    const timer = setTimeout(() => {
      navigate("/lobby", { replace: true });
    }, 2000);
    return () => clearTimeout(timer);
  }, [isRoomClosed, matchStarted, navigate]);

  // Handle system:room_kicked — toast + redirect kicked user back to /lobby.
  // Suppress the unmount auto-leave: the server has already removed us, so
  // the cleanup `/leave` would 404; setting hasLeftRef before navigate keeps
  // the cleanup quiet.
  useEffect(() => {
    if (kickedFromRoomId !== null && id && kickedFromRoomId === Number(id)) {
      hasLeftRef.current = true;
      toast.error(t("room.kickedToast", { name: storeRoom?.name ?? "" }));
      useRoomStore.getState().setKickedFromRoom(null);
      navigate("/lobby");
    }
  }, [kickedFromRoomId, id, navigate, storeRoom?.name, t]);

  // Clear swap-mode whenever the room exits "waiting" status — owner controls
  // disappear in the same render, so a stale source-seat must not survive.
  useEffect(() => {
    if (storeRoom && storeRoom.status !== "waiting") {
      setSwapSourceSeat(null);
    }
  }, [storeRoom]);

  // Clear swap-mode if the source player vacates their seat (left the room or
  // moved seats while we were deciding) — otherwise the next click would fire
  // a swap with an empty source seat and trip SEAT_NOT_OCCUPIED.
  useEffect(() => {
    if (swapSourceSeat !== null && getPlayerAtSeat(storePlayers, swapSourceSeat) === undefined) {
      setSwapSourceSeat(null);
    }
  }, [storePlayers, swapSourceSeat]);

  // Reset stores on unmount AND on :id change — React Router keeps RoomPage
  // mounted across /rooms/:id navigations, so a []-dep cleanup would let
  // room-A chat leak into room-B's UI. Keying the cleanup on `id` fires it
  // both when the route param changes and when the component unmounts.
  useEffect(() => {
    return () => {
      useRoomStore.getState().reset();
      useChatStore.getState().clearRoom();
    };
  }, [id]);

  // Leave room on unmount if player joined and hasn't explicitly left.
  // NOTE: this covers SPA navigation only — a hard unload (address-bar nav to
  // /lobby, refresh, tab close) does NOT run React cleanups; the `pagehide`
  // handler below covers that case with a keepalive request.
  useEffect(() => {
    return () => {
      if (id && hasJoinedRef.current && !hasLeftRef.current) {
        leaveRoomMutation.mutate(Number(id));
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  // Hard unload (typing a URL, refresh, tab close): fire a keepalive leave so
  // the membership doesn't linger and trap the user "in a room". On a refresh
  // the deep-link auto-join above re-establishes membership when the room page
  // reloads, so this only sticks when the user actually navigates away.
  useEffect(() => {
    const onPageHide = () => {
      if (id && hasJoinedRef.current && !hasLeftRef.current) {
        const token = useAuthStore.getState().token;
        if (token) sendLeaveBeacon(Number(id), token);
      }
    };
    window.addEventListener("pagehide", onPageHide);
    return () => window.removeEventListener("pagehide", onPageHide);
  }, [id]);

  // `justCopied` flips the copy-code button into a transient success state
  // (check icon + "Copied!" inline) so the user gets immediate confirmation
  // without relying on the toast alone — the toast is easy to miss when the
  // user's eyes are on the button they just clicked.
  const [justCopied, setJustCopied] = useState(false);
  const copyResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (copyResetTimerRef.current) clearTimeout(copyResetTimerRef.current);
    };
  }, []);

  const handleCopyLink = async () => {
    if (!storeRoom) return;
    try {
      await navigator.clipboard.writeText(storeRoom.code);
      toast.success(t("room.copyLinkSuccess"));
      setJustCopied(true);
      if (copyResetTimerRef.current) clearTimeout(copyResetTimerRef.current);
      copyResetTimerRef.current = setTimeout(() => setJustCopied(false), 2000);
    } catch {
      toast.error(t("room.copyLinkFailed", { code: storeRoom.code }));
    }
  };

  const handleLeaveRoom = async () => {
    hasLeftRef.current = true;
    try {
      await leaveRoomMutation.mutateAsync(Number(id));
    } catch (err) {
      // Story 8.5-1 AC3: server returns 409 MATCH_ALREADY_STARTED when the
      // room transitioned to "playing" between the user clicking leave and
      // the leave tx running. Surface a toast and abort the navigation —
      // the client should stay in the room (the auto-start UI flow takes
      // over from here).
      if (err instanceof FetchError && err.code === "MATCH_ALREADY_STARTED") {
        hasLeftRef.current = false;
        toast.error(t("room.errors.alreadyStarted"));
        return;
      }
      // Other errors: still navigate back (preserves prior UX).
    }
    navigate("/lobby");
  };

  const handleSelectSeat = async (seatIndex: number) => {
    if (!storeRoom) return;
    // If the owner is in swap-mode, route the click to the swap target handler
    // instead of the regular seat-select path.
    if (swapSourceSeat !== null) {
      handleSwapTarget(seatIndex);
      return;
    }
    try {
      const data = await selectSeatMutation.mutateAsync({ roomId: storeRoom.id, seat: seatIndex });
      // Update players from response (in case WS hasn't arrived yet)
      useRoomStore.getState().setPlayers(data.players);
      if (data.matchStarted) {
        hasLeftRef.current = true;
        navigate(`/match/${storeRoom.id}`, { state: { fromRoom: true } });
        return;
      }
    } catch (err) {
      if (err instanceof FetchError && err.code === "SEAT_TAKEN") {
        toast.error(t("room.seatTaken"));
      } else {
        toast.error(t("room.errors.seatFailed"));
      }
    }
  };

  const handleSwapTarget = async (targetSeat: number) => {
    if (!storeRoom || swapSourceSeat === null) return;

    // Click the source again, the current user's own seat, or any click while
    // a swap is already pending cancels swap mode without firing the API.
    // Empty target seats are valid — they trigger a move-to-empty server-side.
    const targetPlayer = getPlayerAtSeat(players, targetSeat);
    const isCurrentUser =
      targetPlayer !== undefined && currentUser !== null && targetPlayer.userId === currentUser.id;
    if (targetSeat === swapSourceSeat || isCurrentUser || swapSeatsMutation.isPending) {
      setSwapSourceSeat(null);
      return;
    }

    const seatA = swapSourceSeat;
    setSwapSourceSeat(null);
    try {
      await swapSeatsMutation.mutateAsync({
        roomId: storeRoom.id,
        seatA,
        seatB: targetSeat,
      });
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "NOT_ROOM_OWNER") {
          toast.error(t("room.errors.notOwnerAction"));
        } else if (err.code === "ROOM_NOT_WAITING") {
          toast.error(t("room.errors.roomStarted"));
        } else if (err.code === "SEAT_NOT_OCCUPIED") {
          toast.error(t("room.errors.seatNotOccupied"));
        } else {
          toast.error(t("room.errors.swapFailed"));
        }
      } else {
        toast.error(t("room.errors.swapFailed"));
      }
    }
  };

  const handleLeaveSeat = async () => {
    if (!storeRoom || leaveSeatMutation.isPending) return;
    try {
      const data = await leaveSeatMutation.mutateAsync({ roomId: storeRoom.id });
      useRoomStore.getState().setPlayers(data.players);
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "QUICK_PLAY_LEAVE_SEAT_BLOCKED") {
          toast.error(t("room.errors.leaveSeatQuickPlay"));
        } else if (err.code === "OWNER_CANNOT_LEAVE_SEAT") {
          toast.error(t("room.errors.leaveSeatOwner"));
        } else if (err.code === "ROOM_NOT_WAITING") {
          toast.error(t("room.errors.roomStarted"));
        } else {
          toast.error(t("room.errors.leaveSeatFailed"));
        }
      } else {
        toast.error(t("room.errors.leaveSeatFailed"));
      }
    }
  };

  const handleTransferConfirm = async () => {
    if (!storeRoom || !transferConfirm) return;
    const target = transferConfirm;
    setTransferConfirm(null);
    try {
      await transferOwnershipMutation.mutateAsync({
        roomId: storeRoom.id,
        userId: target.userId,
      });
      // Server WS broadcast (system:room_owner_changed) converges store state.
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "NOT_ROOM_OWNER") {
          toast.error(t("room.errors.notOwnerAction"));
        } else if (err.code === "ROOM_NOT_WAITING") {
          toast.error(t("room.errors.roomStarted"));
        } else if (err.code === "CANNOT_PROMOTE_UNSEATED") {
          toast.error(t("room.errors.promoteUnseated"));
        } else if (err.code === "NOT_IN_ROOM") {
          toast.error(t("room.errors.promoteNotInRoom"));
        } else {
          toast.error(t("room.errors.transferFailed"));
        }
      } else {
        toast.error(t("room.errors.transferFailed"));
      }
    }
  };

  const handleKickConfirm = async () => {
    if (!storeRoom || !kickConfirm) return;
    const target = kickConfirm;
    setKickConfirm(null);
    try {
      await kickPlayerMutation.mutateAsync({
        roomId: storeRoom.id,
        userId: target.userId,
      });
      // Server WS broadcast (system:player_left) converges store state — no
      // optimistic update.
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "NOT_ROOM_OWNER") {
          toast.error(t("room.errors.notOwnerAction"));
        } else if (err.code === "ROOM_NOT_WAITING") {
          toast.error(t("room.errors.roomStarted"));
        } else {
          toast.error(t("room.errors.kickFailed"));
        }
      } else {
        toast.error(t("room.errors.kickFailed"));
      }
    }
  };

  const handleStartGame = async () => {
    if (!storeRoom || startGameMutation.isPending) return;
    try {
      await startGameMutation.mutateAsync(storeRoom.id);
      hasLeftRef.current = true; // Prevent cleanup leave on navigation
      navigate(`/match/${storeRoom.id}`, { state: { fromRoom: true } });
    } catch (err) {
      if (err instanceof FetchError && err.code === "NOT_ROOM_OWNER") {
        toast.error(t("room.errors.notOwner"));
      } else if (err instanceof FetchError && err.code === "NOT_ALL_SEATED") {
        toast.error(t("room.errors.notAllSeated"));
      } else {
        toast.error(t("room.errors.startFailed"));
      }
    }
  };

  // Use query loading state for initial load
  if (roomQuery.isPending) {
    return (
      <div className="mx-auto max-w-330 px-8 py-8" data-testid="room-page-loading">
        <div className="bg-surface-sunken mb-6 h-8 w-48 animate-pulse rounded" />
        <div className="grid grid-cols-2 gap-6">
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="bg-surface border-border h-32 animate-pulse rounded-2xl border"
            />
          ))}
        </div>
      </div>
    );
  }

  if (roomQuery.isError || (!storeRoom && !roomQuery.data)) {
    return (
      <div className="mx-auto max-w-330 px-8 py-12 text-center" data-testid="room-page-error">
        <p className="text-ink-dim mb-4 text-sm">{t("room.notFound")}</p>
        <Button variant="ghost" onClick={() => navigate("/lobby")} data-testid="back-to-lobby">
          {t("room.notFoundAction")}
        </Button>
      </div>
    );
  }

  if (isRoomClosed && !matchStarted) {
    return (
      <div className="mx-auto max-w-330 px-8 py-12 text-center" data-testid="room-page-closed">
        <p className="font-display text-ink mb-2 text-lg font-semibold">{t("room.roomClosed")}</p>
        <p className="text-ink-dim animate-pulse text-sm">{t("room.returningToLobby")}</p>
      </div>
    );
  }

  // Render from store (source of truth after initial seed), fall back to query data
  // during the brief window before the seeding effect runs
  const room = storeRoom ?? roomQuery.data!.room;

  // Quick-play rooms live on the dedicated matchmaking screen; the redirect
  // effect above navigates there. Render nothing rather than flashing the
  // custom-room seat grid for a frame.
  if (room.isQuickPlay) return null;

  const players = storePlayers.length > 0 ? storePlayers : roomQuery.data!.players;

  const variantLabel = t(variantKeys[room.variant] ?? room.variant);
  const matchModeLabel = t(matchModeKeys[room.matchMode] ?? room.matchMode);
  const timerLabel =
    room.timerStyle === "relaxed"
      ? t("lobby.card.relaxed")
      : t("lobby.card.timerSeconds", { seconds: room.timerDurationSeconds ?? "?" });

  const isOwner = currentUser !== null && currentUser.id === room.ownerId;
  const seatedCount = players.filter((p) => p.seat !== null).length;
  const allSeated = seatedCount === 4;

  const ownerPlayer = players.find((p) => p.userId === room.ownerId);
  const ownerUsername = ownerPlayer?.username ?? t("room.seatOwner");
  const isRelaxed = room.timerStyle === "relaxed";

  const viewerSeat = currentUser
    ? (players.find((p) => p.userId === currentUser.id)?.seat ?? null)
    : null;
  const resolveSeatMode = (seatIndex: number): "us" | "them" | "neutral" => {
    if (viewerSeat === null || viewerSeat === undefined) return "neutral";
    return viewerSeat % 2 === seatIndex % 2 ? "us" : "them";
  };

  // Team legend (seated): render both seats of each diagonal pair as slots so
  // the viewer sees their whole table at a glance. Whichever pair contains the
  // viewer is "Us". `slotLabel` returns null for an empty (still "open") seat.
  const yourPair = viewerSeat !== null && viewerSeat % 2 === 0 ? [0, 2] : [1, 3];
  const otherPair = yourPair[0] === 0 ? [1, 3] : [0, 2];
  // Render a diagonal pair's two seats as "(name + name)" — your own seat reads
  // "you" (accent), an empty seat reads a blinking "open". The parentheses
  // bracket the pair so the legend reads "Us (you + open) vs Them (open + open)".
  const renderSlots = (pair: number[]) => (
    <span className="text-ink-mute inline-flex flex-wrap items-center">
      <span className="text-ink-off" aria-hidden>
        (
      </span>
      <span className="inline-flex flex-wrap items-center gap-1">
        {pair.map((seatIndex, i) => {
          const p = getPlayerAtSeat(players, seatIndex);
          const isYouSlot = p?.userId === currentUser?.id;
          return (
            <span key={seatIndex} className="inline-flex items-center gap-1">
              {i > 0 && <span className="text-ink-off">+</span>}
              {p ? (
                <span className={isYouSlot ? "text-accent font-semibold" : "text-ink-dim"}>
                  {isYouSlot ? t("room.seatYouInline") : p.username}
                </span>
              ) : (
                <span className="text-ink-mute animate-pulse">{t("room.legendOpen")}</span>
              )}
            </span>
          );
        })}
      </span>
      <span className="text-ink-off" aria-hidden>
        )
      </span>
    </span>
  );

  // Roster avatar tone — mirrors the seat tiles' perspective coloring: gold
  // (us) / silver (them) for seated players once the viewer has sat, and
  // felt-green (undetermined) for standing members AND for everyone while the
  // viewer is still unseated (no team perspective established yet).
  const rosterTeam = (p: RoomPlayer): "A" | "B" | null => {
    if (p.seat === null || viewerSeat === null) return null;
    return resolveSeatMode(p.seat) === "us" ? "A" : "B";
  };

  // Live player rows for the open owner-confirmation dialogs — re-derived from
  // the roster so the dialog's seat/team chips reflect the current state.
  const kickTarget = kickConfirm
    ? (players.find((p) => p.userId === kickConfirm.userId) ?? null)
    : null;
  const transferTarget = transferConfirm
    ? (players.find((p) => p.userId === transferConfirm.userId) ?? null)
    : null;

  // Contextual tip under the seat diamond, keyed to the viewer's role.
  const tipKey =
    viewerSeat === null ? "room.tip.unseated" : isOwner ? "room.tip.owner" : "room.tip.seated";

  // Action-bar CTA — one primary felt-green button whose label/enabled state is
  // driven by swap mode, quick-play auto-start, and ownership. Only the owner's
  // "Start match" (all seated, not swapping) is interactive; everything else is
  // a disabled "waiting" affordance.
  const inSwapMode = swapSourceSeat !== null;
  let ctaLabel: string;
  let ctaDisabled = true;
  let ctaOnClick: (() => void) | undefined;
  let ctaTestId = "room-cta";
  if (inSwapMode) {
    ctaLabel = allSeated ? t("room.actionBarFinishSwap") : t("room.waitingForPlayers");
  } else if (room.isQuickPlay) {
    ctaLabel = allSeated ? t("room.autoStarting") : t("room.waitingForPlayers");
  } else if (isOwner) {
    ctaTestId = "start-game";
    ctaDisabled = !allSeated || startGameMutation.isPending;
    ctaOnClick = ctaDisabled ? undefined : handleStartGame;
    ctaLabel = startGameMutation.isPending
      ? t("room.matchStarting")
      : allSeated
        ? t("room.startMatch")
        : t("room.waitingForPlayers");
  } else {
    ctaLabel = allSeated ? t("room.waitingForOwner") : t("room.waitingForPlayers");
  }

  return (
    <div className="mx-auto max-w-330 px-4 pt-6 pb-24 md:px-8 md:pb-6">
      <div data-testid="room-page">
        {/* Room info card — parchment chrome (brass top hairline + soft inset
            shadow). Left: name + meta row (host · roster · seated count).
            Right: game-mode badges + copy-code chip. No top back button —
            the bottom "Leave room" action returns to the lobby. */}
        <div
          className="bg-surface border-border relative mb-6 rounded-lg border p-6 shadow-[0_18px_44px_-28px_rgba(14,58,36,0.32),0_2px_0_rgba(255,255,255,0.6)_inset]"
          data-testid="room-info-card"
        >
          <div
            aria-hidden="true"
            className="pointer-events-none absolute -top-px right-7 left-7 h-0.5 bg-[linear-gradient(90deg,transparent,var(--brass)_50%,transparent)] opacity-70"
          />

          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            {/* Left — name + meta row */}
            <div className="flex min-w-0 flex-col gap-1.5">
              <h1
                // pr reserves room for the code chip that floats onto this row
                // on mobile (absolute, top-right of the card); cleared at md
                // where the chip returns to the right column.
                className="font-display text-ink m-0 truncate pr-28 text-[28px] leading-tight font-bold tracking-[-0.6px] md:pr-0"
                data-testid="room-info-name"
              >
                {room.name}
              </h1>
              <div className="text-ink-dim flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[13px]">
                <span className="inline-flex items-center gap-1.5">
                  <Crown className="text-brass-deep size-3" aria-hidden />
                  <Trans
                    i18nKey="room.ownerLine"
                    values={{ owner: ownerUsername }}
                    components={{ name: <span className="text-ink font-semibold" /> }}
                  />
                </span>
                <span className="text-ink-off" aria-hidden>
                  ·
                </span>
                <DropdownMenu>
                  <DropdownMenuTrigger
                    className="text-ink hover:bg-surface-sunken focus-visible:ring-accent inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-0.5 outline-none transition-colors focus-visible:ring-2"
                    aria-label={t("room.inRoomList.ariaLabel")}
                    data-testid="in-room-count"
                  >
                    <Users className="text-ink-dim size-3.5" />
                    <span>{t("room.inRoomCount", { count: players.length })}</span>
                    <ChevronDown className="text-ink-mute size-3" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="start"
                    className="bg-surface-elevated min-w-64"
                    data-testid="in-room-list"
                  >
                    <div className="text-brass-deep px-2 pt-1.5 pb-1 font-mono text-[10.5px] font-semibold tracking-[1.8px] uppercase">
                      {t("room.inRoomList.title")}
                    </div>
                    {players.length === 0 ? (
                      <p className="text-ink-mute px-2 py-1.5 text-xs">
                        {t("room.inRoomList.empty")}
                      </p>
                    ) : (
                      <ul className="flex flex-col">
                        {players.map((p) => {
                          const isYou = currentUser?.id === p.userId;
                          const isRoomOwner = p.userId === room.ownerId;
                          const isWaiting = room.status === "waiting";
                          const ownerCanActOnRow = isOwner && isWaiting && !isRoomOwner && !isYou;
                          const ownerCanKickRow = ownerCanActOnRow;
                          const ownerCanPromoteRow = ownerCanActOnRow && p.seat !== null;
                          const seated = p.seat !== null;
                          const seatLabel = seated
                            ? t("room.inRoomList.seated", {
                                seat: (p.seat as number) + 1,
                              })
                            : t("room.inRoomList.notSeated");
                          return (
                            <li
                              key={p.userId}
                              className="flex items-center gap-2 px-2 py-1.5 text-sm"
                              data-testid={`in-room-list-item-${p.userId}`}
                            >
                              <Avatar
                                name={p.username}
                                size={22}
                                team={rosterTeam(p)}
                                you={isYou}
                                owner={isRoomOwner}
                              />
                              <span className="text-ink flex min-w-0 flex-1 items-center gap-1">
                                {isRoomOwner && (
                                  <Crown className="text-brass-deep size-3 shrink-0" aria-hidden />
                                )}
                                <span className="truncate font-medium">{p.username}</span>
                                {isYou && (
                                  <span className="text-accent shrink-0 text-xs font-semibold">
                                    · {t("room.seatYouInline")}
                                  </span>
                                )}
                              </span>
                              <span
                                className={cn(
                                  "shrink-0 rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold tracking-[0.4px] uppercase",
                                  seated
                                    ? "border-accent bg-accent-soft text-accent-deep"
                                    : "border-border bg-surface-sunken text-ink-mute",
                                )}
                              >
                                {seatLabel}
                              </span>
                              {ownerCanPromoteRow && (
                                <button
                                  type="button"
                                  className="text-ink-mute hover:bg-surface-sunken hover:text-brass-deep rounded p-0.5 transition-colors disabled:cursor-not-allowed disabled:opacity-40"
                                  aria-label={t("room.promoteIconLabel", {
                                    username: p.username,
                                  })}
                                  title={t("room.promoteIconLabel", {
                                    username: p.username,
                                  })}
                                  data-testid={`promote-player-${p.userId}`}
                                  disabled={transferOwnershipMutation.isPending}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    setTransferConfirm({
                                      userId: p.userId,
                                      username: p.username,
                                    });
                                  }}
                                >
                                  <Crown className="size-3.5" />
                                </button>
                              )}
                              {ownerCanKickRow && (
                                <button
                                  type="button"
                                  className="text-ink-mute hover:bg-surface-sunken hover:text-destructive rounded p-0.5 transition-colors disabled:cursor-not-allowed disabled:opacity-40"
                                  aria-label={t("room.kickIconLabel", {
                                    username: p.username,
                                  })}
                                  title={t("room.kickIconLabel", {
                                    username: p.username,
                                  })}
                                  data-testid={`kick-list-${p.userId}`}
                                  disabled={kickPlayerMutation.isPending}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    setSwapSourceSeat(null);
                                    setKickConfirm({
                                      seat: p.seat ?? -1,
                                      userId: p.userId,
                                      username: p.username,
                                    });
                                  }}
                                >
                                  <UserX className="size-3.5" />
                                </button>
                              )}
                            </li>
                          );
                        })}
                      </ul>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
                <span className="text-ink-off" aria-hidden>
                  ·
                </span>
                <span
                  className={seatedCount === 4 ? "text-accent font-semibold" : undefined}
                  data-testid="seated-count"
                >
                  {t("room.seatedCount", { current: seatedCount, max: 4 })}
                </span>
              </div>
            </div>

            {/* Right — copy-code chip on top, game-mode badges below. */}
            <div className="flex flex-col items-start gap-2 md:items-end">
              {/* On mobile the chip floats to the card's top-right so it shares
                  the title's row (the card is `relative`); at md it returns to
                  static flow above the badges — desktop layout unchanged. */}
              <div className="absolute top-6 right-6 md:static md:top-auto md:right-auto">
                <CodeChip
                  code={room.code}
                  variant="compact"
                  copied={justCopied}
                  onCopy={handleCopyLink}
                  ariaLabel={t("room.copyLinkAriaLabel", { code: room.code })}
                  testId="copy-link"
                  codeTestId="room-code"
                />
              </div>
              <div
                className="flex flex-wrap items-center gap-2 md:justify-end"
                data-testid="room-info-badges"
              >
                <Badge tone="brass" icon={<Trophy className="size-3" />}>
                  <span data-testid="badge-variant">{variantLabel}</span>
                </Badge>
                <Badge tone="brass">
                  <span data-testid="badge-match-mode">{matchModeLabel}</span>
                </Badge>
                <Badge tone={isRelaxed ? "accent" : "neutral"} icon={<Clock className="size-3" />}>
                  <span data-testid="badge-timer">{timerLabel}</span>
                </Badge>
                {room.isQuickPlay && (
                  <Badge tone="accent" icon={<Zap className="size-3" />}>
                    <span data-testid="badge-quick-play">{t("lobby.quickPlay")}</span>
                  </Badge>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Team legend — dashed parchment card in every state. Unseated: a
            single neutral square indicator + "open seats" copy. Seated: the
            two diagonal pairs read "Us {you + open} vs Them {open + open}",
            centred, with solid gold/silver square indicators. */}
        <div
          className="border-border-2 bg-surface mb-3 rounded-lg border border-dashed px-4 py-3"
          data-testid="team-legend"
        >
          {viewerSeat === null ? (
            <div className="flex items-center justify-center gap-2.5">
              <LegendIndicator variant="neutral" />
              <span className="text-ink-dim text-[12.5px]">{t("room.legendNeutral")}</span>
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-[12.5px]">
              <span className="inline-flex items-center gap-2" data-team="teamA">
                <LegendIndicator variant="us" />
                <span className="text-team-a font-semibold">{t("room.legendUs")}</span>
                {renderSlots(yourPair)}
              </span>
              <span className="text-ink-off" aria-hidden>
                {t("room.legendVs")}
              </span>
              <span className="inline-flex items-center gap-2" data-team="teamB">
                <LegendIndicator variant="them" />
                <span className="text-team-b font-semibold">{t("room.legendThem")}</span>
                {renderSlots(otherPair)}
              </span>
            </div>
          )}
        </div>

        {/* Diamond seat layout. Seats are pinned to fixed cardinal positions
            (seat 0 → south, 1 → east, 2 → north, 3 → west) — the lobby does
            NOT rotate. Visual mode is viewer-relative: once the viewer sits,
            same-parity tiles become "us" (gold tint) and the other parity
            become "them" (silver tint). When unseated everything is neutral. */}
        <div className="relative" data-testid="seats-grid">
          {/* Partner connectors — vertical for the Team A pair, horizontal for
              the Team B pair. Sit behind seat tiles via z-index. Hidden on
              the <sm stacked layout where row/column position already conveys
              partnership. Color follows the viewer's perspective when seated. */}
          <div aria-hidden className="pointer-events-none absolute inset-0 z-0 hidden sm:block">
            <div
              className="absolute top-[12%] left-1/2 h-[76%] w-0.5 -translate-x-1/2 opacity-50"
              style={{
                background:
                  viewerSeat === null
                    ? "linear-gradient(180deg, transparent 0%, var(--border-2) 25%, var(--border-2) 75%, transparent 100%)"
                    : viewerSeat % 2 === 0
                      ? "linear-gradient(180deg, transparent 0%, var(--team-a-edge) 20%, var(--team-a-edge) 80%, transparent 100%)"
                      : "linear-gradient(180deg, transparent 0%, var(--team-b-edge-soft) 20%, var(--team-b-edge-soft) 80%, transparent 100%)",
              }}
            />
            <div
              className="absolute top-1/2 left-[12%] h-0.5 w-[76%] -translate-y-1/2 opacity-50"
              style={{
                background:
                  viewerSeat === null
                    ? "linear-gradient(90deg, transparent 0%, var(--border-2) 25%, var(--border-2) 75%, transparent 100%)"
                    : viewerSeat % 2 === 1
                      ? "linear-gradient(90deg, transparent 0%, var(--team-a-edge) 20%, var(--team-a-edge) 80%, transparent 100%)"
                      : "linear-gradient(90deg, transparent 0%, var(--team-b-edge-soft) 20%, var(--team-b-edge-soft) 80%, transparent 100%)",
              }}
            />
          </div>

          <div className="relative z-10 mb-4 grid grid-cols-2 gap-3 sm:mb-6 sm:grid-cols-[1fr_1fr_1fr] sm:gap-5 sm:[grid-template-areas:'._north_.''west_center_east''._south_.']">
            {SEAT_INDEXES.map((seatIndex) => {
              const player = getPlayerAtSeat(players, seatIndex);
              const cardinal = SEAT_TO_CARDINAL[seatIndex] as CardinalPosition;
              const isCurrentUser =
                player !== undefined && currentUser !== null && player.userId === currentUser.id;
              const isSeatOwner = player !== undefined && player.userId === room.ownerId;
              const isWaiting = room.status === "waiting";
              const ownerCanKick = isOwner && isWaiting && player !== undefined && !isSeatOwner;
              const isSwapSource = swapSourceSeat === seatIndex;
              const ownerCanInitiateSwap =
                isOwner && isWaiting && player !== undefined && !isCurrentUser && !inSwapMode;
              const canLeaveOwnSeat =
                isCurrentUser && isWaiting && !room.isQuickPlay && !isSeatOwner && !inSwapMode;
              const isClickable =
                player === undefined || isCurrentUser || ownerCanInitiateSwap || inSwapMode;
              const kickPendingForThisSeat =
                kickPlayerMutation.isPending &&
                player !== undefined &&
                kickPlayerMutation.variables?.userId === player.userId;
              const swapPendingForThisSeat =
                swapSeatsMutation.isPending &&
                (swapSeatsMutation.variables?.seatA === seatIndex ||
                  swapSeatsMutation.variables?.seatB === seatIndex);
              const leaveSeatPendingForThisSeat = leaveSeatMutation.isPending && isCurrentUser;
              const isPendingForThisSeat =
                kickPendingForThisSeat || swapPendingForThisSeat || leaveSeatPendingForThisSeat;

              return (
                <SeatTile
                  key={seatIndex}
                  seatIndex={seatIndex as 0 | 1 | 2 | 3}
                  cardinal={cardinal}
                  mode={resolveSeatMode(seatIndex)}
                  player={player}
                  isYou={isCurrentUser}
                  isHost={isSeatOwner}
                  isSwapSource={isSwapSource}
                  swapMode={inSwapMode}
                  isClickable={isClickable}
                  isPending={isPendingForThisSeat}
                  ownerCanActOnRow={ownerCanKick}
                  onSelect={() => {
                    if (inSwapMode) {
                      handleSwapTarget(seatIndex);
                      return;
                    }
                    if (ownerCanInitiateSwap) {
                      setSwapSourceSeat(seatIndex);
                      return;
                    }
                    if (canLeaveOwnSeat) {
                      handleLeaveSeat();
                      return;
                    }
                    if (isClickable) {
                      handleSelectSeat(seatIndex);
                    }
                  }}
                  onKick={
                    ownerCanKick && player
                      ? () => {
                          setSwapSourceSeat(null);
                          setKickConfirm({
                            seat: seatIndex,
                            userId: player.userId,
                            username: player.username,
                          });
                        }
                      : undefined
                  }
                  onPromote={
                    ownerCanKick && player
                      ? () => {
                          setSwapSourceSeat(null);
                          setTransferConfirm({
                            userId: player.userId,
                            username: player.username,
                          });
                        }
                      : undefined
                  }
                />
              );
            })}

            {/* Felt-green centre diamond — only in the 3×3 desktop layout (the
                mobile 2×2 stack has no centre cell). Reads just "TABLE"; the
                points mode already lives in the room-info card above. */}
            <div
              className="hidden items-center justify-center sm:flex"
              style={{ gridArea: "center" }}
              aria-hidden
              data-testid="table-center"
            >
              <div
                className="flex size-18 items-center justify-center rounded-lg border-[1.5px]"
                style={{
                  transform: "rotate(45deg)",
                  background:
                    "radial-gradient(circle at 30% 30%, #1d6c3b 0%, var(--accent-deep) 70%), var(--accent-deep)",
                  borderColor: "var(--brass)",
                  boxShadow:
                    "0 6px 18px -6px rgba(14,58,36,0.40), inset 0 0 0 4px rgba(201,168,118,0.12)",
                }}
              >
                <span
                  className="font-mono text-[9px] font-semibold tracking-[2px] uppercase"
                  style={{ transform: "rotate(-45deg)", color: "var(--brass)" }}
                >
                  {t("room.tableCenter")}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Contextual tip — guidance keyed to the viewer's role, sitting between
            the diamond and the action bar. */}
        <div
          className="text-ink-mute mb-4 flex items-center justify-center gap-2 px-2 text-center text-[12px]"
          data-testid="room-tip"
        >
          <Sparkles className="text-brass-deep size-3 shrink-0" aria-hidden />
          <span>{t(tipKey)}</span>
        </div>

        {/* Action bar — leave-room (and, in swap mode, a "pick a target" chip)
            on the left; one primary CTA on the right whose state is resolved
            above (Start match / Finish swap to start / Waiting …). */}
        <div
          // Stack on phones: the long "waiting for host" CTA label can't shrink
          // (fixed-height, nowrap button), so a single justify-between row made
          // it overlap the Leave button. flex-col puts Leave on top + a
          // full-width CTA below; the original row returns at sm.
          className="border-border bg-surface flex flex-col gap-3 rounded-lg border px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
          data-testid="action-bar"
        >
          <div className="flex min-w-0 flex-wrap items-center gap-3">
            <Button
              variant="ghost"
              onClick={handleLeaveRoom}
              data-testid="leave-room"
              className="text-destructive hover:text-destructive shrink-0"
            >
              {t("room.leaveRoom")}
            </Button>
            {inSwapMode && (
              <span
                className="border-accent bg-accent-soft text-accent-deep inline-flex shrink-0 items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium"
                data-testid="swap-mode-banner"
              >
                <Shuffle className="size-3" />
                {t("room.actionBarSwap")}
              </span>
            )}
          </div>
          <Button
            size="cta"
            onClick={ctaOnClick}
            disabled={ctaDisabled}
            title={!inSwapMode && isOwner && !allSeated ? t("room.startMatchDisabled") : undefined}
            data-testid={ctaTestId}
            // Full-width on phones; let the long "waiting for host" label wrap
            // (the cta size is fixed-height + nowrap by default) so it never
            // clips. Original single-line pill returns at sm.
            className="h-auto min-h-11.5 w-full py-2.5 leading-tight whitespace-normal sm:h-11.5 sm:w-auto sm:py-0 sm:leading-normal sm:whitespace-nowrap"
          >
            {ctaLabel}
          </Button>
        </div>
      </div>

      <RoomChatDock roomId={room.id} />

      {/* Owner transfer-ownership confirmation dialog */}
      <TransferOwnershipDialog
        open={transferConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setTransferConfirm(null);
        }}
        onConfirm={handleTransferConfirm}
        pending={transferOwnershipMutation.isPending}
        fromName={currentUser?.username ?? ownerUsername}
        target={{
          name: transferConfirm?.username ?? "",
          seat: transferTarget?.seat ?? null,
          team: transferTarget ? rosterTeam(transferTarget) : null,
        }}
      />

      {/* Owner kick confirmation dialog */}
      <KickPlayerDialog
        open={kickConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setKickConfirm(null);
        }}
        onConfirm={handleKickConfirm}
        pending={kickPlayerMutation.isPending}
        target={{
          name: kickConfirm?.username ?? "",
          seat: kickTarget?.seat ?? kickConfirm?.seat ?? null,
          team: kickTarget ? rosterTeam(kickTarget) : null,
        }}
      />
    </div>
  );
}
