import { useCallback, useEffect, useRef, useState } from "react";

import { refresh } from "@/shared/api/auth";
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

/**
 * App-specific WS close code matching the server's `CloseCodeReplaced`
 * (`server/internal/ws/hub.go`). Sent when a newer connection for the same
 * user joins and kicks this one. The client must NOT auto-reconnect on this
 * code — otherwise two tabs as the same user enter a tight kick/reconnect
 * loop that floods the server with auth churn.
 */
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

    // Guard against being called while already connecting
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

      // Clock-offset sample: match messages stamp the server's wall clock on
      // the envelope so countdowns can render against corrected time.
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

      // Forward all other messages to dispatch
      onMessageRef.current(message);
    };

    ws.onclose = (event: CloseEvent) => {
      if (!mountedRef.current || wsRef.current !== ws) return;
      wsRef.current = null;
      // The server sends CLOSE_CODE_REPLACED when another tab/device for the
      // same user took over the slot. Auto-reconnecting here would just kick
      // the newer connection back, so we park in a terminal "replaced" state
      // until the user explicitly takes action (refresh, re-login, logout).
      if (event.code === CLOSE_CODE_REPLACED) {
        updateState("replaced");
        return;
      }
      updateState("disconnected");
      scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose will fire after onerror — reconnection handled there
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleAuthFailure = useCallback(async () => {
    try {
      const res = await refresh();
      useAuthStore.getState().setToken(res.token);
      // Retry connection with fresh token
      reconnectAttemptRef.current = 0;
      scheduleReconnect();
    } catch {
      // Refresh failed — user must re-login
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
    // Defer connect so React.StrictMode's synchronous unmount cancels
    // the timer before any WebSocket is created, avoiding the
    // "WebSocket is closed before the connection is established" warning.
    const connectTimer = setTimeout(connect, 0);

    return () => {
      mountedRef.current = false;
      clearTimeout(connectTimer);
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

  // Reconnect when token changes (e.g., after refresh)
  useEffect(() => {
    const unsub = useAuthStore.subscribe((state, prev) => {
      if (state.token && !prev.token && mountedRef.current) {
        // Token appeared (login or refresh) — connect if disconnected
        if (!wsRef.current) {
          reconnectAttemptRef.current = 0;
          connect();
        }
      }
      if (!state.token && prev.token) {
        // Token cleared (logout) — disconnect
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
