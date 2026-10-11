import { useEffect, useRef } from "react";

import { refreshAccessToken, subscribeToTokenBroadcast } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";

const REFRESH_AT = 0.75;
const DEFAULT_LIFETIME_SEC = 15 * 60;
const MIN_DELAY_MS = 5_000;
const RETRY_DELAY_MS = 30_000;

interface JwtTiming {
  exp: number;
  iat: number;
}

/** Extracts exp/iat (epoch seconds) from a JWT, or null if it is not a decodable JWT with a numeric exp. */
export function decodeJwtTiming(token: string): JwtTiming | null {
  const parts = token.split(".");
  const payloadSegment = parts[1];
  if (parts.length !== 3 || !payloadSegment) return null;
  try {
    const base64 = payloadSegment.replace(/-/g, "+").replace(/_/g, "/");
    const payload = JSON.parse(atob(base64)) as { exp?: unknown; iat?: unknown };
    if (typeof payload.exp !== "number") return null;
    const iat = typeof payload.iat === "number" ? payload.iat : payload.exp - DEFAULT_LIFETIME_SEC;
    return { exp: payload.exp, iat };
  } catch {
    return null;
  }
}

/**
 * Returns how long from nowMs to wait before refreshing: REFRESH_AT of the token's lifetime,
 * floored at MIN_DELAY_MS. Returns null when the token cannot be decoded.
 */
export function computeRefreshDelayMs(token: string, nowMs: number): number | null {
  const timing = decodeJwtTiming(token);
  if (!timing) return null;
  const lifetimeMs = Math.max(0, (timing.exp - timing.iat) * 1000);
  const refreshAtMs = timing.iat * 1000 + lifetimeMs * REFRESH_AT;
  return Math.max(MIN_DELAY_MS, refreshAtMs - nowMs);
}

/**
 * Keeps the access token fresh by refreshing it shortly before expiry while the tab is visible.
 * Sibling tabs adopt the broadcast token instead of each refreshing; mount once near the app root.
 */
export function useTokenRefresh(): void {
  const token = useAuthStore((s) => s.token);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const dueWhileHiddenRef = useRef(false);

  useEffect(() => {
    return subscribeToTokenBroadcast((broadcast) => {
      if (broadcast !== useAuthStore.getState().token) {
        useAuthStore.getState().setToken(broadcast);
      }
    });
  }, []);

  useEffect(() => {
    dueWhileHiddenRef.current = false;
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (!token) return;

    const delay = computeRefreshDelayMs(token, Date.now());
    if (delay === null) return;

    const fire = () => {
      if (typeof document !== "undefined" && document.hidden) {
        dueWhileHiddenRef.current = true;
        return;
      }
      void refreshAccessToken().catch(() => {
        if (timerRef.current) clearTimeout(timerRef.current);
        timerRef.current = setTimeout(fire, RETRY_DELAY_MS);
      });
    };

    timerRef.current = setTimeout(fire, delay);

    const onVisibilityChange = () => {
      if (document.hidden) return;
      const remaining = computeRefreshDelayMs(token, Date.now());
      if (dueWhileHiddenRef.current || (remaining !== null && remaining <= MIN_DELAY_MS)) {
        dueWhileHiddenRef.current = false;
        fire();
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [token]);
}
