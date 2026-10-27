import { describe, expect, it } from "vitest";

import { overlayDimBackground } from "./overlayDim";

describe("overlayDimBackground", () => {
  it("returns one radial wash per level, darker at the edge than the centre", () => {
    expect(overlayDimBackground("prompt")).toBe(
      "radial-gradient(ellipse at center, rgba(0,0,0,0.4) 0%, rgba(0,0,0,0.5) 100%)",
    );
    expect(overlayDimBackground("blocker")).toBe(
      "radial-gradient(ellipse at center, rgba(0,0,0,0.56) 0%, rgba(0,0,0,0.7) 100%)",
    );
    expect(overlayDimBackground("spotlight")).toBe(
      "radial-gradient(ellipse at center, rgba(0,0,0,0.66) 0%, rgba(0,0,0,0.82) 100%)",
    );
  });
});
