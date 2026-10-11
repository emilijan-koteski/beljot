import { useCallback, useEffect, useRef, useState } from "react";

import { refreshAccessToken } from "@/shared/api/axiosClient";
import { recordServerNow } from "@/shared/lib/clockSync";
import { useAuthStore } from "@/shared/stores/authStore";
import type { AuthenticatedPayload, AuthFailedPayload, WsMessage } from "@/shared/types/wsEvents";
import {
  ACTION_AUTHENTICATE,
  ERROR_AUTH_FAILED,
  SYSTEM_AUTHENTICATED,
} from "@/shared/types/wsEvents";

export type WsConnectionState =
  | "disconnected"
  | "connecting"
  | "authenticating"
  | "connected"
  | "replaced";

const CLOSE_CODE_REPLACED = 4001;

interface UseWebSocketOptions {
  onMessage: (message: WsMessage) => void;
}

interface UseWebSocketReturn {
  state: WsConnectionState;
  sendMessage: (type: string, payload: unknown) => void;
}

const MAX_RECONNECT_DELAY = 30000;
const BASE_RECONNECT_DELAY = 1000;

function getWsUrl(): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/ws`;
}

/**
 * Keep an authenticated WebSocket open with exponential-backoff reconnects.
 * A CLOSE_CODE_REPLACED close parks in "replaced" instead, so two tabs never kick each other in a loop.
 */
export function useWebSocket({ onMessage }: UseWebSocketOptions): UseWebSocketReturn {
  const [state, setState] = useState<WsConnectionState>("disconnected");
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  const stateRef = useRef<WsConnectionState>("disconnected");
  const updateState = useCallback((s: WsConnectionState) => {
    stateRef.current = s;
    setState(s);
  }, []);

  const sendMessage = useCallback((type: string, payload: unknown) => {
    if (stateRef.current === "connected" && wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, payload }));
    }
  }, []);

  const connect = useCallback(() => {
    if (!mountedRef.current) return;

    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) return;

    const token = useAuthStore.getState().token;
    if (!token) {
      updateState("disconnected");
      return;
    }

    updateState("connecting");
    const ws = new WebSocket(getWsUrl());
    wsRef.current = ws;

    ws.onopen = () => {
      if (!mountedRef.current || wsRef.current !== ws) return;
      updateState("authenticating");
      ws.send(
        JSON.stringify({
          type: ACTION_AUTHENTICATE,
          payload: { token: useAuthStore.getState().token },
        }),
      );
    };

    ws.onmessage = (event: MessageEvent) => {
      if (!mountedRef.current || wsRef.current !== ws) return;

      let message: WsMessage;
      try {
        message = JSON.parse(String(event.data)) as WsMessage;
      } catch {
        return;
      }

      if (typeof message.serverNow === "string") {
        recordServerNow(message.serverNow);
      }

      if (message.type === SYSTEM_AUTHENTICATED) {
        updateState("connected");
        reconnectAttemptRef.current = 0;
        const payload = message.payload as AuthenticatedPayload;
        if (payload.userId !== undefined) {
          onMessageRef.current(message);
        }
        return;
      }

      if (message.type === ERROR_AUTH_FAILED) {
        const payload = message.payload as AuthFailedPayload;
        console.warn("WS auth failed:", payload.message);
        handleAuthFailure();
        return;
      }

      onMessageRef.current(message);
    };

    ws.onclose = (event: CloseEvent) => {
      if (!mountedRef.current || wsRef.current !== ws) return;
      wsRef.current = null;
      if (event.code === CLOSE_CODE_REPLACED) {
        updateState("replaced");
        return;
      }
      updateState("disconnected");
      scheduleReconnect();
    };

    ws.onerror = () => {};
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleAuthFailure = useCallback(async () => {
    try {
      await refreshAccessToken();
      reconnectAttemptRef.current = 0;
      scheduleReconnect();
    } catch {
      useAuthStore.getState().logout();
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const scheduleReconnect = useCallback(() => {
    if (!mountedRef.current) return;
    if (reconnectTimerRef.current) return;

    const delay = Math.min(
      BASE_RECONNECT_DELAY * Math.pow(2, reconnectAttemptRef.current),
      MAX_RECONNECT_DELAY,
    );
    reconnectAttemptRef.current++;

    reconnectTimerRef.current = setTimeout(() => {
      reconnectTimerRef.current = null;
      connect();
    }, delay);
  }, [connect]);

  useEffect(() => {
    mountedRef.current = true;
    const deferredConnectSurvivingStrictMode = setTimeout(connect, 0);

    return () => {
      mountedRef.current = false;
      clearTimeout(deferredConnectSurvivingStrictMode);
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.close(1000, "component unmount");
        wsRef.current = null;
      }
    };
  }, [connect]);

  useEffect(() => {
    const unsub = useAuthStore.subscribe((state, prev) => {
      if (state.token && !prev.token && mountedRef.current) {
        if (!wsRef.current) {
          reconnectAttemptRef.current = 0;
          connect();
        }
      }
      if (!state.token && prev.token) {
        if (wsRef.current) {
          wsRef.current.close(1000, "logout");
          wsRef.current = null;
        }
        if (reconnectTimerRef.current) {
          clearTimeout(reconnectTimerRef.current);
          reconnectTimerRef.current = null;
        }
        updateState("disconnected");
      }
    });
    return unsub;
  }, [connect, updateState]);

  return { state, sendMessage };
}
