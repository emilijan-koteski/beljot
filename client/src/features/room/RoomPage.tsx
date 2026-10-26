import {
  Ban,
  Bot,
  ChevronDown,
  Clock,
  Coins,
  Crown,
  DoorOpen,
  Flag,
  History,
  Lock,
  LockOpen,
  RotateCw,
  Send,
  Shuffle,
  Trophy,
  Unlink,
  UserCheck,
  Users,
  UserX,
  Zap,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { InviteFriendsDialog } from "@/features/room/components/InviteFriendsDialog";
import { LastMatchDialog } from "@/features/room/components/LastMatchDialog";
import { RoomChatDock } from "@/features/room/components/RoomChatDock";
import { SeatTile } from "@/features/room/components/SeatTile";
import {
  KickPlayerDialog,
  RemoveBotDialog,
  TransferOwnershipDialog,
} from "@/features/room/OwnerConfirmDialogs";
import { RoomPrivacyDialog } from "@/features/room/RoomPrivacyDialog";
import { FetchError } from "@/shared/api/axiosClient";
import { HonorShield } from "@/shared/components/HonorShield";
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
  useAddBotMutation,
  useJoinRoomMutation,
  useKickPlayerMutation,
  useLeaveRoomMutation,
  useLeaveSeatMutation,
  useRemoveBotMutation,
  useSelectSeatMutation,
  useStartMatchMutation,
  useSwapSeatsMutation,
  useTransferOwnershipMutation,
} from "@/shared/hooks/mutations/useRooms";
import { useRoomLastMatchQuery } from "@/shared/hooks/queries/useMatches";
import { useRoomDetailQuery } from "@/shared/hooks/queries/useRooms";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { botDisplayName } from "@/shared/lib/botName";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import {
  honorFloorLabel,
  honorQualifies,
  honorRoomIsGated,
  honorTierForScore,
} from "@/shared/lib/honor";
import { joinFailureMessage as sharedJoinFailureMessage } from "@/shared/lib/joinFailure";
import { modeLabel, variantLabel } from "@/shared/lib/roomLabels";
import { cn } from "@/shared/lib/utils";
import { useWsConnectionState } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

// Waiting-room seat position. Seats map to fixed cardinals, unlike the in-game table
// that rotates the viewer to the bottom.
type CardinalPosition = "south" | "east" | "north" | "west";

const SEAT_RENDER_ORDER = [0, 2, 1, 3] as const;

const SEAT_TO_CARDINAL: Record<number, CardinalPosition> = {
  0: "south",
  1: "east",
  2: "north",
  3: "west",
};

function getPlayerAtSeat(players: RoomPlayer[], seatIndex: number): RoomPlayer | undefined {
  return players.find((p) => p.seat === seatIndex);
}

// Team-legend square mirroring the seat tiles: dashed and blinking while the viewer is unseated,
// solid gold or silver with a team dot once seated.
function LegendIndicator({ variant }: { variant: "neutral" | "us" | "them" }) {
  const tokens = {
    neutral: { border: "var(--brass)", dot: "var(--brass)", dashed: true, pulse: true },
    us: { border: "var(--team-a)", dot: "var(--team-a-fill)", dashed: false, pulse: false },
    them: { border: "var(--team-b)", dot: "var(--team-b-fill)", dashed: false, pulse: false },
  }[variant];
  return (
    <span
      aria-hidden
      className="inline-flex h-4.5 w-7 items-center justify-center rounded"
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

// Leaves the room with a keepalive POST that outlives a hard unload (refresh, tab close, address-bar nav).
// React skips effect cleanups on unload, so without this the player stays a member and cannot join other rooms.
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
  const returnToLobby = useLobbyReturn();
  const currentUser = useAuthStore((s) => s.user);

  const roomQuery = useRoomDetailQuery(id ? Number(id) : undefined);

  const wsState = useWsConnectionState();
  const prevWsStateRef = useRef(wsState);

  const storeRoom = useRoomStore((s) => s.room);
  const storePlayers = useRoomStore((s) => s.players);
  const matchStarted = useRoomStore((s) => s.matchStarted);
  const kickedFromRoomId = useRoomStore((s) => s.kickedFromRoomId);
  const roomEjection = useRoomStore((s) => s.roomEjection);
  const returnedUserIds = useRoomStore((s) => s.returnedUserIds);

  const skipAutoLeaveRef = useRef(false);
  const hasJoinedRef = useRef(false);
  const joinAttemptedRef = useRef(false);
  const pendingLeaveRef = useRef<{ id: number; timer: ReturnType<typeof setTimeout> } | null>(null);

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
  const [removeBotConfirm, setRemoveBotConfirm] = useState<{ seat: number } | null>(null);
  const [showPrivacy, setShowPrivacy] = useState(false);
  const [showInviteFriends, setShowInviteFriends] = useState(false);
  const [showLastMatch, setShowLastMatch] = useState(false);

  const [deepLinkPrivateRoom, setDeepLinkPrivateRoom] = useState<Room | null>(null);
  const [deepLinkPasswordError, setDeepLinkPasswordError] = useState<string | null>(null);

  const joinRoomMutation = useJoinRoomMutation();
  const selectSeatMutation = useSelectSeatMutation();
  const startGameMutation = useStartMatchMutation();
  const leaveRoomMutation = useLeaveRoomMutation();
  const kickPlayerMutation = useKickPlayerMutation();
  const swapSeatsMutation = useSwapSeatsMutation();
  const leaveSeatMutation = useLeaveSeatMutation();
  const transferOwnershipMutation = useTransferOwnershipMutation();
  const addBotMutation = useAddBotMutation();
  const removeBotMutation = useRemoveBotMutation();

  useEffect(() => {
    if (roomQuery.data) {
      const store = useRoomStore.getState();
      store.setRoom(roomQuery.data.room);
      store.setPlayers(roomQuery.data.players);
      store.setReturnedUserIds(roomQuery.data.returnedUserIds ?? []);
      store.setCurrentRoomId(roomQuery.data.room.id);
      const userId = useAuthStore.getState().user?.id;
      if (userId && roomQuery.data.players.some((p) => p.userId === userId)) {
        hasJoinedRef.current = true;
      }
    }
  }, [roomQuery.data]);

  function joinFailureMessage(code: string | null, room: Room): string {
    return sharedJoinFailureMessage(code, room);
  }

  useEffect(() => {
    if (!roomQuery.isSuccess || !roomQuery.data || !id) return;
    if (!roomQuery.isFetchedAfterMount) return;
    if (joinAttemptedRef.current) return;
    const { room, players } = roomQuery.data;
    const userId = useAuthStore.getState().user?.id;
    if (!userId) return;
    if (players.some((p) => p.userId === userId)) return;
    if (room.isQuickPlay || room.status !== "waiting") return;

    if (room.isPrivate) {
      joinAttemptedRef.current = true;
      setDeepLinkPasswordError(null);
      setDeepLinkPrivateRoom(room);
      return;
    }

    joinAttemptedRef.current = true;
    joinRoomMutation
      .mutateAsync({ id: Number(id) })
      .then(() => {
        hasJoinedRef.current = true;
        void roomQuery.refetch();
      })
      .catch((err) => {
        const code = err instanceof FetchError ? err.code : null;
        if (code === "ALREADY_IN_ROOM") {
          hasJoinedRef.current = true;
          void roomQuery.refetch();
          return;
        }
        skipAutoLeaveRef.current = true;
        toast.error(joinFailureMessage(code, room));
        navigate("/lobby", { replace: true });
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [roomQuery.isSuccess, roomQuery.isFetchedAfterMount, roomQuery.data, id]);

  function joinDeepLinkPrivate(room: Room, password: string) {
    setDeepLinkPasswordError(null);
    joinRoomMutation
      .mutateAsync({ id: room.id, password })
      .then(() => {
        hasJoinedRef.current = true;
        setDeepLinkPrivateRoom(null);
        void roomQuery.refetch();
      })
      .catch((err) => {
        const code = err instanceof FetchError ? err.code : null;
        if (code === "WRONG_ROOM_PASSWORD") {
          setDeepLinkPasswordError("room.errors.wrongPassword");
          return;
        }
        if (code === "ALREADY_IN_ROOM") {
          hasJoinedRef.current = true;
          setDeepLinkPrivateRoom(null);
          void roomQuery.refetch();
          return;
        }
        skipAutoLeaveRef.current = true;
        setDeepLinkPrivateRoom(null);
        toast.error(joinFailureMessage(code, room));
        navigate("/lobby", { replace: true });
      });
  }

  useEffect(() => {
    if (roomQuery.isSuccess && roomQuery.data?.room.isQuickPlay && id) {
      skipAutoLeaveRef.current = true;
      navigate(`/matchmaking/${id}`, { replace: true });
    }
  }, [roomQuery.isSuccess, roomQuery.data, id, navigate]);

  useEffect(() => {
    const prev = prevWsStateRef.current;
    prevWsStateRef.current = wsState;
    if (wsState === "connected" && prev !== "connected") {
      roomQuery.refetch();
    }
  }, [wsState, roomQuery]);

  useEffect(() => {
    if (matchStarted && id) {
      skipAutoLeaveRef.current = true;
      navigate(`/match/${id}`, { replace: true, state: { fromRoom: true } });
    }
  }, [matchStarted, id, navigate]);

  const currentRoomStatus = storeRoom?.status ?? roomQuery.data?.room.status ?? null;
  const isRoomClosed =
    currentRoomStatus === "completed" ||
    currentRoomStatus === "cancelled" ||
    currentRoomStatus === "finished";
  const currentRoomIsQuickPlay =
    storeRoom?.isQuickPlay ?? roomQuery.data?.room.isQuickPlay ?? false;
  const parsedRoomId = id ? Number(id) : null;
  const roomIdNum =
    parsedRoomId !== null && Number.isFinite(parsedRoomId) && parsedRoomId > 0
      ? parsedRoomId
      : undefined;
  const roomIsWaiting = currentRoomStatus === "waiting";
  const lastMatchQuery = useRoomLastMatchQuery(roomIdNum, roomIsWaiting && !currentRoomIsQuickPlay);

  useEffect(() => {
    if (!isRoomClosed || matchStarted) return;
    skipAutoLeaveRef.current = true;
    const timer = setTimeout(() => {
      navigate("/lobby", { replace: true });
    }, 2000);
    return () => clearTimeout(timer);
  }, [isRoomClosed, matchStarted, navigate]);

  useEffect(() => {
    if (kickedFromRoomId !== null && id && kickedFromRoomId === Number(id)) {
      skipAutoLeaveRef.current = true;
      toast.error(t("room.kickedToast", { name: storeRoom?.name ?? "" }));
      useRoomStore.getState().setKickedFromRoom(null);
      returnToLobby();
    }
  }, [kickedFromRoomId, id, returnToLobby, storeRoom?.name, t]);

  useEffect(() => {
    if (roomEjection !== null && id && roomEjection.roomId === Number(id)) {
      skipAutoLeaveRef.current = true;
    }
  }, [roomEjection, id]);

  useEffect(() => {
    if (storeRoom && storeRoom.status !== "waiting") {
      setSwapSourceSeat(null);
    }
  }, [storeRoom]);

  useEffect(() => {
    if (swapSourceSeat !== null && getPlayerAtSeat(storePlayers, swapSourceSeat) === undefined) {
      setSwapSourceSeat(null);
    }
  }, [storePlayers, swapSourceSeat]);

  useEffect(() => {
    return () => {
      useRoomStore.getState().reset();
      useChatStore.getState().clearRoom();
    };
  }, [id]);

  useEffect(() => {
    const roomId = Number(id);
    if (pendingLeaveRef.current && pendingLeaveRef.current.id === roomId) {
      clearTimeout(pendingLeaveRef.current.timer);
      pendingLeaveRef.current = null;
    }
    return () => {
      if (id && hasJoinedRef.current && !skipAutoLeaveRef.current) {
        const timer = setTimeout(() => {
          pendingLeaveRef.current = null;
          leaveRoomMutation.mutate(roomId);
        }, 0);
        pendingLeaveRef.current = { id: roomId, timer };
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  useEffect(() => {
    const onPageHide = () => {
      if (id && hasJoinedRef.current && !skipAutoLeaveRef.current) {
        const token = useAuthStore.getState().token;
        if (token) sendLeaveBeacon(Number(id), token);
      }
    };
    window.addEventListener("pagehide", onPageHide);
    return () => window.removeEventListener("pagehide", onPageHide);
  }, [id]);

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
    skipAutoLeaveRef.current = true;
    try {
      await leaveRoomMutation.mutateAsync(Number(id));
    } catch (err) {
      if (err instanceof FetchError && err.code === "MATCH_ALREADY_STARTED") {
        skipAutoLeaveRef.current = false;
        toast.error(t("room.errors.alreadyStarted"));
        return;
      }
    }
    returnToLobby();
  };

  const handleSelectSeat = async (seatIndex: number) => {
    if (!storeRoom) return;
    if (swapSourceSeat !== null) {
      handleSwapTarget(seatIndex);
      return;
    }
    try {
      const data = await selectSeatMutation.mutateAsync({ roomId: storeRoom.id, seat: seatIndex });
      useRoomStore.getState().setPlayers(data.players);
      if (data.matchStarted) {
        skipAutoLeaveRef.current = true;
        navigate(`/match/${storeRoom.id}`, { replace: true, state: { fromRoom: true } });
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

    const targetPlayer = getPlayerAtSeat(players, targetSeat);
    const sourcePlayer = getPlayerAtSeat(players, swapSourceSeat);
    const sourceIsBot = sourcePlayer?.isBot === true;
    const isCurrentUser =
      targetPlayer !== undefined &&
      targetPlayer.isBot !== true &&
      currentUser !== null &&
      targetPlayer.userId === currentUser.id;
    if (
      targetSeat === swapSourceSeat ||
      (isCurrentUser && !sourceIsBot) ||
      swapSeatsMutation.isPending
    ) {
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

  const handleAddBot = async (seat: number) => {
    if (!storeRoom || addBotMutation.isPending) return;
    try {
      const data = await addBotMutation.mutateAsync({ roomId: storeRoom.id, seat });
      useRoomStore.getState().setPlayers(data.players);
    } catch (err) {
      if (err instanceof FetchError && err.code === "SEAT_TAKEN") {
        toast.error(t("room.seatTaken"));
      } else if (err instanceof FetchError && err.code === "BOTS_NOT_ALLOWED") {
        toast.error(t("room.errors.botsNotAllowed"));
      } else {
        toast.error(t("room.errors.addBotFailed"));
      }
    }
  };

  const handleRemoveBotConfirm = async () => {
    if (!storeRoom || !removeBotConfirm) return;
    const { seat } = removeBotConfirm;
    setRemoveBotConfirm(null);
    try {
      const data = await removeBotMutation.mutateAsync({ roomId: storeRoom.id, seat });
      useRoomStore.getState().setPlayers(data.players);
    } catch (err) {
      if (err instanceof FetchError && err.code === "NO_BOT_ON_SEAT") {
        return;
      }
      toast.error(t("room.errors.removeBotFailed"));
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
      skipAutoLeaveRef.current = true;
      navigate(`/match/${storeRoom.id}`, { replace: true, state: { fromRoom: true } });
    } catch (err) {
      if (err instanceof FetchError && err.code === "NOT_ROOM_OWNER") {
        toast.error(t("room.errors.notOwner"));
      } else if (err instanceof FetchError && err.code === "NOT_ALL_SEATED") {
        toast.error(t("room.errors.notAllSeated"));
      } else if (err instanceof FetchError && err.code === "INSUFFICIENT_COINS") {
        toast.error(t("room.errors.insufficientCoinsStart"));
      } else if (
        err instanceof FetchError &&
        (err.code === "HONOR_TOO_LOW" || err.code === "NEW_PLAYER_NOT_ALLOWED")
      ) {
        toast.error(t("room.errors.honorGateStart"));
      } else {
        toast.error(t("room.errors.startFailed"));
      }
    }
  };

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
        <Button variant="ghost" onClick={() => returnToLobby()} data-testid="back-to-lobby">
          {t("room.notFoundAction")}
        </Button>
      </div>
    );
  }

  if (isRoomClosed && !matchStarted) {
    return (
      <div className="mx-auto max-w-330 px-8 py-12 text-center" data-testid="room-page-closed">
        <p className="text-ink mb-2 text-lg font-semibold">{t("room.roomClosed")}</p>
        <p className="text-ink-dim animate-pulse text-sm">{t("room.returningToLobby")}</p>
      </div>
    );
  }

  const room = storeRoom ?? roomQuery.data!.room;

  if (room.isQuickPlay) return null;

  const players = storePlayers.length > 0 ? storePlayers : roomQuery.data!.players;

  const variantText = variantLabel(t, room.variant);
  const matchModeText = modeLabel(t, room.matchMode);
  const timerLabel =
    room.timerStyle === "relaxed"
      ? t("lobby.card.relaxed")
      : t("lobby.card.timerSeconds", { seconds: room.timerDurationSeconds ?? "?" });

  const isOwner = currentUser !== null && currentUser.id === room.ownerId;
  const canEditPrivacy = isOwner && room.status === "waiting" && !room.isQuickPlay;
  const seatedCount = players.filter((p) => p.seat !== null).length;
  const allSeated = seatedCount === 4;

  const seatedHumans = players.filter((p) => p.seat !== null && p.isBot !== true);
  const presenceTracked = returnedUserIds.length > 0;
  const allReturned =
    !presenceTracked || seatedHumans.every((p) => returnedUserIds.includes(p.userId));
  const isWaitingToReturn = (p: RoomPlayer): boolean =>
    presenceTracked && p.seat !== null && p.isBot !== true && !returnedUserIds.includes(p.userId);

  const isBelowHonorBar = (p: RoomPlayer): boolean =>
    honorRoomIsGated(room) &&
    p.isBot !== true &&
    typeof p.honorScore === "number" &&
    !honorQualifies(room, { honorScore: p.honorScore, isNewPlayer: false });
  const blockedSeats = players.filter(isBelowHonorBar);

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

  const yourPair = viewerSeat !== null && viewerSeat % 2 === 0 ? [0, 2] : [1, 3];
  const otherPair = yourPair[0] === 0 ? [1, 3] : [0, 2];
  const renderSlots = (pair: number[]) => (
    <span className="text-ink-mute inline-flex flex-wrap items-center">
      <span className="text-ink-off" aria-hidden>
        (
      </span>
      <span className="inline-flex flex-wrap items-center gap-1">
        {pair.map((seatIndex, i) => {
          const p = getPlayerAtSeat(players, seatIndex);
          const isYouSlot = p !== undefined && p.isBot !== true && p.userId === currentUser?.id;
          return (
            <span key={seatIndex} className="inline-flex items-center gap-1">
              {i > 0 && <span className="text-ink-off">+</span>}
              {p ? (
                <span className={isYouSlot ? "text-accent font-semibold" : "text-ink-dim"}>
                  {isYouSlot
                    ? t("room.seatYouInline")
                    : p.isBot === true
                      ? botDisplayName(t, p.seat)
                      : p.username}
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

  const rosterTeam = (p: RoomPlayer): "A" | "B" | null => {
    if (p.seat === null || viewerSeat === null) return null;
    return resolveSeatMode(p.seat) === "us" ? "A" : "B";
  };

  const kickTarget = kickConfirm
    ? (players.find((p) => p.userId === kickConfirm.userId) ?? null)
    : null;
  const transferTarget = transferConfirm
    ? (players.find((p) => p.userId === transferConfirm.userId) ?? null)
    : null;

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
    ctaDisabled =
      !allSeated || !allReturned || blockedSeats.length > 0 || startGameMutation.isPending;
    ctaOnClick = ctaDisabled ? undefined : handleStartGame;
    ctaLabel = startGameMutation.isPending
      ? t("room.matchStarting")
      : !allSeated
        ? t("room.waitingForPlayers")
        : !allReturned
          ? t("room.waitingForReturn")
          : t("room.startMatch");
  } else {
    ctaLabel = allSeated ? t("room.waitingForOwner") : t("room.waitingForPlayers");
  }

  return (
    <div className="mx-auto max-w-330 px-4 pt-6 pb-24 md:px-8 md:pb-6">
      <div data-testid="room-page">
        <div
          className="bg-surface border-border relative mb-4 rounded-lg border p-4 shadow-[0_18px_44px_-28px_rgba(14,58,36,0.32),0_2px_0_rgba(255,255,255,0.6)_inset] md:mb-6 md:p-6"
          data-testid="room-info-card"
        >
          <div
            aria-hidden="true"
            className="pointer-events-none absolute -top-px right-7 left-7 h-0.5 bg-[linear-gradient(90deg,transparent,var(--brass)_50%,transparent)] opacity-70"
          />

          <div className="flex items-center gap-3">
            <h1
              className="text-ink m-0 min-w-0 flex-1 truncate text-xl leading-tight font-bold tracking-tight md:text-3xl"
              data-testid="room-info-name"
            >
              {room.name}
            </h1>
            <div className="flex shrink-0 items-center gap-2">
              {canEditPrivacy && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setShowPrivacy(true)}
                  data-testid="room-privacy-control"
                  aria-label={
                    room.isPrivate
                      ? t("room.privacy.manageButtonPrivate")
                      : t("room.privacy.manageButtonPublic")
                  }
                  className="w-7 px-0 whitespace-nowrap md:w-auto md:px-2.5"
                >
                  <Lock className="size-3.5" />
                  <span className="hidden md:inline">
                    {room.isPrivate
                      ? t("room.privacy.manageButtonPrivate")
                      : t("room.privacy.manageButtonPublic")}
                  </span>
                </Button>
              )}
              <CodeChip
                code={room.code}
                variant="compact"
                copied={justCopied}
                onCopy={handleCopyLink}
                ariaLabel={t("room.copyLinkAriaLabel", { code: room.code })}
                testId="copy-link"
                codeTestId="room-code"
                className="px-2.5 py-1 text-xs md:px-3 md:py-2 md:text-sm"
              />
            </div>
          </div>

          <div className="mt-3 flex flex-col gap-3 md:mt-4 md:flex-row md:items-center md:justify-between md:gap-4">
            <div className="text-ink-dim flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs md:text-sm">
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
                  <div className="text-brass-deep px-2 pt-1.5 pb-1 font-mono text-2xs font-semibold tracking-caps uppercase">
                    {t("room.inRoomList.title")}
                  </div>
                  {players.length === 0 ? (
                    <p className="text-ink-mute px-2 py-1.5 text-xs">
                      {t("room.inRoomList.empty")}
                    </p>
                  ) : (
                    <ul className="flex flex-col">
                      {players.map((p) => {
                        const isBotRow = p.isBot === true;
                        const isYou = !isBotRow && currentUser?.id === p.userId;
                        const isRoomOwner = !isBotRow && p.userId === room.ownerId;
                        const isWaiting = room.status === "waiting";
                        const ownerCanActOnRow =
                          isOwner && isWaiting && !isRoomOwner && !isYou && !isBotRow;
                        const ownerCanKickRow = ownerCanActOnRow;
                        const ownerCanPromoteRow = ownerCanActOnRow && p.seat !== null;
                        const seated = p.seat !== null;
                        const rowName = isBotRow ? botDisplayName(t, p.seat) : p.username;
                        const seatLabel = seated
                          ? t("room.inRoomList.seated", {
                              seat: (p.seat as number) + 1,
                            })
                          : t("room.inRoomList.notSeated");
                        return (
                          <li
                            key={isBotRow ? `bot-${p.seat}` : p.userId}
                            className="flex items-center gap-2 px-2 py-1.5 text-sm"
                            data-testid={
                              isBotRow
                                ? `in-room-list-bot-${p.seat}`
                                : `in-room-list-item-${p.userId}`
                            }
                          >
                            <Avatar
                              name={rowName}
                              avatarUrl={isBotRow ? null : p.avatarUrl}
                              size={24}
                              team={rosterTeam(p)}
                              icon={isBotRow ? <Bot aria-hidden="true" /> : undefined}
                            />
                            <span className="text-ink flex min-w-0 flex-1 items-center gap-1">
                              {isRoomOwner && (
                                <Crown className="text-brass-deep size-3 shrink-0" aria-hidden />
                              )}
                              <span className="truncate font-medium">{rowName}</span>
                              {isYou && (
                                <span className="text-accent shrink-0 text-xs font-semibold">
                                  · {t("room.seatYouInline")}
                                </span>
                              )}
                            </span>
                            <span
                              className={cn(
                                "shrink-0 rounded border px-1.5 py-0.5 font-mono text-2xs font-semibold tracking-wider uppercase",
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
                                className="text-ink-mute hover:bg-surface-sunken hover:text-danger rounded p-0.5 transition-colors disabled:cursor-not-allowed disabled:opacity-40"
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
            <div
              className="flex flex-wrap items-center gap-1.5 [&>span]:px-2 [&>span]:py-0.5 [&>span]:text-xs md:justify-end md:gap-2 md:[&>span]:px-2.5 md:[&>span]:py-1 md:[&>span]:text-xs"
              data-testid="room-info-badges"
            >
              <Badge tone="brass" icon={<Trophy className="size-3" />}>
                <span data-testid="badge-variant">{variantText}</span>
              </Badge>
              <Badge tone="brass">
                <span data-testid="badge-match-mode">{matchModeText}</span>
              </Badge>
              {room.declarationsEnabled === false && (
                <Badge tone="neutral" icon={<Ban className="size-3" />}>
                  <span
                    data-testid="badge-no-declarations"
                    role="img"
                    aria-label={t("lobby.card.noDeclarationsAriaLabel")}
                  >
                    {t("lobby.card.noDeclarations")}
                  </span>
                </Badge>
              )}
              {room.stopAtTarget === true && (
                <Badge tone="neutral" icon={<Flag className="size-3" />}>
                  <span
                    data-testid="badge-stop-at-target"
                    role="img"
                    aria-label={t("lobby.card.stopAtTargetAriaLabel")}
                  >
                    {t("lobby.card.stopAtTarget")}
                  </span>
                </Badge>
              )}
              {room.declarationsTransfer === false && (
                <Badge tone="neutral" icon={<Unlink className="size-3" />}>
                  <span
                    data-testid="badge-no-declarations-transfer"
                    role="img"
                    aria-label={t("lobby.card.noDeclarationsTransferAriaLabel")}
                  >
                    {t("lobby.card.noDeclarationsTransfer")}
                  </span>
                </Badge>
              )}
              {room.clockwise === true && (
                <Badge tone="neutral" icon={<RotateCw className="size-3" />}>
                  <span
                    data-testid="badge-clockwise"
                    role="img"
                    aria-label={t("lobby.card.clockwiseAriaLabel")}
                  >
                    {t("lobby.card.clockwise")}
                  </span>
                </Badge>
              )}
              <Badge tone={isRelaxed ? "accent" : "neutral"} icon={<Clock className="size-3" />}>
                <span data-testid="badge-timer">{timerLabel}</span>
              </Badge>
              <Badge tone="brass" icon={<Coins className="size-3" style={{ color: COIN_GOLD }} />}>
                <span data-testid="badge-buy-in">
                  {room.coinBuyIn > 0 ? formatCoins(room.coinBuyIn) : t("lobby.card.buyInFree")}
                </span>
              </Badge>
              {room.minHonor > 0 && (
                <Badge
                  tone="neutral"
                  icon={<HonorShield tier={honorTierForScore(room.minHonor)} size={12} />}
                >
                  <span data-testid="badge-min-honor" data-min-honor={room.minHonor}>
                    {t("lobby.card.minHonor", { minHonor: honorFloorLabel(room.minHonor) })}
                  </span>
                </Badge>
              )}
              {room.allowNewPlayers === false && (
                <Badge tone="neutral" icon={<UserCheck className="size-3" />}>
                  <span data-testid="badge-veterans-only">{t("lobby.card.veteransOnly")}</span>
                </Badge>
              )}
              {room.isQuickPlay && (
                <Badge tone="accent" icon={<Zap className="size-3" />}>
                  <span data-testid="badge-quick-play">{t("lobby.quickPlay")}</span>
                </Badge>
              )}
              {room.isPrivate ? (
                <Badge tone="neutral" icon={<Lock className="size-3" />}>
                  <span data-testid="badge-private">{t("room.privacy.privateBadge")}</span>
                </Badge>
              ) : (
                <Badge tone="neutral" icon={<LockOpen className="size-3" />}>
                  <span data-testid="badge-public">{t("room.privacy.publicBadge")}</span>
                </Badge>
              )}
              {players.some((p) => p.isBot === true) && (
                <Badge tone="bot" icon={<Bot className="size-3" />}>
                  <span data-testid="badge-bots">{t("bots.withBots")}</span>
                </Badge>
              )}
            </div>
          </div>
        </div>

        <div
          className="border-border bg-surface mb-4 flex items-center justify-between gap-2 rounded-lg border px-3 py-2.5 sm:mb-6 sm:gap-3 sm:px-4 sm:py-3"
          data-testid="action-bar"
        >
          <div className="flex min-w-0 items-center gap-2 sm:gap-3">
            <Button
              variant="ghost"
              onClick={handleLeaveRoom}
              data-testid="leave-room"
              aria-label={t("room.leaveRoom")}
              className="text-danger hover:text-danger size-10 shrink-0 px-0 sm:h-8 sm:w-auto sm:px-2.5"
            >
              <DoorOpen className="size-5 sm:size-4" />
              <span className="hidden sm:inline">{t("room.leaveRoom")}</span>
            </Button>
            {room.status === "waiting" && (
              <Button
                variant="ghost"
                onClick={() => setShowInviteFriends(true)}
                data-testid="open-invite-friends"
                aria-label={t("roomInvite.panel.openButton")}
                className="size-10 shrink-0 px-0 sm:h-8 sm:w-auto sm:px-2.5"
              >
                <Send className="size-5 sm:size-4" />
                <span className="hidden sm:inline">{t("roomInvite.panel.openButton")}</span>
              </Button>
            )}
            {roomIsWaiting && lastMatchQuery.data !== undefined && (
              <Button
                variant="ghost"
                onClick={() => setShowLastMatch(true)}
                data-testid="open-last-match"
                aria-label={t("room.lastMatch.openButton")}
                className="size-10 shrink-0 px-0 sm:h-8 sm:w-auto sm:px-2.5"
              >
                <History className="size-5 sm:size-4" />
                <span className="hidden sm:inline">{t("room.lastMatch.openButton")}</span>
              </Button>
            )}
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
            title={
              !inSwapMode && isOwner && blockedSeats.length > 0
                ? t("room.startBlockedByHonor", { minHonor: room.minHonor })
                : !inSwapMode && isOwner && !allSeated
                  ? t("room.startMatchDisabled")
                  : undefined
            }
            data-testid={ctaTestId}
            className="h-9 min-w-0 shrink px-3.5 text-sm sm:h-11.5 sm:px-5.5 sm:text-sm"
          >
            <span className="min-w-0 truncate">{ctaLabel}</span>
          </Button>
        </div>

        <div
          className="border-brass bg-surface mb-3 rounded-lg border border-dashed px-3 py-2 sm:px-4 sm:py-3"
          data-testid="team-legend"
        >
          {viewerSeat === null ? (
            <div className="flex items-center justify-center gap-2 sm:gap-2.5">
              <LegendIndicator variant="neutral" />
              <span className="text-ink-dim text-xs sm:text-xs">{t("room.legendNeutral")}</span>
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1.5 text-xs sm:gap-x-4 sm:gap-y-2 sm:text-xs">
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

        <div className="relative mb-4 sm:mb-6" data-testid="seats-grid">
          <div aria-hidden className="pointer-events-none absolute inset-0 z-0 hidden sm:block">
            <div
              className="absolute top-[12%] left-1/2 h-[76%] w-0.5 -translate-x-1/2 opacity-50"
              style={{
                background:
                  viewerSeat === null
                    ? "linear-gradient(180deg, transparent 0%, var(--brass) 25%, var(--brass) 75%, transparent 100%)"
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
                    ? "linear-gradient(90deg, transparent 0%, var(--brass) 25%, var(--brass) 75%, transparent 100%)"
                    : viewerSeat % 2 === 1
                      ? "linear-gradient(90deg, transparent 0%, var(--team-a-edge) 20%, var(--team-a-edge) 80%, transparent 100%)"
                      : "linear-gradient(90deg, transparent 0%, var(--team-b-edge-soft) 20%, var(--team-b-edge-soft) 80%, transparent 100%)",
              }}
            />
          </div>

          <div aria-hidden className="pointer-events-none absolute inset-0 z-0 block sm:hidden">
            <div
              className="absolute top-1/4 left-[12%] h-0.5 w-[76%] -translate-y-1/2 opacity-50"
              style={{
                background:
                  viewerSeat === null
                    ? "linear-gradient(90deg, transparent 0%, var(--brass) 25%, var(--brass) 75%, transparent 100%)"
                    : viewerSeat % 2 === 0
                      ? "linear-gradient(90deg, transparent 0%, var(--team-a-edge) 20%, var(--team-a-edge) 80%, transparent 100%)"
                      : "linear-gradient(90deg, transparent 0%, var(--team-b-edge-soft) 20%, var(--team-b-edge-soft) 80%, transparent 100%)",
              }}
            />
            <div
              className="absolute top-3/4 left-[12%] h-0.5 w-[76%] -translate-y-1/2 opacity-50"
              style={{
                background:
                  viewerSeat === null
                    ? "linear-gradient(90deg, transparent 0%, var(--brass) 25%, var(--brass) 75%, transparent 100%)"
                    : viewerSeat % 2 === 1
                      ? "linear-gradient(90deg, transparent 0%, var(--team-a-edge) 20%, var(--team-a-edge) 80%, transparent 100%)"
                      : "linear-gradient(90deg, transparent 0%, var(--team-b-edge-soft) 20%, var(--team-b-edge-soft) 80%, transparent 100%)",
              }}
            />
          </div>

          <div className="relative z-10 grid grid-cols-2 gap-3 sm:grid-cols-[1fr_1fr_1fr] sm:gap-5 sm:[grid-template-areas:'._north_.''west_center_east''._south_.']">
            {SEAT_RENDER_ORDER.map((seatIndex) => {
              const player = getPlayerAtSeat(players, seatIndex);
              const cardinal = SEAT_TO_CARDINAL[seatIndex] as CardinalPosition;
              const isBotSeat = player?.isBot === true;
              const isCurrentUser =
                player !== undefined &&
                !isBotSeat &&
                currentUser !== null &&
                player.userId === currentUser.id;
              const isSeatOwner =
                player !== undefined && !isBotSeat && player.userId === room.ownerId;
              const isWaiting = room.status === "waiting";
              const ownerCanKick =
                isOwner && isWaiting && player !== undefined && !isSeatOwner && !isBotSeat;
              const ownerCanRemoveBot = isOwner && isWaiting && isBotSeat;
              const ownerCanAddBot =
                isOwner && isWaiting && player === undefined && !room.isQuickPlay;
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
              const botPendingForThisSeat =
                (addBotMutation.isPending && addBotMutation.variables?.seat === seatIndex) ||
                (removeBotMutation.isPending && removeBotMutation.variables?.seat === seatIndex);
              const isPendingForThisSeat =
                kickPendingForThisSeat ||
                swapPendingForThisSeat ||
                leaveSeatPendingForThisSeat ||
                botPendingForThisSeat;

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
                  waitingToReturn={isWaiting && player !== undefined && isWaitingToReturn(player)}
                  belowHonorBar={isWaiting && player !== undefined && isBelowHonorBar(player)}
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
                  onAddBot={ownerCanAddBot ? () => handleAddBot(seatIndex) : undefined}
                  onRemoveBot={
                    ownerCanRemoveBot
                      ? () => {
                          setSwapSourceSeat(null);
                          setRemoveBotConfirm({ seat: seatIndex });
                        }
                      : undefined
                  }
                />
              );
            })}

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
                  boxShadow: "0 6px 18px -6px rgba(14,58,36,0.40), inset 0 0 0 4px var(--brass-10)",
                }}
              >
                <span
                  className="font-mono text-2xs font-semibold tracking-caps uppercase"
                  style={{ transform: "rotate(-45deg)", color: "var(--brass)" }}
                >
                  {t("room.tableCenter")}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <RoomChatDock roomId={room.id} />

      <TransferOwnershipDialog
        open={transferConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setTransferConfirm(null);
        }}
        onConfirm={handleTransferConfirm}
        pending={transferOwnershipMutation.isPending}
        fromName={currentUser?.username ?? ownerUsername}
        fromAvatarUrl={currentUser?.avatarUrl ?? null}
        target={{
          name: transferConfirm?.username ?? "",
          seat: transferTarget?.seat ?? null,
          team: transferTarget ? rosterTeam(transferTarget) : null,
          avatarUrl: transferTarget?.avatarUrl ?? null,
        }}
      />

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
          avatarUrl: kickTarget?.avatarUrl ?? null,
        }}
      />

      <RemoveBotDialog
        open={removeBotConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setRemoveBotConfirm(null);
        }}
        onConfirm={handleRemoveBotConfirm}
        pending={removeBotMutation.isPending}
        target={{
          name: removeBotConfirm !== null ? botDisplayName(t, removeBotConfirm.seat) : "",
          seat: removeBotConfirm?.seat ?? null,
          team:
            removeBotConfirm !== null && viewerSeat !== null
              ? resolveSeatMode(removeBotConfirm.seat) === "us"
                ? "A"
                : "B"
              : null,
        }}
      />

      <InviteFriendsDialog
        open={showInviteFriends}
        roomId={room.id}
        onOpenChange={setShowInviteFriends}
      />

      <LastMatchDialog
        open={showLastMatch}
        roomId={room.id}
        onOpenChange={setShowLastMatch}
        playersInRoom={players.map((p) => p.userId)}
      />

      <RoomPrivacyDialog open={showPrivacy} room={room} onClose={() => setShowPrivacy(false)} />

      <PasswordPromptDialog
        open={deepLinkPrivateRoom !== null}
        roomName={deepLinkPrivateRoom?.name ?? ""}
        pending={joinRoomMutation.isPending}
        errorKey={deepLinkPasswordError}
        onSubmit={(password) => {
          if (deepLinkPrivateRoom) joinDeepLinkPrivate(deepLinkPrivateRoom, password);
        }}
        onClose={() => {
          setDeepLinkPrivateRoom(null);
          setDeepLinkPasswordError(null);
          skipAutoLeaveRef.current = true;
          navigate("/lobby", { replace: true });
        }}
      />
    </div>
  );
}
