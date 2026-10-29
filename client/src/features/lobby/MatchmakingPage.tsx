import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";

import { MatchmakingDiagram } from "@/features/lobby/components/MatchmakingDiagram";
import { FetchError } from "@/shared/api/axiosClient";
import { Skeleton } from "@/shared/components/ui/skeleton";
import { useLeaveRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { useRoomDetailQuery } from "@/shared/hooks/queries/useRooms";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useWsConnectionState } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useRoomStore } from "@/shared/stores/roomStore";

function formatElapsed(totalSeconds: number): string {
  const mm = Math.floor(totalSeconds / 60);
  const ss = totalSeconds % 60;
  return `${String(mm).padStart(2, "0")}:${String(ss).padStart(2, "0")}`;
}

/** The Quick Play waiting screen (`/matchmaking/:id`), which navigates everyone to the game once the last seat fills. */
export function MatchmakingPage() {
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

  const hasLeftRef = useRef(false);
  const hasJoinedRef = useRef(false);

  const fetchStartedRef = useRef(false);
  const [freshFetchSettled, setFreshFetchSettled] = useState(false);
  useEffect(() => {
    if (roomQuery.isFetching) {
      fetchStartedRef.current = true;
    } else if (fetchStartedRef.current) {
      setFreshFetchSettled(true);
    }
  }, [roomQuery.isFetching]);

  const leaveRoomMutation = useLeaveRoomMutation();

  const startRef = useRef(Date.now());
  const [elapsedSec, setElapsedSec] = useState(0);
  useEffect(() => {
    const tick = () => setElapsedSec(Math.floor((Date.now() - startRef.current) / 1000));
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, []);

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

  useEffect(() => {
    const prev = prevWsStateRef.current;
    prevWsStateRef.current = wsState;
    if (wsState === "connected" && prev !== "connected") {
      roomQuery.refetch();
    }
  }, [wsState, roomQuery]);

  useEffect(() => {
    if (matchStarted && id) {
      hasLeftRef.current = true;
      navigate(`/match/${id}`, { replace: true, state: { fromRoom: true } });
    }
  }, [matchStarted, id, navigate]);

  useEffect(() => {
    if (roomQuery.isError) {
      hasLeftRef.current = true;
      navigate("/lobby", { replace: true });
      return;
    }
    if (!freshFetchSettled || !roomQuery.data) return;
    const r = roomQuery.data.room;
    if (!r.isQuickPlay) {
      hasLeftRef.current = true;
      navigate(`/rooms/${id}`, { replace: true });
      return;
    }
    if (!currentUser) return;
    const isMember = roomQuery.data.players.some((p) => p.userId === currentUser.id);
    if (!isMember) {
      hasLeftRef.current = true;
      navigate("/lobby", { replace: true });
    }
  }, [roomQuery.isError, freshFetchSettled, roomQuery.data, id, navigate, currentUser]);

  const status = storeRoom?.status ?? roomQuery.data?.room.status ?? null;
  const isClosed = status === "completed" || status === "cancelled" || status === "finished";
  useEffect(() => {
    if (status === "playing" && id && hasJoinedRef.current) {
      hasLeftRef.current = true;
      navigate(`/match/${id}`, { replace: true, state: { fromRoom: true } });
    }
  }, [status, id, navigate]);
  useEffect(() => {
    if (isClosed && !matchStarted) {
      hasLeftRef.current = true;
      navigate("/lobby", { replace: true });
    }
  }, [isClosed, matchStarted, navigate]);

  useEffect(() => {
    if (kickedFromRoomId !== null && id && kickedFromRoomId === Number(id)) {
      hasLeftRef.current = true;
      useRoomStore.getState().setKickedFromRoom(null);
      returnToLobby();
    }
  }, [kickedFromRoomId, id, returnToLobby]);

  useEffect(() => {
    return () => {
      useRoomStore.getState().reset();
    };
  }, [id]);

  useEffect(() => {
    return () => {
      if (id && hasJoinedRef.current && !hasLeftRef.current) {
        leaveRoomMutation.mutate(Number(id));
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const handleCancel = async () => {
    hasLeftRef.current = true;
    try {
      await leaveRoomMutation.mutateAsync(Number(id));
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      if (code === "MATCH_ALREADY_STARTED") {
        hasLeftRef.current = false;
        toast.error(t("room.errors.alreadyStarted"));
        return;
      }
      if (code !== "NOT_IN_ROOM" && code !== "ROOM_NOT_FOUND") {
        hasLeftRef.current = false;
        toast.error(t("lobby.matchmaking.errors.cancelFailed"));
        return;
      }
    }
    returnToLobby();
  };

  if (roomQuery.isLoading || !freshFetchSettled || (!storeRoom && !roomQuery.data)) {
    return (
      <div
        className="mx-auto flex min-h-[60vh] max-w-330 items-center justify-center px-7 py-10"
        data-testid="matchmaking-loading"
      >
        <Skeleton className="size-24 rounded-full" />
      </div>
    );
  }

  const room = storeRoom ?? roomQuery.data?.room;
  if (!room || !room.isQuickPlay) return null;

  const players = storePlayers.length > 0 ? storePlayers : (roomQuery.data?.players ?? []);
  const viewerPlayer = players.find((p) => p.userId === currentUser?.id);
  const viewerSeat = viewerPlayer?.seat ?? null;
  if (viewerSeat === null) return null;

  const currentUsername = viewerPlayer?.username ?? currentUser?.username ?? "";
  const found = players.filter((p) => p.seat !== null).length;

  return (
    <div
      className="mx-auto flex min-h-[calc(100vh-160px)] max-w-330 flex-col items-center justify-center px-7 py-6 md:py-10"
      data-testid="matchmaking-page"
    >
      <MatchmakingDiagram
        room={room}
        found={found}
        players={players}
        viewerSeat={viewerSeat}
        currentUsername={currentUsername}
        currentAvatarUrl={currentUser?.avatarUrl ?? null}
        elapsed={formatElapsed(elapsedSec)}
        onCancel={handleCancel}
        cancelDisabled={leaveRoomMutation.isPending}
      />
    </div>
  );
}
