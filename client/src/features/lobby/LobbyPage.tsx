import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import type { FilterCounts, LobbyFilter, LobbySort } from "@/features/lobby/components/FilterRail";
import { FilterRail } from "@/features/lobby/components/FilterRail";
import { HeroBlock } from "@/features/lobby/components/HeroBlock";
import { InsolventEjectionModal } from "@/features/lobby/components/InsolventEjectionModal";
import { LobbyChatDock } from "@/features/lobby/components/LobbyChatDock";
import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
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
import { formatCoins } from "@/shared/lib/formatCoins";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Room } from "@/shared/types/apiTypes";

function filterAndSort(
  rooms: Room[],
  search: string,
  filter: LobbyFilter,
  sort: LobbySort,
): Room[] {
  const q = search.trim().toLowerCase();
  const filtered = rooms.filter((r) => {
    if (q && !r.name.toLowerCase().includes(q) && !r.code.toLowerCase().includes(q)) return false;
    if (filter === "open" && r.playerCount >= 4) return false;
    if (filter === "relaxed" && r.timerStyle !== "relaxed") return false;
    if (filter === "timed" && r.timerStyle === "relaxed") return false;
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

function deriveCounts(rooms: Room[]): FilterCounts {
  return {
    all: rooms.length,
    open: rooms.filter((r) => r.playerCount < 4).length,
    relaxed: rooms.filter((r) => r.timerStyle === "relaxed").length,
    timed: rooms.filter((r) => r.timerStyle !== "relaxed").length,
  };
}

export function LobbyPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  // Lobby grid is always-on now — no separate "options" vs "browse" view.
  const roomsQuery = useRoomsQuery("waiting", true);
  const statsQuery = useLobbyStatsQuery();
  const quickPlayMutation = useQuickPlayMutation();
  const quickJoinMutation = useQuickJoinMutation();
  const joinRoomMutation = useJoinRoomMutation();

  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<LobbyFilter>("all");
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
  const counts = useMemo(() => deriveCounts(rooms), [rooms]);
  const filtered = useMemo(
    () => filterAndSort(rooms, search, filter, sort),
    [rooms, search, filter, sort],
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
      if (code === "ROOM_FULL") toast.error(t("lobby.errors.roomFull"));
      else if (code === "INSUFFICIENT_COINS")
        // Compose the rich message locally — we know the room's buy-in and our
        // own balance (Design Decision B: the error payload carries only a code).
        toast.error(
          t("room.errors.insufficientCoins", {
            buyIn: formatCoins(room.coinBuyIn),
            balance: formatCoins(useAuthStore.getState().user?.walletBalance ?? 0),
          }),
        );
      else if (code === "ALREADY_IN_ROOM") toast.error(t("lobby.errors.alreadyInRoom"));
      else toast.error(t("lobby.errors.joinFailed"));
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
      <InsolventEjectionModal />
      <LobbyChatDock />
      <Toast message={toastMsg} onClear={() => setToastMsg(null)} />
    </div>
  );
}
