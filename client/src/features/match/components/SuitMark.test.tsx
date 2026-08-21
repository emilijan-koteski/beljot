import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "../lib/cardFace";
import { SUIT_GLYPH } from "../lib/suitArt";
import { SuitMark } from "./SuitMark";

const SUITS: Suit[] = ["S", "H", "D", "C"];
const DECKS: CardDeck[] = ["french", "croatian"];

describe("SuitMark", () => {
  it.each(SUITS)("renders the french Unicode glyph for %s, coloured by accent", (suit) => {
    render(<SuitMark suit={suit} deck="french" size={22} />);

    const mark = screen.getByTestId(`suit-mark-${suit}`);
    expect(mark.tagName).toBe("SPAN");
    expect(mark).toHaveTextContent(SUIT_GLYPH[suit]);
    expect(mark).toHaveAttribute("data-deck", "french");
    expect(mark.style.fontSize).toBe("22px");
  });

  it.each(SUITS)("renders the croatian icon img for %s", (suit) => {
    render(<SuitMark suit={suit} deck="croatian" size={30} />);

    const mark = screen.getByTestId(`suit-mark-${suit}`);
    expect(mark.tagName).toBe("IMG");
    expect(mark).toHaveAttribute("src", `/suits/croatian/${suit}.webp`);
    expect(mark).toHaveAttribute("data-deck", "croatian");
    // Both decks occupy the same box at the same `size`, so a surface does not
    // reflow when the player switches decks.
    expect(mark.style.width).toBe("30px");
    expect(mark.style.height).toBe("30px");
  });

  it("does not tint the croatian icon — the art carries its own ink", () => {
    render(<SuitMark suit="D" deck="croatian" size={30} />);
    expect(screen.getByTestId("suit-mark-D").style.color).toBe("");
  });

  // Every call site already names the suit on an ancestor (chip aria-label,
  // button aria-label, seal aria-label, seat chip title), so the mark itself
  // must not be announced a second time.
  it.each(DECKS)("is decorative on the %s deck", (deck) => {
    render(<SuitMark suit="H" deck={deck} size={18} />);
    expect(screen.getByTestId("suit-mark-H")).toHaveAttribute("aria-hidden", "true");
  });

  it("gives the croatian icon an empty alt", () => {
    render(<SuitMark suit="H" deck="croatian" size={18} />);
    expect(screen.getByTestId("suit-mark-H")).toHaveAttribute("alt", "");
  });

  it("hides a croatian icon that cannot load, leaving the surrounding chrome", () => {
    // Production answers a bad asset path with index.html at 200, which browsers
    // paint as a broken-image glyph — inside a 44 px parchment orb that is worse
    // than an empty orb.
    render(<SuitMark suit="C" deck="croatian" size={30} />);

    const mark = screen.getByTestId("suit-mark-C");
    expect(mark.style.visibility).toBe("");

    fireEvent.error(mark);

    expect(mark.style.visibility).toBe("hidden");
  });

  it("applies the caller's className to a glyph and its style to both decks", () => {
    const { unmount } = render(
      <SuitMark
        suit="S"
        deck="french"
        size={22}
        className="font-display"
        style={{ opacity: 0.5 }}
      />,
    );
    expect(screen.getByTestId("suit-mark-S").className).toContain("font-display");
    expect(screen.getByTestId("suit-mark-S").style.opacity).toBe("0.5");
    unmount();

    render(<SuitMark suit="S" deck="croatian" size={22} style={{ opacity: 0.5 }} />);
    expect(screen.getByTestId("suit-mark-S").style.opacity).toBe("0.5");
  });
});
