import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Z } from "@/shared/lib/zLayers";

import { DiffMarker } from "./DiffMarker";

function renderMarker(tone: "light" | "dark") {
  return render(<DiffMarker note="THE-OTHER-VARIANT" label="Differs" tone={tone} />);
}

/** The portalled positioner is the element whose stacking tier competes with the overlay panel. */
function positionerZ(): number {
  const el = document.querySelector<HTMLElement>('[data-slot="info-popover-positioner"]');
  if (!el) throw new Error("info popover positioner is not rendered");
  return Number(el.style.zIndex || 0);
}

describe("DiffMarker", () => {
  it("stacks above the in-match overlay when rendered in the dark tone", async () => {
    const user = userEvent.setup();
    renderMarker("dark");
    await user.hover(screen.getByTestId("rules-diff-marker"));
    expect(await screen.findByText("THE-OTHER-VARIANT")).toBeInTheDocument();
    expect(positionerZ()).toBeGreaterThan(Z.UTIL);
  });

  it("leaves the light page on the primitive's default tier", async () => {
    const user = userEvent.setup();
    renderMarker("light");
    await user.hover(screen.getByTestId("rules-diff-marker"));
    expect(await screen.findByText("THE-OTHER-VARIANT")).toBeInTheDocument();
    expect(positionerZ()).toBe(0);
  });

  it("opens on tap for touch screens", async () => {
    const user = userEvent.setup();
    renderMarker("light");
    await user.click(screen.getByTestId("rules-diff-marker"));
    expect(await screen.findByText("THE-OTHER-VARIANT")).toBeInTheDocument();
  });
});
