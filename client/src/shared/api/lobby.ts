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
