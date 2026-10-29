import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { mockViewportWidth } from "@/test-utils";

import { useLayoutBand } from "./useLayoutBand";

let restoreViewport = () => {};

afterEach(() => {
  restoreViewport();
  restoreViewport = () => {};
});

describe("useLayoutBand", () => {
  it.each([
    [390, "phone"],
    [767, "phone"],
    [768, "tablet"],
    [1023, "tablet"],
    [1024, "desktop"],
    [2560, "desktop"],
  ])("reports a %ipx window as %s", (width, band) => {
    restoreViewport = mockViewportWidth(width);
    const { result } = renderHook(() => useLayoutBand());
    expect(result.current).toBe(band);
  });

  it("falls back to the phone layout when matchMedia is unavailable", () => {
    const { result } = renderHook(() => useLayoutBand());
    expect(result.current).toBe("phone");
  });
});
