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
    // Purely client-side: the room's gate and the viewer's score are both already
    // on hand, so this needs no request. Cosmetic like every other honour mirror —
    // the server re-validates the actual join.
    if (filter === "qualify" && !honorQualifies(r, viewer)) return false;
    return true;
  });
  const sorted = [...filtered];
  if (sort === "newest") {
    sorted.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
  } else {
    // "filling" — most-occupied first, breaking ties by newer-first
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
  // Record this history entry as the app root so every "return to lobby"
  // (match end, room leave, TopBar "Play") can pop back to it instead of
  // stacking a fresh entry.
  useMarkLobbyRoot();

  // Lobby grid is always-on now — no separate "options" vs "browse" view.
  const roomsQuery = useRoomsQuery("waiting", true);
  const statsQuery = useLobbyStatsQuery();
  const quickPlayMutation = useQuickPlayMutation();
  const quickJoinMutation = useQuickJoinMutation();
  const joinRoomMutation = useJoinRoomMutation();

  const [search, setSearch] = useState("");
  // The ejection modal's "Rooms I qualify for" action routes here with this state,
  // so an ejected player lands on a shorter list instead of the wall they just hit.
  // Read once as the initial value: the filter is the user's afterwards, and
  // re-applying it on every render would fight them.
  const location = useLocation();
  const [filter, setFilter] = useState<LobbyFilter>(() =>
    (location.state as { lobbyFilter?: LobbyFilter } | null)?.lobbyFilter === "qualify"
      ? "qualify"
      : "all",
  );
  const [sort, setSort] = useState<LobbySort>("filling");
  const [showCreate, setShowCreate] = useState(false);
  const [toastMsg, setToastMsg] = useState<string | null>(null);
  // Private-room join (Story 9.6): the room awaiting a password prompt + the
  // current prompt error key (set on WRONG_ROOM_PASSWORD so the dialog stays open).
  const [pendingPrivateRoom, setPendingPrivateRoom] = useState<Room | null>(null);
  const [passwordErrorKey, setPasswordErrorKey] = useState<string | null>(null);

  // Stabilise the array reference: `roomsQuery.data ?? []` would mint a fresh
  // `[]` every render while data is undefined, busting the useMemos below.
  const rooms = useMemo(() => roomsQuery.data ?? [], [roomsQuery.data]);
  // Narrowed to just the two fields the gate reads, so the memos below re-run when
  // honour changes but not on every unrelated auth-store write (wallet, XP…).
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

  // Routes a quick-play response (from either Quick Play or a quick-join) to the
  // matchmaking screen, or straight to the game if this entry filled the table.
  function goToMatchmaking(result: { room: Room; matchStarted: boolean }) {
    if (result.matchStarted) {
      // `fromRoom: true` triggers MatchPage's "Game is starting…" splash so the
      // auto-start has the same deliberate beat as a normal lobby.
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
    // Quick-play rooms get the matchmaking queue, not the in-room seat grid:
    // quick-join auto-seats the player so the auto-start check can fire.
    if (room.isQuickPlay) {
      if (quickJoinMutation.isPending) return;
      try {
        goToMatchmaking(await quickJoinMutation.mutateAsync(room.id));
      } catch (err) {
        const code = err instanceof FetchError ? err.code : null;
        // Story 9.4: tapping a quick-play room in the wrong coin bracket is
        // rejected with QUICK_PLAY_BRACKET_MISMATCH. The stake is charged at
        // auto-start (not at join), so INSUFFICIENT_COINS isn't returned here —
        // keep its handler as a forward-compat safety net.
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

    // Private rooms (Story 9.6): prompt for the password before joining. The
    // client knows the room is private from room.isPrivate; the server verifies.
    if (room.isPrivate) {
      setPasswordErrorKey(null);
      setPendingPrivateRoom(room);
      return;
    }

    await joinRoomFlow(room);
  }

  // Shared join → navigate path for a non-quick-play room. `password` is sent
  // only for private rooms (via the prompt). Errors are surfaced as toasts,
  // except WRONG_ROOM_PASSWORD which the private-room prompt handles inline.
  async function joinRoomFlow(room: Room, password?: string) {
    // The "Joining…" toast is only for the public path. For a private room the
    // password prompt's own pending state covers the in-flight feedback and a
    // wrong password is shown inline in the dialog — showing the toast first
    // would flash "Joining…" and then a wrong-password error (confusing).
    if (password === undefined) setToastMsg(t("lobby.card.joining", { name: room.name }));
    try {
      await joinRoomMutation.mutateAsync({ id: room.id, password });
      setPendingPrivateRoom(null);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      setToastMsg(null);
      const code = err instanceof FetchError ? err.code : null;
      if (code === "WRONG_ROOM_PASSWORD") {
        // Keep the prompt open and show the inline error so the player can retry.
        setPasswordErrorKey("room.errors.wrongPassword");
        return;
      }
      setPendingPrivateRoom(null);
      // One shared mapping for every join entry point (Story 11.5 D4) — the rich
      // {{buyIn}}/{{balance}} and {{minHonor}}/{{honor}} messages are composed
      // locally inside it from this room plus the viewer's own auth envelope.
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

      {/* Story 11.2: the friends card, full width — one card for the whole
          relationship. It carries the pending-requests section, the roster with
          online status and whisper, and player search behind its header icon, so
          the lobby spends one heading and one border on all three instead of
          three of each. */}
      <FriendList />

      {/* NO MAIN/ASIDE SPLIT. Story 13.2 put a 320px seasonal-leaderboard
          column here; it was removed once the dedicated /leaderboard page (one
          click away in the top nav) made a lobby preview of the same ten rows
          redundant. Nothing else wanted the aside, so the room browser takes
          the full width back rather than leaving a column empty beside it.

          The room filters belong to the room grid, so they sit directly above
          it — with the friends row above, the search field is no longer
          separated from the list it filters by two unrelated cards. */}
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
      {/* Story 11.5's RoomInviteModal is NOT mounted here — it lives in AppLayout
          so an invite renders on every authed route, not just the lobby. */}
      <LobbyChatDock />
      <Toast message={toastMsg} onClear={() => setToastMsg(null)} />
    </div>
  );
}
