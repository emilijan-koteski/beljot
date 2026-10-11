import { useMemo } from "react";

import { useMatchStartRedirect } from "@/shared/hooks/useMatchStartRedirect";
import { useRoomEjectRedirect } from "@/shared/hooks/useRoomEjectRedirect";
import { useWebSocket } from "@/shared/hooks/useWebSocket";
import { useWsDispatch } from "@/shared/hooks/useWsDispatch";

import { WebSocketContext } from "./WebSocketContext";

export function WebSocketProvider({ children }: { children: React.ReactNode }) {
  const dispatch = useWsDispatch();
  const { sendMessage, state } = useWebSocket({ onMessage: dispatch });
  useMatchStartRedirect();
  useRoomEjectRedirect();

  const value = useMemo(() => ({ sendMessage, connectionState: state }), [sendMessage, state]);

  return <WebSocketContext.Provider value={value}>{children}</WebSocketContext.Provider>;
}
