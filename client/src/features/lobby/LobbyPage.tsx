import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";
import { toast } from "sonner";

import { FriendList } from "@/features/friends/FriendList";
import type { FilterCounts, LobbyFilter, LobbySort } from "@/features/lobby/components/FilterRail";
import { FilterRail } from "@/features/lobby/components/FilterRail";
import { HeroBlock } from "@/features/lobby/components/HeroBlock";
import { LobbyChatDock } from "@/features/lobby/components/LobbyChatDock";
import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { RoomEjectionModal } from "@/features/lobby/components/RoomEjectionModal";
import { RoomGrid } from "@/features/lobby/components/RoomGrid";
import { Toast } from "@/features/lobby/components/Toast";
import { CreateRoomModal } from "@/features/room/CreateRoomModal";
import { FetchError } from "@/shared/api/axiosClient";
import { SupportNote } from "@/shared/components/support/SupportNote";
import {
  useJoinRoomMutation,
  useQuickJoinMutation,
  useQuickPlayMutation,
} from "@/shared/hooks/mutations/useRooms";
import { useLobbyStatsQuery } from "@/shared/hooks/queries/useLobbyStats";
import { useRoomsQuery } from "@/shared/hooks/queries/useRooms";
import { useMarkLobbyRoot } from "@/shared/hooks/useLobbyReturn";
import { type HonorGateViewer, honorQualifies } from "@/shared/lib/honor";
import { joinFailureMessage } from "@/shared/lib/joinFailure";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Room } from "@/shared/types/apiTypes";

function filterAndSort(
  rooms: Room[],
  search: string,
  filter: LobbyFilter,
  sort: LobbySort,
  viewer: HonorGateViewer,
): Room[] {
  const q = search.trim().toLowerCase();
  const filtered = rooms.filter((r) => {
    if (q && !r.name.toLowerCase().includes(q) && !r.code.toLowerCase().includes(q)) return false;
    if (filter === "open" && r.playerCount >= 4) return false;
    if (filter === "relaxed" && r.timerStyle !== "relaxed") return false;
    if (filter === "timed" && r.timerStyle === "relaxed") return false;
    if (filter === "qualify" && !honorQualifies(r, viewer)) return false;
    return true;
  });
  const sorted = [...filtered];
  if (sort === "newest") {
    sorted.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
  } else {
    sorted.sort(
      (a, b) =>
        b.playerCount - a.playerCount ||
        new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
    );
  }
  return sorted;
}

function deriveCounts(rooms: Room[], viewer: HonorGateViewer): FilterCounts {
  return {
    all: rooms.length,
    qualify: rooms.filter((r) => honorQualifies(r, viewer)).length,
    open: rooms.filter((r) => r.playerCount < 4).length,
    relaxed: rooms.filter((r) => r.timerStyle === "relaxed").length,
    timed: rooms.filter((r) => r.timerStyle !== "relaxed").length,
  };
}

export function LobbyPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  useMarkLobbyRoot();

  const roomsQuery = useRoomsQuery("waiting", true);
  const statsQuery = useLobbyStatsQuery();
  const quickPlayMutation = useQuickPlayMutation();
  const quickJoinMutation = useQuickJoinMutation();
  const joinRoomMutation = useJoinRoomMutation();

  const [search, setSearch] = useState("");
  const location = useLocation();
  const [filter, setFilter] = useState<LobbyFilter>(() =>
    (location.state as { lobbyFilter?: LobbyFilter } | null)?.lobbyFilter === "qualify"
      ? "qualify"
      : "all",
  );
  const [sort, setSort] = useState<LobbySort>("filling");
  const [showCreate, setShowCreate] = useState(false);
  const [toastMsg, setToastMsg] = useState<string | null>(null);
  const [pendingPrivateRoom, setPendingPrivateRoom] = useState<Room | null>(null);
  const [passwordErrorKey, setPasswordErrorKey] = useState<string | null>(null);

  const rooms = useMemo(() => roomsQuery.data ?? [], [roomsQuery.data]);
  const viewerHonorScore = useAuthStore((s) => s.user?.honorScore);
  const viewerIsNewPlayer = useAuthStore((s) => s.user?.isNewPlayer);
  const honorViewer = useMemo(
    () => ({ honorScore: viewerHonorScore, isNewPlayer: viewerIsNewPlayer }),
    [viewerHonorScore, viewerIsNewPlayer],
  );
  const counts = useMemo(() => deriveCounts(rooms, honorViewer), [rooms, honorViewer]);
  const filtered = useMemo(
    () => filterAndSort(rooms, search, filter, sort, honorViewer),
    [rooms, search, filter, sort, honorViewer],
  );

  function goToMatchmaking(result: { room: Room; matchStarted: boolean }) {
    if (result.matchStarted) {
      navigate(`/match/${result.room.id}`, { state: { fromRoom: true } });
    } else {
      navigate(`/matchmaking/${result.room.id}`);
    }
  }

  async function handleQuickPlay() {
    if (quickPlayMutation.isPending) return;
    try {
      goToMatchmaking(await quickPlayMutation.mutateAsync(undefined));
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      toast.error(
        code === "ALREADY_IN_ROOM"
          ? t("lobby.matchmaking.errors.alreadyInRoom")
          : t("lobby.errors.matchmakingFailed"),
      );
    }
  }

  async function handleJoinRoom(room: Room) {
    if (room.isQuickPlay) {
      if (quickJoinMutation.isPending) return;
      try {
        goToMatchmaking(await quickJoinMutation.mutateAsync(room.id));
      } catch (err) {
        const code = err instanceof FetchError ? err.code : null;
        if (code === "ROOM_FULL") toast.error(t("lobby.errors.roomFull"));
        else if (code === "QUICK_PLAY_BRACKET_MISMATCH")
          toast.error(t("lobby.errors.quickPlayBracketMismatch"));
        else if (code === "INSUFFICIENT_COINS")
          toast.error(t("room.errors.insufficientCoinsGeneric"));
        else if (code === "ALREADY_IN_ROOM") toast.error(t("lobby.errors.alreadyInRoom"));
        else toast.error(t("lobby.errors.joinFailed"));
      }
      return;
    }

    if (room.isPrivate) {
      setPasswordErrorKey(null);
      setPendingPrivateRoom(room);
      return;
    }

    await joinRoomFlow(room);
  }

  async function joinRoomFlow(room: Room, password?: string) {
    if (password === undefined) setToastMsg(t("lobby.card.joining", { name: room.name }));
    try {
      await joinRoomMutation.mutateAsync({ id: room.id, password });
      setPendingPrivateRoom(null);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      setToastMsg(null);
      const code = err instanceof FetchError ? err.code : null;
      if (code === "WRONG_ROOM_PASSWORD") {
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      setPendingPrivateRoom(null);
      toast.error(joinFailureMessage(code, room));
    }
  }

  return (
    <div className="mx-auto max-w-330 px-7 py-8 pb-32">
      <HeroBlock
        openCount={rooms.length}
        stats={statsQuery.data}
        onQuickPlay={handleQuickPlay}
        onCreateRoom={() => setShowCreate(true)}
        quickPlayDisabled={quickPlayMutation.isPending}
      />

      <FriendList />

      <FilterRail
        search={search}
        setSearch={setSearch}
        filter={filter}
        setFilter={setFilter}
        sort={sort}
        setSort={setSort}
        counts={counts}
      />

      <RoomGrid
        rooms={filtered}
        onJoin={handleJoinRoom}
        hasSearch={search.trim().length > 0}
        onClearSearch={() => setSearch("")}
      />

      <p className="text-ink-mute mt-8 text-center text-xs">
        {t("lobby.footnote", { shown: filtered.length, total: rooms.length })}
      </p>

      <SupportNote variant="lobby" />

      <CreateRoomModal open={showCreate} onOpenChange={setShowCreate} />
      <PasswordPromptDialog
        open={pendingPrivateRoom !== null}
        roomName={pendingPrivateRoom?.name ?? ""}
        pending={joinRoomMutation.isPending}
        errorKey={passwordErrorKey}
        onSubmit={(password) => {
          if (pendingPrivateRoom) {
            setPasswordErrorKey(null);
            void joinRoomFlow(pendingPrivateRoom, password);
          }
        }}
        onClose={() => {
          setPendingPrivateRoom(null);
          setPasswordErrorKey(null);
          setToastMsg(null);
        }}
      />
      <RoomEjectionModal />
      <LobbyChatDock />
      <Toast message={toastMsg} onClear={() => setToastMsg(null)} />
    </div>
  );
}
