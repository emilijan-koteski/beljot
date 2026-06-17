import { useMemo } from "react";

import { useMatchStartRedirect } from "@/shared/hooks/useMatchStartRedirect";
import { useWebSocket } from "@/shared/hooks/useWebSocket";
import { useWsDispatch } from "@/shared/hooks/useWsDispatch";

import { WebSocketContext } from "./WebSocketContext";

export function WebSocketProvider({ children }: { children: React.ReactNode }) {
  const dispatch = useWsDispatch();
  const { sendMessage, state } = useWebSocket({ onMessage: dispatch });
  // D145b: always-mounted navigator that routes a seated player into a freshly
  // started match even when they are not on RoomPage.
  useMatchStartRedirect();

  const value = useMemo(() => ({ sendMessage, connectionState: state }), [sendMessage, state]);

  return <WebSocketContext.Provider value={value}>{children}</WebSocketContext.Provider>;
}
