import { axiosClient } from "@/shared/api/axiosClient";
import type { PlayerSearchResult } from "@/shared/types/apiTypes";

/** Searches players whose username contains `search`, case-insensitively, excluding the caller. */
export function searchUsers(search: string): Promise<PlayerSearchResult[]> {
  return axiosClient.get("/users", { params: { search } });
}

/** Confirms an account deletion: the password when the account has one, otherwise the current username. */
export interface DeleteAccountRequest {
  password?: string;
  username?: string;
}

/** Permanently erases the caller's account; the server also clears the refresh cookie and closes the socket. */
export function deleteAccount(userId: number, data: DeleteAccountRequest): Promise<void> {
  return axiosClient.delete(`/users/${userId}`, { data });
}
