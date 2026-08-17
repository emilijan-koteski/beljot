import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { HonorBandMeter } from "./HonorBandMeter";

function tick(label: string): HTMLElement {
  return screen.getByText(label, { selector: "span" });
}

describe("HonorBandMeter", () => {
  it("places the marker at the score's own position on the track", () => {
    render(<HonorBandMeter score={87} testId="meter" />);

    const marker = screen.getByTestId("meter-marker");
    expect(marker).toHaveAttribute("data-value", "87");
    expect(marker).toHaveStyle({ left: "87%" });
  });

  // The shipped tick row was `flex justify-between`, which spread the six labels
  // evenly (0/20/40/60/80/100% of the width) over a track that is linear in the
  // score: "95" sat at 80%, so an honest marker at 87% rendered to the RIGHT of
  // it and an 87 read as all but perfect. Labels and marker share one scale now.
  it("labels each band boundary at its true position, not evenly spread", () => {
    render(<HonorBandMeter score={87} testId="meter" />);

    expect(tick("0")).toHaveStyle({ left: "0%" });
    expect(tick("50")).toHaveStyle({ left: "50%" });
    expect(tick("70")).toHaveStyle({ left: "70%" });
    expect(tick("85")).toHaveStyle({ left: "85%" });
    expect(tick("95")).toHaveStyle({ left: "95%" });
    expect(tick("100")).toHaveStyle({ left: "100%" });
  });

  it("keeps the marker to the left of the next boundary label above it", () => {
    render(<HonorBandMeter score={87} testId="meter" />);

    const markerLeft = Number.parseFloat(screen.getByTestId("meter-marker").style.left);
    const exemplaryFloor = Number.parseFloat(tick("95").style.left);
    expect(markerLeft).toBeLessThan(exemplaryFloor);
  });

  it("renders no marker without a score, so the scale can be shown unmarked", () => {
    render(<HonorBandMeter testId="meter" />);

    expect(screen.queryByTestId("meter-marker")).not.toBeInTheDocument();
    expect(tick("95")).toBeInTheDocument();
  });
});
