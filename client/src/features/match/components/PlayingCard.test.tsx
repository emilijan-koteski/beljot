import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { Card } from "@/shared/types/matchTypes";

import { PlayingCard } from "./PlayingCard";

const kingOfSpades: Card = { rank: "K", suit: "S" };
const tenOfHearts: Card = { rank: "T", suit: "H" };
const queenOfDiamonds: Card = { rank: "Q", suit: "D" };

/** The face artwork layer inside a card wrapper (see `public/cards/`). */
function faceImage(card: HTMLElement): HTMLImageElement | null {
  return card.querySelector("img");
}

describe("PlayingCard", () => {
  it("renders face-up card with the deck artwork for its card ID", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const card = screen.getByTestId("playing-card-KS");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/KS.svg");
  });

  it("uses the rank character in the asset name, not the '10' display label", () => {
    render(<PlayingCard card={tenOfHearts} state="default" size="md" />);

    const card = screen.getByTestId("playing-card-TH");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/TH.svg");
  });

  it("renders court cards from their own artwork file", () => {
    render(<PlayingCard card={queenOfDiamonds} state="default" size="lg" />);

    const card = screen.getByTestId("playing-card-QD");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/QD.svg");
  });

  it("keeps the face artwork decorative so the wrapper label is the only announcement", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const image = faceImage(screen.getByTestId("playing-card-KS"));
    expect(image).toHaveAttribute("alt", "");
    expect(image).toHaveAttribute("aria-hidden", "true");
  });

  it("renders face-down card with no artwork and no suit/rank visible", () => {
    render(<PlayingCard card={null} state="face-down" size="md" />);

    const card = screen.getByTestId("playing-card-facedown");
    expect(faceImage(card)).toBeNull();
    expect(card).not.toHaveTextContent("K");
    expect(card).not.toHaveTextContent("\u2660");
  });

  it("has lime halo and cursor-pointer when playable", () => {
    render(<PlayingCard card={kingOfSpades} state="playable" size="md" />);

    const card = screen.getByTestId("playing-card-KS");
    expect(card.className).toContain("cursor-pointer");
    // Lime turn-signal halo is applied inline (channel is independent of theme)
    expect(card.style.boxShadow).toContain("var(--turn-lime");
  });

  it("stays at full opacity when unplayable but blocks the cursor (per design — visible, not transparent)", () => {
    render(<PlayingCard card={kingOfSpades} state="unplayable" size="md" />);

    const card = screen.getByTestId("playing-card-KS");
    expect(card.className).not.toContain("opacity-40");
    expect(card.className).not.toContain("grayscale");
    expect(card.className).toContain("motion-safe:translate-y-[4px]");
    expect(card.className).toContain("cursor-not-allowed");
  });

  it("is raised above baseline when playable", () => {
    render(<PlayingCard card={kingOfSpades} state="playable" size="md" />);

    const card = screen.getByTestId("playing-card-KS");
    expect(card.className).toContain("motion-safe:translate-y-[-10px]");
    expect(card.className).toContain("motion-safe:hover:translate-y-[-14px]");
  });

  it("calls onClick only when state is playable", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    const { rerender } = render(
      <PlayingCard card={kingOfSpades} state="playable" size="md" onClick={onClick} />,
    );

    await user.click(screen.getByTestId("playing-card-KS"));
    expect(onClick).toHaveBeenCalledTimes(1);

    onClick.mockClear();
    rerender(<PlayingCard card={kingOfSpades} state="unplayable" size="md" onClick={onClick} />);

    await user.click(screen.getByTestId("playing-card-KS"));
    expect(onClick).not.toHaveBeenCalled();
  });

  it("has correct aria-label for face-up card", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    expect(screen.getByTestId("playing-card-KS")).toHaveAttribute("aria-label", "King of Spades");
  });

  it("has correct aria-label for face-down card", () => {
    render(<PlayingCard card={null} state="face-down" size="md" />);

    expect(screen.getByTestId("playing-card-facedown")).toHaveAttribute(
      "aria-label",
      "face-down card",
    );
  });

  it("fires onClick on Enter key when playable", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(<PlayingCard card={kingOfSpades} state="playable" size="md" onClick={onClick} />);

    const card = screen.getByTestId("playing-card-KS");
    card.focus();
    await user.keyboard("{Enter}");

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("fires onClick on Space key when playable", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(<PlayingCard card={kingOfSpades} state="playable" size="md" onClick={onClick} />);

    const card = screen.getByTestId("playing-card-KS");
    card.focus();
    await user.keyboard(" ");

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("has tabIndex={0} when playable, tabIndex={-1} when not", () => {
    const { rerender } = render(<PlayingCard card={kingOfSpades} state="playable" size="md" />);

    expect(screen.getByTestId("playing-card-KS")).toHaveAttribute("tabindex", "0");

    rerender(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    expect(screen.getByTestId("playing-card-KS")).toHaveAttribute("tabindex", "-1");
  });

  it("sets aria-disabled when unplayable", () => {
    render(<PlayingCard card={kingOfSpades} state="unplayable" size="md" />);

    expect(screen.getByTestId("playing-card-KS")).toHaveAttribute("aria-disabled", "true");
  });
});
