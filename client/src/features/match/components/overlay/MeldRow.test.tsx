import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { Card } from "@/shared/types/matchTypes";

import { CARD_SIZES } from "../../lib/cardFace";
import { MeldRow } from "./MeldRow";

const QUARTE: Card[] = [
  { rank: "9", suit: "H" },
  { rank: "T", suit: "H" },
  { rank: "J", suit: "H" },
  { rank: "Q", suit: "H" },
];

const EIGHT_RUN: Card[] = (["7", "8", "9", "T", "J", "Q", "K", "A"] as const).map((rank) => ({
  rank,
  suit: "S",
}));

function overlapBefore(cardId: string): number {
  const wrapper = screen.getByTestId(`playing-card-${cardId}`).parentElement;
  return -Number.parseFloat(wrapper?.style.marginLeft ?? "0");
}

function fanWidth(count: number, overlap: number): number {
  return CARD_SIZES.sm.width + (CARD_SIZES.sm.width - overlap) * (count - 1);
}

describe("MeldRow", () => {
  it("renders the label, byline and the value as +points", () => {
    render(
      <MeldRow
        label="Quarte"
        cards={QUARTE}
        value={50}
        byline={<span>by alice</span>}
        testId="meld"
      />,
    );

    expect(screen.getByTestId("meld-label")).toHaveTextContent("Quarte");
    expect(screen.getByTestId("meld")).toHaveTextContent("by alice");
    expect(screen.getByTestId("meld-value")).toHaveTextContent("+50");
  });

  it("fans every card of the meld", () => {
    render(<MeldRow label="Quarte" cards={QUARTE} value={50} />);

    for (const card of QUARTE) {
      expect(screen.getByTestId(`playing-card-${card.rank}${card.suit}`)).toBeInTheDocument();
    }
  });

  it("tightens the fan of a long run to the width of a five-card fan", () => {
    const { unmount } = render(<MeldRow label="Quarte" cards={QUARTE} value={50} />);
    const fullOverlap = overlapBefore("TH");
    unmount();

    render(<MeldRow label="Run" cards={EIGHT_RUN} value={100} />);
    const runOverlap = overlapBefore("8S");

    expect(runOverlap).toBeGreaterThan(fullOverlap);
    expect(fanWidth(8, runOverlap)).toBeCloseTo(fanWidth(5, fullOverlap));
  });
});
