import { createContext, useContext } from "react";

import type { WsConnectionState } from "@/shared/hooks/useWebSocket";

interface WebSocketContextValue {
  sendMessage: (type: string, payload: unknown) => void;
  connectionState: WsConnectionState;
}

export const WebSocketContext = createContext<WebSocketContextValue | null>(null);

export function useWsSendMessage(): (type: string, payload: unknown) => void {
  const ctx = useContext(WebSocketContext);
  if (!ctx) {
    throw new Error("useWsSendMessage must be used within WebSocketProvider");
  }
  return ctx.sendMessage;
}

export function useWsConnectionState(): WsConnectionState {
  const ctx = useContext(WebSocketContext);
  if (!ctx) {
    throw new Error("useWsConnectionState must be used within WebSocketProvider");
  }
  return ctx.connectionState;
}
