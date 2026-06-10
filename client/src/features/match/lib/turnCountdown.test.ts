import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { recordServerNow, resetClockSyncForTest } from "@/shared/lib/clockSync";

import { ringDrainStyle, useRingDrain, useTurnCountdown } from "./turnCountdown";

describe("useTurnCountdown", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(0);
    resetClockSyncForTest();
  });

  afterEach(() => {
    resetClockSyncForTest();
    vi.useRealTimers();
  });

  it("counts true deadline seconds and reaches 0 at the deadline", () => {
    const expiresAt = new Date(5500).toISOString();
    const { result } = renderHook(() => useTurnCountdown(expiresAt));

    expect(result.current).toBe(6); // ceil(5.5s)

    // Ticks align to the deadline's whole-second boundaries (500ms phase),
    // not to a free-running interval.
    act(() => vi.advanceTimersByTime(520));
    expect(result.current).toBe(5);

    act(() => vi.advanceTimersByTime(1020));
    expect(result.current).toBe(4);

    // At the deadline the label truly shows 0 — it never parks on 1.
    act(() => vi.advanceTimersByTime(4000));
    expect(result.current).toBe(0);
  });

  it("measures remaining time on the server's clock, not the local one", () => {
    // Server is 2s ahead: a deadline 5s away locally is 3s away for real.
    recordServerNow(new Date(2000).toISOString());
    const { result } = renderHook(() => useTurnCountdown(new Date(5000).toISOString()));
    expect(result.current).toBe(3);
  });

  it("returns 0 for null and for already-past deadlines", () => {
    const { result: nullResult } = renderHook(() => useTurnCountdown(null));
    expect(nullResult.current).toBe(0);

    const { result: pastResult } = renderHook(() =>
      useTurnCountdown(new Date(-1000).toISOString()),
    );
    expect(pastResult.current).toBe(0);
  });
});

describe("useRingDrain / ringDrainStyle", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(0);
    resetClockSyncForTest();
  });

  afterEach(() => {
    resetClockSyncForTest();
    vi.useRealTimers();
  });

  it("returns undefined without a deadline or duration", () => {
    expect(renderHook(() => useRingDrain(null, 30)).result.current).toBeUndefined();
    expect(
      renderHook(() => useRingDrain(new Date(5000).toISOString(), 0)).result.current,
    ).toBeUndefined();
  });

  it("anchors the sweep to the deadline via a negative delay", () => {
    // 10s left of a 30s window: animation is 20s in.
    const { result } = renderHook(() => useRingDrain(new Date(10_000).toISOString(), 30, 219.9));
    expect(result.current).toMatchObject({
      "--ring-empty": "219.9",
      animationName: "ring-drain",
      animationDuration: "30000ms",
      animationDelay: "-20000ms",
      animationTimingFunction: "linear",
      animationFillMode: "forwards",
    });
  });

  it("stays end-anchored when skew makes remaining exceed the window", () => {
    // Remaining (35s) exceeding the window (30s): the sweep stretches over
    // the longer remaining (delay 0) so the arc still empties AT the
    // deadline — never early, never scheduled in the future.
    const style = ringDrainStyle(30_000, 35_000);
    expect(style.animationDuration).toBe("35000ms");
    expect(style.animationDelay).toBe("0ms");
  });

  it("parks an already-expired ring on empty", () => {
    const style = ringDrainStyle(30_000, 0);
    expect(style.animationDelay).toBe("-30000ms");
    expect(style.animationFillMode).toBe("forwards");
  });
});
