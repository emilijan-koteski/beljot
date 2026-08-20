import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MOTION } from "@/shared/lib/motion";

import { DealAnimation } from "./DealAnimation";

beforeEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  });
});

describe("DealAnimation", () => {
  it("renders deal animation container", () => {
    render(<DealAnimation trumpCandidate={{ rank: "K", suit: "H" }} />);
    expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
  });

  it("shows trump candidate card when available", () => {
    vi.useFakeTimers();
    render(<DealAnimation trumpCandidate={{ rank: "K", suit: "H" }} />);
    // Advance to revealing phase
    vi.advanceTimersByTime(900);
    expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
    vi.useRealTimers();
  });

  it("renders without trump candidate", () => {
    render(<DealAnimation trumpCandidate={null} />);
    expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
  });

  // Story 12.8: with no candidate the reveal phase has nothing to show, so the
  // component used to sit invisible on an empty table centre for the whole
  // trump-flip beat before bidding opened. It must be gone the moment the deal
  // beat ends.
  it("ends at the deal beat when there is no trump candidate to flip", () => {
    vi.useFakeTimers();
    try {
      render(<DealAnimation trumpCandidate={null} />);
      expect(screen.getByTestId("deal-animation")).toBeInTheDocument();

      // Read the constant rather than restating it: the assertion is "still on
      // screen right up to the deal beat, gone just after", which must follow
      // the constant if it is ever retuned.
      act(() => vi.advanceTimersByTime(MOTION.DEAL_PHASE_DEAL - 1));
      expect(screen.getByTestId("deal-animation")).toBeInTheDocument();

      act(() => vi.advanceTimersByTime(2));
      expect(screen.queryByTestId("deal-animation")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps the trump-flip beat when there IS a candidate", () => {
    vi.useFakeTimers();
    try {
      render(<DealAnimation trumpCandidate={{ rank: "K", suit: "H" }} />);

      // Past the deal beat the candidate is on the table, not gone.
      act(() => vi.advanceTimersByTime(MOTION.DEAL_PHASE_DEAL + 100));
      expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
      expect(screen.getByTestId("playing-card-KH")).toBeInTheDocument();

      // And it stays rendered after `done`, because the candidate is still
      // face-up on the table (the self-hide only applies with no candidate).
      act(() => vi.advanceTimersByTime(MOTION.DEAL_PHASE_TRUMP));
      expect(screen.getByTestId("deal-animation")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("has aria-label for deal animation", () => {
    render(<DealAnimation trumpCandidate={{ rank: "K", suit: "H" }} />);
    expect(screen.getByTestId("deal-animation")).toHaveAttribute("aria-label");
  });

  it("skips animation instantly when prefers-reduced-motion is set", () => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });
    render(<DealAnimation trumpCandidate={{ rank: "K", suit: "H" }} />);
    // With reduced motion, deal phase goes to done immediately
  });
});
