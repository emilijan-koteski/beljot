import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { TooltipProvider } from "@/shared/components/ui/tooltip";
import { Z } from "@/shared/lib/zLayers";

import { DiffMarker } from "./DiffMarker";

function renderMarker(tone: "light" | "dark") {
  return render(
    <TooltipProvider delay={0}>
      <DiffMarker note="THE-OTHER-VARIANT" label="Differs" tone={tone} />
    </TooltipProvider>,
  );
}

/** The portalled positioner is the element whose stacking tier competes with the
 *  overlay panel — the popup inside it cannot escape its parent's z-index. */
function positionerZ(): number {
  const el = document.querySelector<HTMLElement>('[data-slot="tooltip-positioner"]');
  if (!el) throw new Error("tooltip positioner is not rendered");
  return Number(el.style.zIndex || 0);
}

describe("DiffMarker", () => {
  it("stacks above the in-match overlay when rendered in the dark tone", async () => {
    const user = userEvent.setup();
    renderMarker("dark");
    await user.hover(screen.getByTestId("rules-diff-marker"));
    expect(await screen.findByText("THE-OTHER-VARIANT")).toBeInTheDocument();
    // RulesDialog paints at Z.UTIL and both portal to document.body as
    // siblings, so anything at or below Z.UTIL is painted behind an ~98% opaque
    // panel. jsdom has no paint model, which is why this asserts the tier
    // rather than visibility — findByText passes either way.
    expect(positionerZ()).toBeGreaterThan(Z.UTIL);
  });

  it("leaves the light page on the primitive's default tier", async () => {
    const user = userEvent.setup();
    renderMarker("light");
    await user.hover(screen.getByTestId("rules-diff-marker"));
    expect(await screen.findByText("THE-OTHER-VARIANT")).toBeInTheDocument();
    // No competitor on the standalone page — no inline override.
    expect(positionerZ()).toBe(0);
  });
});
