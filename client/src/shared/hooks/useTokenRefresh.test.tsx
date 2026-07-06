import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";

import { computeRefreshDelayMs, decodeJwtTiming, useTokenRefresh } from "./useTokenRefresh";

const refreshAccessToken = vi.fn();

vi.mock("@/shared/api/axiosClient", () => ({
  refreshAccessToken: () => refreshAccessToken(),
  subscribeToTokenBroadcast: () => () => {},
}));

// makeJwt builds a header.payload.signature string whose payload carries iat/exp
// (epoch seconds). The signature is irrelevant — the hook only decodes the
// payload, never verifies it (the server does that).
function makeJwt(iatSec: number, expSec: number): string {
  const payload = btoa(JSON.stringify({ iat: iatSec, exp: expSec }));
  return `header.${payload}.sig`;
}

function setHidden(hidden: boolean) {
  Object.defineProperty(document, "hidden", { configurable: true, get: () => hidden });
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => (hidden ? "hidden" : "visible"),
  });
}

describe("decodeJwtTiming", () => {
  it("extracts exp and iat from a JWT", () => {
    expect(decodeJwtTiming(makeJwt(1000, 1900))).toEqual({ iat: 1000, exp: 1900 });
  });

  it("returns null for non-JWT strings", () => {
    expect(decodeJwtTiming("not-a-jwt")).toBeNull();
    expect(decodeJwtTiming("a.b.c")).toBeNull();
  });
});

describe("computeRefreshDelayMs", () => {
  it("schedules at 75% of the token lifetime", () => {
    const nowMs = 1000 * 1000; // iat, in ms
    // 900s lifetime → refresh at +675s.
    expect(computeRefreshDelayMs(makeJwt(1000, 1900), nowMs)).toBe(675_000);
  });

  it("floors to the minimum delay when already past the threshold", () => {
    const nowMs = 1900 * 1000; // at expiry
    expect(computeRefreshDelayMs(makeJwt(1000, 1900), nowMs)).toBe(5_000);
  });

  it("returns null for an undecodable token", () => {
    expect(computeRefreshDelayMs("garbage", Date.now())).toBeNull();
  });
});

describe("useTokenRefresh", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    refreshAccessToken.mockReset();
    refreshAccessToken.mockResolvedValue("new-token");
    setHidden(false);
  });

  afterEach(() => {
    // Reset inside act(): the hook is still mounted here (auto-cleanup order is
    // not guaranteed), so a bare setState would re-render it outside act().
    act(() => {
      useAuthStore.setState({ token: null, user: null, isLoading: false });
    });
    vi.useRealTimers();
    setHidden(false);
  });

  it("refreshes proactively before expiry while the tab is visible", () => {
    const nowSec = 1_000_000;
    vi.setSystemTime(nowSec * 1000);
    useAuthStore.setState({ token: makeJwt(nowSec, nowSec + 900), user: null, isLoading: false });

    renderHook(() => useTokenRefresh());
    expect(refreshAccessToken).not.toHaveBeenCalled();

    // Advance just past the 75% (675s) mark.
    act(() => {
      vi.advanceTimersByTime(676_000);
    });
    expect(refreshAccessToken).toHaveBeenCalledTimes(1);
  });

  it("does not refresh while the tab is hidden", () => {
    const nowSec = 1_000_000;
    vi.setSystemTime(nowSec * 1000);
    setHidden(true);
    useAuthStore.setState({ token: makeJwt(nowSec, nowSec + 900), user: null, isLoading: false });

    renderHook(() => useTokenRefresh());
    act(() => {
      vi.advanceTimersByTime(1_000_000);
    });
    expect(refreshAccessToken).not.toHaveBeenCalled();
  });

  it("does not schedule anything without a token", () => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    renderHook(() => useTokenRefresh());
    act(() => {
      vi.advanceTimersByTime(1_000_000);
    });
    expect(refreshAccessToken).not.toHaveBeenCalled();
  });

  it("re-arms a retry after a failed proactive refresh", async () => {
    const nowSec = 1_000_000;
    vi.setSystemTime(nowSec * 1000);
    refreshAccessToken.mockRejectedValueOnce(new Error("network")).mockResolvedValue("new-token");
    useAuthStore.setState({ token: makeJwt(nowSec, nowSec + 900), user: null, isLoading: false });

    renderHook(() => useTokenRefresh());

    // First proactive fire (~675s in) rejects.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(676_000);
    });
    expect(refreshAccessToken).toHaveBeenCalledTimes(1);

    // The re-armed retry fires ~30s later — a single blip must not disable
    // proactive renewal for the rest of the token's life.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(31_000);
    });
    expect(refreshAccessToken).toHaveBeenCalledTimes(2);
  });
});
