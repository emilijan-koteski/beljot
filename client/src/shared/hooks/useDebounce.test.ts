import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useDebounce } from "@/shared/hooks/useDebounce";

describe("useDebounce", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("returns the initial value immediately", () => {
    const { result } = renderHook(() => useDebounce("a", 250));
    expect(result.current).toBe("a");
  });

  it("delays updates until the delay elapses", () => {
    const { result, rerender } = renderHook(({ v }) => useDebounce(v, 250), {
      initialProps: { v: "a" },
    });

    rerender({ v: "b" });
    expect(result.current).toBe("a");

    act(() => vi.advanceTimersByTime(249));
    expect(result.current).toBe("a");

    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe("b");
  });

  it("resets the timer on rapid changes so only the latest value lands", () => {
    const { result, rerender } = renderHook(({ v }) => useDebounce(v, 250), {
      initialProps: { v: "a" },
    });

    rerender({ v: "ab" });
    act(() => vi.advanceTimersByTime(200));
    rerender({ v: "abc" });
    // 400ms have passed since the first change, but only 200ms since the last —
    // still debouncing, so the value has not settled yet.
    act(() => vi.advanceTimersByTime(200));
    expect(result.current).toBe("a");

    act(() => vi.advanceTimersByTime(50));
    expect(result.current).toBe("abc");
  });
});
