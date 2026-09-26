import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DealAnimation } from "./DealAnimation";

describe("DealAnimation", () => {
  it("announces the deal and shows nothing at the centre before the flip", () => {
    render(
      <DealAnimation flippedCandidate={null} isReshuffle={false} prefersReducedMotion={false} />,
    );

    expect(screen.getByRole("status")).toHaveTextContent("Dealing cards…");
    expect(screen.queryByTestId("deal-candidate")).not.toBeInTheDocument();
    expect(screen.queryByTestId("deal-reshuffle-caption")).not.toBeInTheDocument();
  });

  it("turns the candidate face-up once the deal flips it", () => {
    render(
      <DealAnimation
        flippedCandidate={{ rank: "K", suit: "H" }}
        isReshuffle={false}
        prefersReducedMotion={false}
      />,
    );

    const candidate = screen.getByTestId("deal-candidate");
    expect(candidate).toContainElement(screen.getByTestId("playing-card-KH"));
    expect(candidate.style.animation).toContain("deal-candidate-flip");
  });

  it("shows the candidate without the flip under reduced motion", () => {
    render(
      <DealAnimation
        flippedCandidate={{ rank: "K", suit: "H" }}
        isReshuffle={false}
        prefersReducedMotion={true}
      />,
    );

    expect(screen.getByTestId("deal-candidate").style.animation).toBe("");
  });

  it("says the deck is being reshuffled after an all-pass", () => {
    render(
      <DealAnimation flippedCandidate={null} isReshuffle={true} prefersReducedMotion={false} />,
    );

    expect(screen.getByTestId("deal-reshuffle-caption")).toHaveTextContent("Reshuffling deck…");
    expect(screen.getByRole("status")).toHaveTextContent("Reshuffling deck…");
  });
});
