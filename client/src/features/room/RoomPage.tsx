import { ArrowLeftRight, Check, DoorOpen, UserPlus, Users } from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { InviteFriendsDialog, InvitePlayers } from "@/features/room/components/InviteFriendsDialog";
import { PlayerActionsDialog } from "@/features/room/components/PlayerActionsDialog";
import { RoomChatDock } from "@/features/room/components/RoomChatDock";
import { PhoneRoomCard, WideRoomHeader } from "@/features/room/components/RoomHeader";
import { LastMatchOutcome, RoomLastMatch } from "@/features/room/components/RoomLastMatch";
import {
  RoomRoster,
  RoomRosterDialog,
  type RoomRosterProps,
} from "@/features/room/components/RoomRoster";
import { RoomTable } from "@/features/room/components/RoomTable";
import { cardinalFor, type TableSeat } from "@/features/room/lib/tableSeat";
import { useHorizontalSwipe } from "@/features/room/lib/useHorizontalSwipe";
import {
  KickPlayerDialog,
  RemoveBotDialog,
  TransferOwnershipDialog,
} from "@/features/room/OwnerConfirmDialogs";
import { RoomPrivacyDialog } from "@/features/room/RoomPrivacyDialog";
import { FetchError } from "@/shared/api/axiosClient";
import { PageShell } from "@/shared/components/layout/PageShell";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { Panel } from "@/shared/components/ui/panel";
import { Segmented } from "@/shared/components/ui/segmented";
import { Skeleton } from "@/shared/components/ui/skeleton";
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
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { botDisplayName } from "@/shared/lib/botName";
import { honorQualifies, honorRoomIsGated } from "@/shared/lib/honor";
import { joinFailureMessage as sharedJoinFailureMessage } from "@/shared/lib/joinFailure";
import { cn } from "@/shared/lib/utils";
import { useWsConnectionState } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useChatStore } from "@/shared/stores/chatStore";
import { useRoomStore } from "@/shared/stores/roomStore";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

type RoomTab = "table" | "last";

const VIEW_ORDER = ["south", "east", "north", "west"] as const;

const SCROLL_LIST = "overflow-y-auto overscroll-contain pr-1 [scrollbar-width:thin]";

function getPlayerAtSeat(players: RoomPlayer[], seatIndex: number): RoomPlayer | undefined {
  return players.find((p) => p.seat === seatIndex);
}

interface RoomCta {
  label: string;
  disabled: boolean;
  testId: string;
  onClick?: () => void;
  title?: string;
  waitingFor?: RoomPlayer;
}

/** The one action the viewer has, as the phone's bottom-bar button or the centre of the wide table. */
function CtaButton({ cta, className }: { cta: RoomCta; className?: string }) {
  return (
    <Button
      size="cta"
      onClick={cta.onClick}
      disabled={cta.disabled}
      title={cta.title}
      data-testid={cta.testId}
      className={cn("min-w-0", className)}
    >
      {cta.waitingFor !== undefined && (
        <Avatar name={cta.waitingFor.username} avatarUrl={cta.waitingFor.avatarUrl} size={22} />
      )}
      <span className="min-w-0 truncate">{cta.label}</span>
    </Button>
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
  const [tab, setTab] = useState<RoomTab>("table");
  const [playerTarget, setPlayerTarget] = useState<number | null>(null);
  const [showRoster, setShowRoster] = useState(false);
  const band = useLayoutBand();
  const swipe = useHorizontalSwipe(
    () => setTab("last"),
    () => setTab("table"),
  );

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
      <PageShell testId="room-page-loading">
        <Skeleton className="h-28 w-full rounded-xl" />
        <Skeleton className="h-80 w-full rounded-[2rem]" />
      </PageShell>
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

  const isOwner = currentUser !== null && currentUser.id === room.ownerId;
  const isWaiting = room.status === "waiting";
  const canEditPrivacy = isOwner && isWaiting && !room.isQuickPlay;
  const seatedCount = players.filter((p) => p.seat !== null).length;
  const allSeated = seatedCount === 4;

  const seatedHumans = players.filter((p) => p.seat !== null && p.isBot !== true);
  const presenceTracked = returnedUserIds.length > 0;
  const isWaitingToReturn = (p: RoomPlayer): boolean =>
    presenceTracked && p.seat !== null && p.isBot !== true && !returnedUserIds.includes(p.userId);
  const firstAway = seatedHumans.find(isWaitingToReturn);
  const allReturned = firstAway === undefined;

  const isBelowHonorBar = (p: RoomPlayer): boolean =>
    honorRoomIsGated(room) &&
    p.isBot !== true &&
    typeof p.honorScore === "number" &&
    !honorQualifies(room, { honorScore: p.honorScore, isNewPlayer: false });
  const blockedSeats = players.filter(isBelowHonorBar);

  const ownerPlayer = players.find((p) => p.userId === room.ownerId);
  const ownerUsername = ownerPlayer?.username ?? t("room.seatOwner");
  const viewerPlayer = currentUser ? players.find((p) => p.userId === currentUser.id) : undefined;
  const viewerSeat = viewerPlayer?.seat ?? null;
  const inSwapMode = swapSourceSeat !== null;
  const phone = band === "phone";
  const canInvite = isWaiting && !allSeated;

  const teamOf = (seat: number): "A" | "B" | null =>
    viewerSeat === null ? null : viewerSeat % 2 === seat % 2 ? "A" : "B";

  const table: TableSeat[] = ([0, 1, 2, 3] as const).map((seat) => {
    const player = getPlayerAtSeat(players, seat);
    const isBot = player?.isBot === true;
    const isYou = player !== undefined && !isBot && player.userId === currentUser?.id;
    const isHost = player !== undefined && !isBot && player.userId === room.ownerId;
    const canLeaveOwnSeat = isYou && isWaiting && !isHost && !inSwapMode;
    const hostMenu = isOwner && isWaiting && player !== undefined && !isYou && !inSwapMode;
    const pending =
      (kickPlayerMutation.isPending &&
        player !== undefined &&
        kickPlayerMutation.variables?.userId === player.userId) ||
      (swapSeatsMutation.isPending &&
        (swapSeatsMutation.variables?.seatA === seat ||
          swapSeatsMutation.variables?.seatB === seat)) ||
      (leaveSeatMutation.isPending && isYou) ||
      (addBotMutation.isPending && addBotMutation.variables?.seat === seat) ||
      (removeBotMutation.isPending && removeBotMutation.variables?.seat === seat);
    return {
      seat,
      cardinal: cardinalFor(seat, viewerSeat),
      team: teamOf(seat),
      player,
      name: player === undefined ? "" : isBot ? botDisplayName(t, seat) : player.username,
      isYou,
      isHost,
      returning: isWaiting && player !== undefined && isWaitingToReturn(player),
      belowHonorBar: isWaiting && player !== undefined && isBelowHonorBar(player),
      swapSource: swapSourceSeat === seat,
      pending,
      clickable: inSwapMode || player === undefined || canLeaveOwnSeat || hostMenu,
      canAddBot: isOwner && isWaiting && player === undefined && !room.isQuickPlay,
      hostMenu,
    };
  });
  const tableFromViewer = [...table].sort(
    (a, b) => VIEW_ORDER.indexOf(a.cardinal) - VIEW_ORDER.indexOf(b.cardinal),
  );
  const standing = players.filter((p) => p.seat === null && p.isBot !== true);
  const inRoom = players.filter((p) => p.isBot !== true).length;

  const handleSeat = (seat: number) => {
    if (inSwapMode) {
      handleSwapTarget(seat);
      return;
    }
    const target = table[seat];
    if (target === undefined) return;
    if (target.player === undefined) {
      handleSelectSeat(seat);
      return;
    }
    if (target.isYou) {
      if (target.clickable) handleLeaveSeat();
      return;
    }
    if (target.hostMenu) setPlayerTarget(seat);
  };

  const actionTarget = playerTarget !== null ? table[playerTarget] : undefined;

  let cta: RoomCta;
  if (isOwner) {
    const waiting = allSeated && !allReturned ? firstAway : undefined;
    cta = {
      testId: "start-game",
      disabled:
        inSwapMode ||
        !allSeated ||
        !allReturned ||
        blockedSeats.length > 0 ||
        startGameMutation.isPending,
      label: startGameMutation.isPending
        ? t("room.matchStarting")
        : waiting !== undefined
          ? t("room.waitingFor", { name: waiting.username })
          : t("room.startMatch"),
      title:
        blockedSeats.length > 0
          ? t("room.startBlockedByHonor", { minHonor: room.minHonor })
          : undefined,
      waitingFor: waiting,
    };
    cta.onClick = cta.disabled ? undefined : handleStartGame;
  } else {
    const waiting = !allSeated ? undefined : (firstAway ?? ownerPlayer);
    cta = {
      testId: "room-cta",
      disabled: true,
      label:
        waiting === undefined
          ? t("room.waitingForPlayers")
          : t("room.waitingFor", { name: waiting.username }),
      waitingFor: waiting,
    };
  }

  const lastMatch = isWaiting ? lastMatchQuery.data : undefined;
  const showLast = tab === "last" && lastMatch !== undefined;

  const rosterProps: RoomRosterProps = {
    table: tableFromViewer,
    standing,
    viewerId: currentUser?.id ?? null,
    ownerId: room.ownerId,
    canManage: isOwner && isWaiting,
    onPromote: (p) => setTransferConfirm({ userId: p.userId, username: p.username }),
    onKick: (p) => {
      setSwapSourceSeat(null);
      setKickConfirm({ seat: p.seat ?? -1, userId: p.userId, username: p.username });
    },
    onRemoveBot: (seat) => {
      setSwapSourceSeat(null);
      setRemoveBotConfirm({ seat });
    },
  };

  const headerProps = {
    room,
    players,
    ownerName: ownerUsername,
    canEditPrivacy,
    copied: justCopied,
    onCopy: handleCopyLink,
    onPrivacy: () => setShowPrivacy(true),
  };

  const center: ReactNode = phone ? (
    <div
      className={cn(
        "bg-surface-elevated flex size-20 flex-col items-center justify-center rounded-full border shadow-sm",
        allSeated ? "border-accent" : "border-brass/50",
      )}
      data-testid="table-center"
    >
      {allSeated ? (
        <Check className="text-accent size-6" aria-hidden="true" />
      ) : (
        <span className="text-ink text-xl font-bold tabular-nums" data-testid="seated-count">
          {seatedCount}
          <span className="text-ink-mute text-sm">/4</span>
        </span>
      )}
      <span className="text-ink-mute max-w-17 text-center text-2xs leading-tight">
        {allSeated ? t("room.table.ready") : t("room.table.seated")}
      </span>
    </div>
  ) : (
    <div className="flex flex-col items-center gap-2 py-4 text-center" data-testid="table-center">
      <span className="text-ink text-2xl font-bold tabular-nums" data-testid="seated-count">
        {seatedCount}
        <span className="text-ink-mute text-base">/4</span>
      </span>
      <span className="text-ink-mute font-mono text-2xs tracking-caps uppercase">
        {t("room.table.seated")}
      </span>
      <CtaButton cta={cta} />
    </div>
  );

  const sidePanels = (
    <>
      {canInvite && (
        <Panel icon={<UserPlus />} title={t("roomInvite.panel.title")} testId="room-invite-panel">
          <InvitePlayers roomId={room.id} active listClassName="max-h-56" />
        </Panel>
      )}
      <Panel
        icon={<Users />}
        title={t("room.roster.title")}
        aside={<span className="text-ink-mute">{inRoom}</span>}
        testId="room-roster-panel"
      >
        <RoomRoster {...rosterProps} className={cn("max-h-80", SCROLL_LIST)} />
      </Panel>
    </>
  );

  const bottomBar = (
    <div className="flex items-center gap-2" data-testid="action-bar">
      <Button
        variant="outline"
        size="icon"
        onClick={handleLeaveRoom}
        aria-label={t("room.leaveRoom")}
        data-testid="leave-room"
        className="text-danger border-danger/30 hover:text-danger size-11 shrink-0 rounded-xl"
      >
        <DoorOpen className="size-5" />
      </Button>
      <CtaButton cta={cta} className="h-11 flex-1 text-base" />
    </div>
  );

  return (
    <PageShell
      testId="room-page"
      sidebar={band === "desktop" ? sidePanels : undefined}
      bottomBar={phone ? bottomBar : undefined}
    >
      {phone ? (
        <PhoneRoomCard {...headerProps} inRoom={inRoom} onRoster={() => setShowRoster(true)} />
      ) : (
        <WideRoomHeader {...headerProps} onLeave={handleLeaveRoom} />
      )}

      {lastMatch !== undefined && (
        <Segmented<RoomTab>
          value={showLast ? "last" : "table"}
          onValueChange={setTab}
          ariaLabel={t("room.tabs.label")}
          testId="room-tabs"
          className={phone ? undefined : "w-80"}
          options={[
            { value: "table", label: t("room.tabs.table") },
            {
              value: "last",
              label: (
                <span className="inline-flex items-center gap-1.5">
                  {t("room.lastMatch.title")}
                  <LastMatchOutcome outcome={lastMatch.outcome} />
                </span>
              ),
            },
          ]}
        />
      )}

      <div className="flex flex-col gap-3.5" {...(phone && lastMatch !== undefined ? swipe : {})}>
        {showLast ? (
          <RoomLastMatch
            match={lastMatch}
            roomId={room.id}
            playersInRoom={players.map((p) => p.userId)}
            compact={band !== "desktop"}
          />
        ) : (
          <>
            {inSwapMode && (
              <div
                className="bg-accent-soft text-accent-deep flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-semibold"
                data-testid="swap-mode-banner"
              >
                <ArrowLeftRight className="size-4 shrink-0" aria-hidden="true" />
                <span className="min-w-0 flex-1">{t("room.swap.pick")}</span>
                <Button size="sm" variant="ghost" onClick={() => setSwapSourceSeat(null)}>
                  {t("common.cancel")}
                </Button>
              </div>
            )}
            <RoomTable
              seats={table}
              layout={phone ? "compact" : "wide"}
              swapMode={inSwapMode}
              center={center}
              onSeat={handleSeat}
              onAddBot={handleAddBot}
            />
            {phone && canInvite && !inSwapMode && (
              <Button
                variant="outline"
                onClick={() => setShowInviteFriends(true)}
                data-testid="invite-open"
                className="h-10"
              >
                <UserPlus className="size-4" />
                {t("roomInvite.panel.title")}
              </Button>
            )}
            {phone && standing.length > 0 && (
              <button
                type="button"
                onClick={() => setShowRoster(true)}
                className="text-ink-mute flex min-w-0 cursor-pointer items-center gap-2 px-1 text-xs"
                data-testid="not-seated-line"
              >
                <Users className="size-3.5 shrink-0" aria-hidden="true" />
                <span className="shrink-0">{t("room.roster.notSeatedShort")}</span>
                {standing.map((p) => (
                  <span key={p.userId} className="inline-flex min-w-0 items-center gap-1.5">
                    <Avatar name={p.username} avatarUrl={p.avatarUrl} size={20} />
                    <span className="text-ink truncate font-medium">{p.username}</span>
                  </span>
                ))}
              </button>
            )}
          </>
        )}
      </div>

      {band === "tablet" && <div className="grid grid-cols-2 items-start gap-4">{sidePanels}</div>}

      <RoomChatDock roomId={room.id} aboveBottomBar={phone} />

      {phone && (
        <RoomRosterDialog {...rosterProps} open={showRoster} onClose={() => setShowRoster(false)} />
      )}

      <PlayerActionsDialog
        target={
          actionTarget?.player !== undefined
            ? { seat: actionTarget.seat, name: actionTarget.name, player: actionTarget.player }
            : null
        }
        canPromote={actionTarget?.player !== undefined && actionTarget.player.isBot !== true}
        onClose={() => setPlayerTarget(null)}
        onSwap={() => {
          if (playerTarget !== null) setSwapSourceSeat(playerTarget);
          setPlayerTarget(null);
        }}
        onPromote={() => {
          const p = actionTarget?.player;
          if (p) setTransferConfirm({ userId: p.userId, username: p.username });
          setPlayerTarget(null);
        }}
        onKick={() => {
          const p = actionTarget?.player;
          if (p) setKickConfirm({ seat: p.seat ?? -1, userId: p.userId, username: p.username });
          setPlayerTarget(null);
        }}
        onRemoveBot={() => {
          if (playerTarget !== null) setRemoveBotConfirm({ seat: playerTarget });
          setPlayerTarget(null);
        }}
      />

      <TransferOwnershipDialog
        open={transferConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setTransferConfirm(null);
        }}
        onConfirm={handleTransferConfirm}
        pending={transferOwnershipMutation.isPending}
        name={transferConfirm?.username ?? ""}
      />

      <KickPlayerDialog
        open={kickConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setKickConfirm(null);
        }}
        onConfirm={handleKickConfirm}
        pending={kickPlayerMutation.isPending}
        name={kickConfirm?.username ?? ""}
      />

      <RemoveBotDialog
        open={removeBotConfirm !== null}
        onOpenChange={(open) => {
          if (!open) setRemoveBotConfirm(null);
        }}
        onConfirm={handleRemoveBotConfirm}
        pending={removeBotMutation.isPending}
        name={removeBotConfirm !== null ? botDisplayName(t, removeBotConfirm.seat) : ""}
      />

      <InviteFriendsDialog
        open={showInviteFriends}
        roomId={room.id}
        onOpenChange={setShowInviteFriends}
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
    </PageShell>
  );
}
