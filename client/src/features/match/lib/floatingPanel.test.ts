import { describe, expect, it } from "vitest";

import { floatingPanelStyle } from "./floatingPanel";

describe("floatingPanelStyle", () => {
  it("draws a hairline brass edge, felt-md shadow and 8px blur by default", () => {
    expect(floatingPanelStyle()).toEqual({
      background: "var(--panel-dark)",
      border: "1px solid var(--brass-40)",
      boxShadow: "var(--shadow-felt-md)",
      backdropFilter: "blur(8px)",
      WebkitBackdropFilter: "blur(8px)",
    });
  });

  it("switches the edge for announcements and errors and lifts menus", () => {
    expect(floatingPanelStyle({ edge: "announce" }).border).toBe("1px solid var(--brass)");
    expect(floatingPanelStyle({ edge: "danger" }).border).toBe(
      "1px solid color-mix(in srgb, var(--danger-light) 40%, transparent)",
    );
    expect(floatingPanelStyle({ menu: true }).boxShadow).toBe("var(--shadow-felt-lg)");
  });
});
