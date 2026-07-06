import { useEffect, useRef } from "react";

import { refreshAccessToken, subscribeToTokenBroadcast } from "@/shared/api/axiosClient";
import { useAuthStore } from "@/shared/stores/authStore";

// Proactively refresh the access token once it has burned through this fraction
// of its lifetime — early enough that no request ever races its expiry.
const REFRESH_AT = 0.75;
const DEFAULT_LIFETIME_SEC = 15 * 60;
// Floor for the scheduled delay: if we're already past the refresh point we wait
// a few seconds rather than busy-refreshing.
const MIN_DELAY_MS = 5_000;
// After a failed proactive refresh, re-arm this soon so a single transient
// failure doesn't disable silent renewal for the rest of the token's life.
const RETRY_DELAY_MS = 30_000;

interface JwtTiming {
  exp: number;
  iat: number;
}

// decodeJwtTiming extracts exp/iat (epoch seconds) from a JWT without pulling in
// a decoding dependency. Returns null for anything that isn't a decodable JWT
// with a numeric exp.
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

// computeRefreshDelayMs returns how long from nowMs to wait before refreshing —
// REFRESH_AT of the token's lifetime, floored at MIN_DELAY_MS. null when the
// token can't be decoded (the caller then skips proactive scheduling and leans
// on the reactive 401 path).
export function computeRefreshDelayMs(token: string, nowMs: number): number | null {
  const timing = decodeJwtTiming(token);
  if (!timing) return null;
  const lifetimeMs = Math.max(0, (timing.exp - timing.iat) * 1000);
  const refreshAtMs = timing.iat * 1000 + lifetimeMs * REFRESH_AT;
  return Math.max(MIN_DELAY_MS, refreshAtMs - nowMs);
}

// useTokenRefresh keeps the in-memory access token fresh while the user is
// active: it schedules a silent refresh shortly before the token expires, but
// only fires while the tab is visible (a backgrounded tab defers until focused).
// Sibling tabs adopt the broadcast token instead of each refreshing. Mount once,
// near the app root.
export function useTokenRefresh(): void {
  const token = useAuthStore((s) => s.token);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const dueWhileHiddenRef = useRef(false);

  // Adopt tokens broadcast by sibling tabs so only one tab actually refreshes.
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
    if (delay === null) return; // undecodable — rely on the reactive 401 refresh

    const fire = () => {
      // Only refresh while the tab is in use; a hidden tab defers until it is
      // shown again (handled by the visibilitychange listener below).
      if (typeof document !== "undefined" && document.hidden) {
        dueWhileHiddenRef.current = true;
        return;
      }
      // On success, refreshAccessToken sets + broadcasts the new token, which
      // re-runs this effect (token changed) and reschedules. On a transient
      // failure the token is unchanged, so re-arm a short retry here — otherwise
      // one blip would leave this tab without proactive renewal until the token
      // actually expires (the reactive 401 path remains the ultimate backstop).
      void refreshAccessToken().catch(() => {
        if (timerRef.current) clearTimeout(timerRef.current);
        timerRef.current = setTimeout(fire, RETRY_DELAY_MS);
      });
    };

    timerRef.current = setTimeout(fire, delay);

    const onVisibilityChange = () => {
      if (document.hidden) return;
      const remaining = computeRefreshDelayMs(token, Date.now());
      // Refresh on focus if a scheduled refresh fired while hidden, or we're now
      // at/past the refresh threshold (delay clamped to its MIN_DELAY_MS floor).
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
