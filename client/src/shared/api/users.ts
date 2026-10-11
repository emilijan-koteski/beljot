import { axiosClient } from "@/shared/api/axiosClient";
import type { PlayerSearchResult } from "@/shared/types/apiTypes";

/** Searches players whose username contains `search`, case-insensitively, excluding the caller. */
export function searchUsers(search: string): Promise<PlayerSearchResult[]> {
  return axiosClient.get("/users", { params: { search } });
}
