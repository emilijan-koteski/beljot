import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";

import { MatchmakingDiagram } from "@/features/lobby/components/MatchmakingDiagram";
import { FetchError } from "@/shared/api/axiosClient";
import { useLeaveRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { useRoomDetailQuery } from "@/shared/hooks/queries/useRooms";
import { useWsConnectionState } from "@/shared/providers/WebSocketContext";
import { useAuthStore } from "@/shared/stores/authStore";
import { useRoomStore } from "@/shared/stores/roomStore";

function formatElapsed(totalSeconds: number): string {
  const mm = Math.floor(totalSeconds / 60);
  const ss = totalSeconds % 60;
  return `${String(mm).padStart(2, "0")}:${String(ss).padStart(2, "0")}`;
}

/**
 * The Quick Play waiting screen (`/matchmaking/:id`). Reuses RoomPage's
 * data/realtime plumbing — `useRoomDetailQuery` seeds `roomStore`, WS
 * events flow into the store via `useWsDispatch`, and the page derives the
 * seated count for the orbit. When the last seat fills the server broadcasts
 * `system:match_started`, flipping the store flag that navigates everyone to the
 * game. Unlike RoomPage this surface never shows a seat grid: it's the
 * dedicated matchmaking experience for `isQuickPlay` rooms.
 */
export function MatchmakingPage() {
  const { id } = useParams<{ id: string }>();
  const { t } = useTranslation();
  const navigate = useNavigate();
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

  // Tracks whether a fetch has actually STARTED and COMPLETED since this mount.
  // We must not judge membership (or redirect) on a stale cached snapshot —
  // `useRoomDetailQuery` refetches on mount, and only once that fresh fetch
  // settles is the players list guaranteed to include a just-joined viewer.
  // (`isFetchedAfterMount` is unreliable here: it reads true on pre-seeded
  // cache before the refetch resolves.)
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

  // Client-side elapsed timer. Cosmetic — the backend has no per-player queue
  // timestamp, so this resets on refresh (acceptable: there's no SLA on it).
  const startRef = useRef(Date.now());
  const [elapsedSec, setElapsedSec] = useState(0);
  useEffect(() => {
    const tick = () => setElapsedSec(Math.floor((Date.now() - startRef.current) / 1000));
    const interval = setInterval(tick, 1000);
    return () => clearInterval(interval);
  }, []);

  // Seed roomStore from the REST fetch. setCurrentRoomId is mandatory —
  // useWsDispatch drops every room event whose roomId !== store.currentRoomId,
  // so without it the orbit would never fill.
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

  // Refetch after a WebSocket reconnect to catch events missed while offline.
  useEffect(() => {
    const prev = prevWsStateRef.current;
    prevWsStateRef.current = wsState;
    if (wsState === "connected" && prev !== "connected") {
      roomQuery.refetch();
    }
  }, [wsState, roomQuery]);

  // Match started — navigate everyone to the game with the starting splash.
  useEffect(() => {
    if (matchStarted && id) {
      hasLeftRef.current = true;
      navigate(`/match/${id}`, { state: { fromRoom: true } });
    }
  }, [matchStarted, id, navigate]);

  // Cross-redirect guards, gated on a FRESH post-mount fetch (never the lagging
  // store or a stale cached snapshot). A custom room belongs on /rooms/:id; a
  // room the viewer isn't actually in, or a 404, sends them back to the lobby.
  // Gating on `freshFetchSettled` is critical: re-entering a recently visited
  // room serves cached data that omits the just-joined viewer, and acting on it
  // fires a spurious "not a member" redirect that strands the player in the
  // room. hasLeftRef is set first so the unmount auto-leave never fires on exit.
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
    // Don't judge membership until the auth user is known — a transient null
    // (e.g. mid-logout) would otherwise read as "not a member" and redirect.
    if (!currentUser) return;
    const isMember = roomQuery.data.players.some((p) => p.userId === currentUser.id);
    if (!isMember) {
      hasLeftRef.current = true;
      navigate("/lobby", { replace: true });
    }
  }, [roomQuery.isError, freshFetchSettled, roomQuery.data, id, navigate, currentUser]);

  // Closed-room (owner left and emptied it, or it was cancelled) → lobby.
  const status = storeRoom?.status ?? roomQuery.data?.room.status ?? null;
  const isClosed = status === "completed" || status === "cancelled" || status === "finished";
  useEffect(() => {
    if (isClosed && !matchStarted) {
      hasLeftRef.current = true;
      navigate("/lobby", { replace: true });
    }
  }, [isClosed, matchStarted, navigate]);

  // Kicked (defensive — Quick Play has no kick affordance, but the WS path
  // exists): toast + redirect, suppressing the cleanup leave.
  useEffect(() => {
    if (kickedFromRoomId !== null && id && kickedFromRoomId === Number(id)) {
      hasLeftRef.current = true;
      useRoomStore.getState().setKickedFromRoom(null);
      navigate("/lobby");
    }
  }, [kickedFromRoomId, id, navigate]);

  // Reset the store on unmount / :id change (mirrors RoomPage).
  useEffect(() => {
    return () => {
      useRoomStore.getState().reset();
    };
  }, [id]);

  // Leave the queue on unmount unless the player already left, was kicked, or
  // the game started — frees their seat when they navigate away (e.g. back).
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
      // The room flipped to "playing" between click and leave tx — stay put and
      // let the match_started flag drive navigation to the game.
      if (err instanceof FetchError && err.code === "MATCH_ALREADY_STARTED") {
        hasLeftRef.current = false;
        toast.error(t("room.errors.alreadyStarted"));
        return;
      }
    }
    navigate("/lobby");
  };

  // Loading skeleton — also shown until the fresh post-mount fetch settles, so
  // a stale cached snapshot (which may omit the just-joined viewer) never
  // renders or triggers the redirect guard above.
  if (roomQuery.isLoading || !freshFetchSettled || (!storeRoom && !roomQuery.data)) {
    return (
      <div
        className="mx-auto flex min-h-[60vh] max-w-330 items-center justify-center px-7 py-10"
        data-testid="matchmaking-loading"
      >
        <div className="bg-surface-sunken size-24 animate-pulse rounded-full" />
      </div>
    );
  }

  const room = storeRoom ?? roomQuery.data?.room;
  // Non-quick-play rooms and rooms the viewer isn't in are handled by the
  // redirect effect above; render nothing while it navigates.
  if (!room || !room.isQuickPlay) return null;

  const players = storePlayers.length > 0 ? storePlayers : (roomQuery.data?.players ?? []);
  const viewerPlayer = players.find((p) => p.userId === currentUser?.id);
  const viewerSeat = viewerPlayer?.seat ?? null;
  if (viewerSeat === null) return null;

  const currentUsername = viewerPlayer?.username ?? currentUser?.username ?? "";
  const found = players.filter((p) => p.seat !== null).length;

  return (
    <div
      className="mx-auto flex min-h-[calc(100vh-160px)] max-w-330 flex-col items-center justify-center px-7 py-10"
      data-testid="matchmaking-page"
    >
      <MatchmakingDiagram
        room={room}
        found={found}
        players={players}
        viewerSeat={viewerSeat}
        currentUsername={currentUsername}
        elapsed={formatElapsed(elapsedSec)}
        onCancel={handleCancel}
        cancelDisabled={leaveRoomMutation.isPending}
      />
    </div>
  );
}
