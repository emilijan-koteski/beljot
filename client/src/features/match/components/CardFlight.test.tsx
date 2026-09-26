import "@/shared/i18n/i18n";

import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Card } from "@/shared/types/matchTypes";

import { CardFlight, type CardFlightDescriptor } from "./CardFlight";

const KS: Card = { rank: "K", suit: "S" };
const SEVEN_H: Card = { rank: "7", suit: "H" };

function descriptor(overrides: Partial<CardFlightDescriptor> = {}): CardFlightDescriptor {
  return {
    id: "throw-self-KS-1",
    card: KS,
    fromRect: { left: 100, top: 600, width: 88, height: 128 },
    toRect: { left: 500, top: 380, width: 72, height: 104 },
    durationMs: 520,
    ...overrides,
  };
}

describe("CardFlight", () => {
  it("renders nothing when there are no flights", () => {
    const { container } = render(<CardFlight flights={[]} onComplete={vi.fn()} />);
    // Empty render — overlay does not mount when there's nothing in flight,
    // so testing libraries do not find the data-testid.
    expect(container.querySelector('[data-testid="card-flight-overlay"]')).toBeNull();
  });

  it("renders a flying card per flight", () => {
    render(
      <CardFlight
        flights={[descriptor(), descriptor({ id: "throw-opp-1-7H-2", card: SEVEN_H })]}
        onComplete={vi.fn()}
      />,
    );

    expect(screen.getByTestId("card-flight-overlay")).toBeInTheDocument();
    expect(screen.getByTestId("card-flight-throw-self-KS-1")).toBeInTheDocument();
    expect(screen.getByTestId("card-flight-throw-opp-1-7H-2")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
  });

  it("flies a face-down back when the flight carries no card", () => {
    render(
      <CardFlight
        flights={[descriptor({ id: "deal-1:0-0-0", card: null })]}
        onComplete={vi.fn()}
      />,
    );

    const flight = screen.getByTestId("card-flight-deal-1:0-0-0-animated");
    expect(flight.querySelector('[data-testid="playing-card-facedown"]')).not.toBeNull();
    expect(
      flight.querySelector(
        '[data-testid^="playing-card-"]:not([data-testid="playing-card-facedown"])',
      ),
    ).toBeNull();
  });

  it("emits a uniquely-named keyframe per flight", () => {
    render(<CardFlight flights={[descriptor()]} onComplete={vi.fn()} />);

    const overlay = screen.getByTestId("card-flight-overlay");
    const styleTag = overlay.querySelector("style");
    expect(styleTag).not.toBeNull();
    // The keyframe name embeds the per-flight safeId; we don't pin the
    // animScope (useId-driven), but the tail must contain the flight id
    // sanitized of dashes etc.
    expect(styleTag!.textContent).toMatch(/@keyframes cardFlight_.*throw_self_KS_1/);
  });

  it("calls onComplete when the animation ends", () => {
    const onComplete = vi.fn();
    render(<CardFlight flights={[descriptor({ id: "test-1" })]} onComplete={onComplete} />);

    const animatedEl = screen.getByTestId("card-flight-test-1-animated");
    // The handler is attached natively via addEventListener (see FlightCard
    // — works around React 19 + jsdom synthetic event flake), so a plain
    // bubbling event triggers it. Wrap in act() so the parent's setActiveFlights
    // updates downstream of onComplete are flushed before the assertion.
    act(() => {
      animatedEl.dispatchEvent(new Event("animationend", { bubbles: true }));
    });

    expect(onComplete).toHaveBeenCalledWith("test-1");
  });
});
