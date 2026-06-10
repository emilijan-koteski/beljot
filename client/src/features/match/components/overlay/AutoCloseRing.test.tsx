import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AutoCloseRing } from "./AutoCloseRing";

let mockReducedMotion = false;
vi.mock("@/shared/hooks/useReducedMotion", () => ({
  useReducedMotion: () => mockReducedMotion,
}));

const RING_CIRC = 2 * Math.PI * 9;

describe("AutoCloseRing", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockReducedMotion = false;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("fires onClose exactly once when the duration elapses", () => {
    const onClose = vi.fn();
    render(<AutoCloseRing onClose={onClose} />);

    act(() => vi.advanceTimersByTime(7_900));
    expect(onClose).not.toHaveBeenCalled();

    act(() => vi.advanceTimersByTime(200));
    expect(onClose).toHaveBeenCalledTimes(1);

    act(() => vi.advanceTimersByTime(10_000));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("fires onClose once on click and suppresses the later auto-fire", () => {
    const onClose = vi.fn();
    render(<AutoCloseRing onClose={onClose} />);

    fireEvent.click(screen.getByTestId("auto-close-ring"));
    expect(onClose).toHaveBeenCalledTimes(1);

    act(() => vi.advanceTimersByTime(10_000));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("never auto-fires while paused", () => {
    const onClose = vi.fn();
    render(<AutoCloseRing onClose={onClose} paused />);

    act(() => vi.advanceTimersByTime(20_000));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("drives the arc with a mount-anchored drain matching the fire timer", () => {
    render(<AutoCloseRing duration={8} onClose={vi.fn()} />);

    const arc = screen.getByTestId("auto-close-ring-arc") as unknown as SVGCircleElement;
    // The sweep and the setTimeout fire share duration and start commit, so
    // the ring reads empty at the exact moment the close fires.
    expect(arc.style.animationName).toBe("ring-drain");
    expect(arc.style.animationDuration).toBe("8000ms");
    expect(arc.style.animationDelay).toBe("0ms");
    expect(arc.style.animationFillMode).toBe("forwards");
    expect(arc.style.transition).not.toContain("stroke-dashoffset");
  });

  it("steps the arc statically per second under reduced motion", () => {
    mockReducedMotion = true;
    const onClose = vi.fn();
    render(<AutoCloseRing duration={8} onClose={onClose} />);

    const arc = screen.getByTestId("auto-close-ring-arc") as unknown as SVGCircleElement;
    expect(arc.style.animationName).toBe("");
    expect(parseFloat(arc.style.strokeDashoffset)).toBe(0); // full at mount

    act(() => vi.advanceTimersByTime(4_000));
    // Half the window elapsed — the static fallback has stepped to ~half drained.
    expect(parseFloat(arc.style.strokeDashoffset)).toBeCloseTo(RING_CIRC / 2, 1);

    act(() => vi.advanceTimersByTime(4_000));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
