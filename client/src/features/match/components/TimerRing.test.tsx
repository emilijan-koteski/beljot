import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TimerRing } from "./TimerRing";

describe("TimerRing", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns null when turnExpiresAt is null", () => {
    const { container } = render(<TimerRing turnExpiresAt={null} totalDuration={30} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders countdown from turnExpiresAt", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 25000); // 25 seconds from now

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    expect(screen.getByTestId("timer-ring")).toBeInTheDocument();
    const seconds = screen.getByTestId("timer-seconds");
    expect(Number(seconds.textContent)).toBeGreaterThanOrEqual(24);
    expect(Number(seconds.textContent)).toBeLessThanOrEqual(26);
  });

  it("renders lime color while above the 1/8 urgency threshold", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 20000); // 20s of 30s = 66.6%

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const seconds = screen.getByTestId("timer-seconds");
    expect(seconds.style.color).toContain("--turn-lime");
    const ring = screen.getByTestId("timer-ring");
    expect(ring.dataset.urgent).toBe("false");
  });

  it("stays lime when above 1/8 even though the old 25% rule would flip", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 6000); // 6s of 30s = 20% — above 1/8

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const seconds = screen.getByTestId("timer-seconds");
    expect(seconds.style.color).toContain("--turn-lime");
    const ring = screen.getByTestId("timer-ring");
    expect(ring.dataset.urgent).toBe("false");
  });

  it("flips to urgent red when ≤1/8 of the turn timer remains", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 3000); // 3s of 30s = 10% — urgent

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const seconds = screen.getByTestId("timer-seconds");
    expect(seconds.style.color).toContain("--turn-urgent");
    const ring = screen.getByTestId("timer-ring");
    expect(ring.dataset.urgent).toBe("true");
    expect(ring.className).toContain("motion-safe:animate-pulse");
  });

  it("stays red when expired (0 seconds), but stops pulsing", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() - 1000); // already expired

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const seconds = screen.getByTestId("timer-seconds");
    expect(seconds.style.color).toContain("--turn-urgent");
    expect(seconds.textContent).toBe("0");
    const ring = screen.getByTestId("timer-ring");
    expect(ring.className).not.toContain("motion-safe:animate-pulse");
  });

  it("decrements countdown over time", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 15000); // 15 seconds

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const seconds = screen.getByTestId("timer-seconds");
    const initialValue = Number(seconds.textContent);

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    const newValue = Number(seconds.textContent);
    expect(newValue).toBeLessThan(initialValue);
    expect(newValue).toBeGreaterThanOrEqual(initialValue - 4);
  });

  it("has accessible aria-label with seconds remaining", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 20000);

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const ring = screen.getByTestId("timer-ring");
    expect(ring.getAttribute("aria-label")).toMatch(/\d+ seconds remaining/);
  });

  it("does not pulse while plenty of time remains", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 20000); // 20s of 30s = 66.6%

    render(<TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />);

    const ring = screen.getByTestId("timer-ring");
    expect(ring.className).not.toContain("motion-safe:animate-pulse");
  });

  it("renders 70px svg in seat size variant (sits inside the 80px avatar frame)", () => {
    const expiry = new Date(Date.now() + 20000);
    const { container } = render(
      <TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />,
    );
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("width")).toBe("70");
    expect(svg?.getAttribute("height")).toBe("70");
    expect(screen.getByTestId("timer-ring").getAttribute("data-size")).toBe("seat");
  });

  it("renders compact 36px svg in button size variant", () => {
    const expiry = new Date(Date.now() + 20000);
    const { container } = render(
      <TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} size="button" />,
    );
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("width")).toBe("36");
    expect(svg?.getAttribute("height")).toBe("36");
    expect(screen.getByTestId("timer-ring").getAttribute("data-size")).toBe("button");
    expect(screen.getByTestId("timer-seconds").className).toContain("text-[10px]");
  });

  it("drives the arc with the deadline-anchored ring-drain animation", () => {
    vi.setSystemTime(0);
    const expiry = new Date(10_000).toISOString(); // 10s left of a 30s window

    render(<TimerRing turnExpiresAt={expiry} totalDuration={30} />);

    const arc = screen.getByTestId("timer-ring-arc") as unknown as SVGCircleElement;
    // One wall-clock-anchored animation owns the sweep — never a per-tick
    // stroke-dashoffset transition, which rendered the arc one tick behind
    // the truth (the bug where timers visibly fired with ~1-2s remaining).
    expect(arc.style.animationName).toBe("ring-drain");
    expect(arc.style.animationDuration).toBe("30000ms");
    expect(arc.style.animationDelay).toBe("-20000ms");
    expect(arc.style.animationFillMode).toBe("forwards");
    expect(arc.style.transition).not.toContain("stroke-dashoffset");
  });

  it("uses motion-safe prefix for transitions (reduced-motion safe)", () => {
    const now = new Date();
    const expiry = new Date(now.getTime() + 5000); // urgent state

    const { container } = render(
      <TimerRing turnExpiresAt={expiry.toISOString()} totalDuration={30} />,
    );

    const svg = container.querySelector("svg");
    const svgClass = svg?.getAttribute("class") ?? "";
    expect(svgClass).toContain("motion-safe:");
    expect(svgClass).toContain("motion-reduce:");
  });
});
