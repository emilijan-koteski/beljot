import { type HonorGateViewer, honorQualifies } from "@/shared/lib/honor";
import { isRoomFull, roomOccupancy } from "@/shared/lib/roomOccupancy";
import type { Room } from "@/shared/types/apiTypes";

export type LobbyTab = "open" | "playing";
export type LobbyFilter = "all" | "qualify" | "open" | "relaxed" | "timed";
export type LobbySort = "newest" | "filling";
export type FilterCounts = Record<LobbyFilter, number>;

export const LOBBY_FILTERS: LobbyFilter[] = ["all", "qualify", "open", "relaxed", "timed"];

function matchesFilter(room: Room, filter: LobbyFilter, viewer: HonorGateViewer): boolean {
  if (filter === "open") return !isRoomFull(room);
  if (filter === "relaxed") return room.timerStyle === "relaxed";
  if (filter === "timed") return room.timerStyle !== "relaxed";
  if (filter === "qualify") return honorQualifies(room, viewer);
  return true;
}

function newestFirst(a: Room, b: Room): number {
  return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
}

/**
 * The open rooms a search, filter and sort leave. "Filling up first" puts the rooms nearest to starting on top
 * and full rooms last, since nobody can join them; bots count as seated either way.
 */
export function filterAndSort(
  rooms: Room[],
  search: string,
  filter: LobbyFilter,
  sort: LobbySort,
  viewer: HonorGateViewer,
): Room[] {
  const q = search.trim().toLowerCase();
  const shown = rooms.filter((r) => {
    if (q && !r.name.toLowerCase().includes(q) && !r.code.toLowerCase().includes(q)) return false;
    return matchesFilter(r, filter, viewer);
  });
  if (sort === "newest") return shown.sort(newestFirst);
  return shown.sort(
    (a, b) =>
      Number(isRoomFull(a)) - Number(isRoomFull(b)) ||
      roomOccupancy(b) - roomOccupancy(a) ||
      newestFirst(a, b),
  );
}

/** How many open rooms each filter chip would show. */
export function filterCounts(rooms: Room[], viewer: HonorGateViewer): FilterCounts {
  const counts = {} as FilterCounts;
  for (const filter of LOBBY_FILTERS) {
    counts[filter] = rooms.filter((r) => matchesFilter(r, filter, viewer)).length;
  }
  return counts;
}
