import { axiosClient } from "@/shared/api/axiosClient";

export interface LobbyStats {
  inLobby: number;
  inRoom: number;
  inMatch: number;
  online: number;
  registered: number;
}

export function getLobbyStats(): Promise<LobbyStats> {
  return axiosClient.get("/lobby/stats");
}

// Public, unauthenticated landing-page counts. Served by GET /api/v1/stats
// (no auth middleware), so this resolves for logged-out visitors.
interface PublicStats {
  online: number;
  openRooms: number;
}

export function getPublicStats(): Promise<PublicStats> {
  return axiosClient.get("/stats");
}
