import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router";
import { toast } from "sonner";

import { FriendsDialog } from "@/features/friends/FriendsDialog";
import { FriendsSidebar } from "@/features/friends/FriendsSidebar";
import { JoinByCodeDialog } from "@/features/lobby/components/JoinByCodeDialog";
import { LobbyActions } from "@/features/lobby/components/LobbyActions";
import { LobbyChatDock } from "@/features/lobby/components/LobbyChatDock";
import { LobbyFilters } from "@/features/lobby/components/LobbyFilters";
import { LobbyCounts, PresenceLine } from "@/features/lobby/components/LobbyPresence";
import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import { RoomEjectionModal } from "@/features/lobby/components/RoomEjectionModal";
import { RoomGrid } from "@/features/lobby/components/RoomGrid";
import {
  filterAndSort,
  filterCounts,
  type LobbyFilter,
  type LobbySort,
  type LobbyTab,
} from "@/features/lobby/lib/lobbyRooms";
import { CreateRoomModal } from "@/features/room/CreateRoomModal";
import { FetchError } from "@/shared/api/axiosClient";
import { PageShell } from "@/shared/components/layout/PageShell";
import { SupportNote } from "@/shared/components/support/SupportNote";
import { Button } from "@/shared/components/ui/button";
import { Segmented } from "@/shared/components/ui/segmented";
import {
  useJoinRoomMutation,
  useQuickJoinMutation,
  useQuickPlayMutation,
} from "@/shared/hooks/mutations/useRooms";
import { useLobbyStatsQuery } from "@/shared/hooks/queries/useLobbyStats";
import { useRoomsQuery } from "@/shared/hooks/queries/useRooms";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { useMarkLobbyRoot } from "@/shared/hooks/useLobbyReturn";
import { joinFailureMessage } from "@/shared/lib/joinFailure";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Room } from "@/shared/types/apiTypes";

const JOINING_TOAST_ID = "lobby-joining";

/**
 * The lobby for every width: action tiles, open rooms and live matches, with filters, search and sort.
 * Friends sit in a sidebar on a desktop and in a sheet elsewhere; a phone pins New room to the bottom.
 */
export function LobbyPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const band = useLayoutBand();
  useMarkLobbyRoot();

  const roomsQuery = useRoomsQuery("waiting", true);
  const liveQuery = useRoomsQuery("playing", true);
  const statsQuery = useLobbyStatsQuery();
  const quickPlayMutation = useQuickPlayMutation();
  const quickJoinMutation = useQuickJoinMutation();
  const joinRoomMutation = useJoinRoomMutation();

  const location = useLocation();
  const [tab, setTab] = useState<LobbyTab>("open");
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<LobbyFilter>(() =>
    (location.state as { lobbyFilter?: LobbyFilter } | null)?.lobbyFilter === "qualify"
      ? "qualify"
      : "all",
  );
  const [sort, setSort] = useState<LobbySort>("filling");
  const [showCreate, setShowCreate] = useState(false);
  const [showCode, setShowCode] = useState(false);
  const [showFriends, setShowFriends] = useState(false);
  const [pendingPrivateRoom, setPendingPrivateRoom] = useState<Room | null>(null);
  const [passwordErrorKey, setPasswordErrorKey] = useState<string | null>(null);

  const rooms = useMemo(() => roomsQuery.data ?? [], [roomsQuery.data]);
  const liveRooms = useMemo(() => liveQuery.data ?? [], [liveQuery.data]);
  const viewerHonorScore = useAuthStore((s) => s.user?.honorScore);
  const viewerIsNewPlayer = useAuthStore((s) => s.user?.isNewPlayer);
  const honorViewer = useMemo(
    () => ({ honorScore: viewerHonorScore, isNewPlayer: viewerIsNewPlayer }),
    [viewerHonorScore, viewerIsNewPlayer],
  );
  const counts = useMemo(() => filterCounts(rooms, honorViewer), [rooms, honorViewer]);
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
    if (password === undefined) {
      toast.loading(t("lobby.card.joining", { name: room.name }), { id: JOINING_TOAST_ID });
    }
    try {
      await joinRoomMutation.mutateAsync({ id: room.id, password });
      setPendingPrivateRoom(null);
      toast.dismiss(JOINING_TOAST_ID);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      toast.dismiss(JOINING_TOAST_ID);
      const code = err instanceof FetchError ? err.code : null;
      if (code === "WRONG_ROOM_PASSWORD") {
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      setPendingPrivateRoom(null);
      toast.error(joinFailureMessage(code, room));
    }
  }

  const desktop = band === "desktop";
  const newRoomBar =
    band === "phone" ? (
      <Button
        size="cta"
        className="w-full text-base"
        onClick={() => setShowCreate(true)}
        data-testid="create-room-card"
      >
        <Plus className="size-5" aria-hidden="true" />
        {t("lobby.actions.createRoom.title")}
      </Button>
    ) : undefined;

  return (
    <PageShell
      sidebar={desktop ? <FriendsSidebar /> : undefined}
      bottomBar={newRoomBar}
      testId="lobby-page"
    >
      <LobbyActions
        band={band}
        onQuickPlay={handleQuickPlay}
        quickPlayDisabled={quickPlayMutation.isPending}
        onOpenCode={() => setShowCode(true)}
        onNewRoom={() => setShowCreate(true)}
      />
      {!desktop && (
        <PresenceLine stats={statsQuery.data} onOpenFriends={() => setShowFriends(true)} />
      )}
      <div className="flex items-center justify-between gap-4">
        <Segmented<LobbyTab>
          value={tab}
          onValueChange={setTab}
          ariaLabel={t("lobby.tabs.label")}
          testId="lobby-tabs"
          className={desktop ? "w-90" : "flex-1"}
          options={[
            { value: "open", label: t("lobby.tabs.open", { count: rooms.length }) },
            { value: "playing", label: t("lobby.tabs.playing", { count: liveRooms.length }) },
          ]}
        />
        {desktop && <LobbyCounts stats={statsQuery.data} />}
      </div>
      {tab === "open" ? (
        <>
          <LobbyFilters
            search={search}
            setSearch={setSearch}
            filter={filter}
            setFilter={setFilter}
            sort={sort}
            setSort={setSort}
            counts={counts}
            collapsed={!desktop}
          />
          <RoomGrid
            rooms={filtered}
            onJoin={handleJoinRoom}
            hasSearch={search.trim().length > 0}
            onClearSearch={() => setSearch("")}
          />
          <p className="text-ink-mute m-0 mt-2 text-center text-xs">
            {t("lobby.footnote", { shown: filtered.length, total: rooms.length })}
          </p>
        </>
      ) : (
        <RoomGrid rooms={liveRooms} live />
      )}
      <SupportNote variant="lobby" className="mt-0" />

      <CreateRoomModal open={showCreate} onOpenChange={setShowCreate} />
      <JoinByCodeDialog open={showCode} onClose={() => setShowCode(false)} />
      {!desktop && <FriendsDialog open={showFriends} onClose={() => setShowFriends(false)} />}
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
        }}
      />
      <RoomEjectionModal />
      <LobbyChatDock aboveBottomBar={band === "phone"} />
    </PageShell>
  );
}
