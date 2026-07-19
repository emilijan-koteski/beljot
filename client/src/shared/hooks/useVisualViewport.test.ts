import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { useVisualViewport } from "./useVisualViewport";

/** Minimal stand-in for window.visualViewport: an EventTarget with mutable metrics. */
function installMockViewport(height: number, offsetTop = 0) {
  const target = new EventTarget() as EventTarget & { height: number; offsetTop: number };
  target.height = height;
  target.offsetTop = offsetTop;
  Object.defineProperty(window, "visualViewport", {
    value: target,
    configurable: true,
    writable: true,
  });
  return target;
}

function removeMockViewport() {
  Object.defineProperty(window, "visualViewport", {
    value: undefined,
    configurable: true,
    writable: true,
  });
}

describe("useVisualViewport", () => {
  afterEach(() => {
    removeMockViewport();
  });

  it("returns null when the VisualViewport API is unavailable", () => {
    removeMockViewport();
    const { result } = renderHook(() => useVisualViewport(true));
    expect(result.current).toBeNull();
  });

  it("returns null while disabled even when the API exists", () => {
    installMockViewport(640);
    const { result } = renderHook(() => useVisualViewport(false));
    expect(result.current).toBeNull();
  });

  it("reports the current viewport box when enabled", () => {
    installMockViewport(640, 0);
    const { result } = renderHook(() => useVisualViewport(true));
    expect(result.current).toEqual({ height: 640, offsetTop: 0 });
  });

  it("updates on viewport resize and scroll (keyboard open + pan)", () => {
    const vv = installMockViewport(640, 0);
    const { result } = renderHook(() => useVisualViewport(true));

    act(() => {
      vv.height = 320;
      vv.dispatchEvent(new Event("resize"));
    });
    expect(result.current).toEqual({ height: 320, offsetTop: 0 });

    act(() => {
      vv.offsetTop = 48;
      vv.dispatchEvent(new Event("scroll"));
    });
    expect(result.current).toEqual({ height: 320, offsetTop: 48 });
  });

  it("clears the box when disabled after being enabled", () => {
    installMockViewport(640);
    const { result, rerender } = renderHook(({ enabled }) => useVisualViewport(enabled), {
      initialProps: { enabled: true },
    });
    expect(result.current).not.toBeNull();
    rerender({ enabled: false });
    expect(result.current).toBeNull();
  });

  it("stops listening after unmount", () => {
    const vv = installMockViewport(640, 0);
    const { result, unmount } = renderHook(() => useVisualViewport(true));
    expect(result.current).toEqual({ height: 640, offsetTop: 0 });
    unmount();
    act(() => {
      vv.height = 320;
      vv.dispatchEvent(new Event("resize"));
    });
    // No crash / no state update on an unmounted hook — nothing to assert
    // beyond the absence of act() warnings, which vitest surfaces as errors.
  });
});
