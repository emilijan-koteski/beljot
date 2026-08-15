import { axiosClient } from "@/shared/api/axiosClient";
import type { Friend, FriendRequest, FriendshipStatus } from "@/shared/types/apiTypes";

/**
 * Friend API (Story 11.2, FR6). Maps 1:1 to the `friend` backend domain. Every
 * function returns the UNWRAPPED payload — the axios response interceptor
 * already strips the `{ data }` envelope, so callers never touch `.data.data`.
 */

/** Send a friend request to `userId`. The requester is the authenticated caller. */
export function sendFriendRequest(userId: number): Promise<void> {
  return axiosClient.post("/friends/request", { userId });
}

/** Friendship state between the viewer and subject `id` (drives the profile button). */
export function getFriendshipStatus(id: number): Promise<FriendshipStatus> {
  return axiosClient.get(`/friends/status/${id}`);
}

/** The viewer's incoming pending friend requests. */
export function listFriendRequests(): Promise<FriendRequest[]> {
  return axiosClient.get("/friends/requests");
}

/** Accept an incoming request by its row id (recipient-only, server-enforced). */
export function acceptFriendRequest(id: number): Promise<void> {
  return axiosClient.post(`/friends/${id}/accept`);
}

/** Decline an incoming request by its row id (recipient-only, server-enforced). */
export function declineFriendRequest(id: number): Promise<void> {
  return axiosClient.post(`/friends/${id}/decline`);
}

/** The viewer's accepted friends, each with a live online flag. */
export function listFriends(): Promise<Friend[]> {
  return axiosClient.get("/friends");
}
