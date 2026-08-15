import { axiosClient } from "@/shared/api/axiosClient";
import type { PlayerSearchResult } from "@/shared/types/apiTypes";

/**
 * Search players by username (Story 11.1, FR5). Hits the authenticated
 * GET /api/v1/users?search=<query> endpoint, which matches usernames
 * case-insensitively as a substring, excludes the caller + soft-deleted users,
 * and caps the result server-side.
 *
 * Returns the unwrapped payload directly — the axios response interceptor
 * already strips the `{ data }` envelope, so callers never touch `.data.data`.
 */
export function searchUsers(search: string): Promise<PlayerSearchResult[]> {
  return axiosClient.get("/users", { params: { search } });
}
