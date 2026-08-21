import "@/shared/i18n/i18n";

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import type { Card } from "@/shared/types/matchTypes";

import { makeUser } from "../../../test-utils";
import { PlayingCard } from "./PlayingCard";

const kingOfSpades: Card = { rank: "K", suit: "S" };
const tenOfHearts: Card = { rank: "T", suit: "H" };
const queenOfDiamonds: Card = { rank: "Q", suit: "D" };

/** The face artwork layer inside a card wrapper (see `public/cards/{deck}/`). */
function faceImage(card: HTMLElement): HTMLImageElement | null {
  return card.querySelector("img");
}

/** Put a signed-in player with the given deck in the store (the one seam). */
function signInWithDeck(deck: "french" | "croatian") {
  useAuthStore.setState({ user: makeUser({ cardDeckPreference: deck }), isLoading: false });
}

describe("PlayingCard", () => {
  beforeEach(() => {
    // Default: no session at all, which is the state most of these assertions
    // care nothing about — and which must still resolve to the French deck.
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  afterEach(async () => {
    // Unmount FIRST. RTL's own auto-cleanup is registered at the file's root
    // scope, so it runs AFTER this inner hook — which would leave a mounted card
    // subscribed to the store and the i18n instance while both are reset here,
    // and every reset would log an act() warning.
    cleanup();
    useAuthStore.setState({ token: null, user: null, isLoading: false });
    await i18n.changeLanguage("en");
  });

  it("renders face-up card with the deck artwork for its card ID", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const card = screen.getByTestId("playing-card-KS");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/french/KS.svg");
  });

  it("uses the rank character in the asset name, not the '10' display label", () => {
    render(<PlayingCard card={tenOfHearts} state="default" size="md" />);

    const card = screen.getByTestId("playing-card-TH");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/french/TH.svg");
  });

  it("renders court cards from their own artwork file", () => {
    render(<PlayingCard card={queenOfDiamonds} state="default" size="lg" />);

    const card = screen.getByTestId("playing-card-QD");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/french/QD.svg");
  });

  it("falls back to the French deck when there is no signed-in player", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    expect(faceImage(screen.getByTestId("playing-card-KS"))).toHaveAttribute(
      "src",
      "/cards/french/KS.svg",
    );
  });

  it("draws the Croatian face when that is the player's deck", () => {
    signInWithDeck("croatian");
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const card = screen.getByTestId("playing-card-KS");
    expect(faceImage(card)).toHaveAttribute("src", "/cards/croatian/KS.webp");
  });

  it("keeps the same test id and geometry across decks — only the URL changes", () => {
    const { rerender } = render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    const before = screen.getByTestId("playing-card-KS");
    const geometry = [before.style.width, before.style.height, before.style.borderRadius];

    act(() => signInWithDeck("croatian"));
    rerender(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const after = screen.getByTestId("playing-card-KS");
    expect(faceImage(after)).toHaveAttribute("src", "/cards/croatian/KS.webp");
    expect([after.style.width, after.style.height, after.style.borderRadius]).toEqual(geometry);
  });

  it("re-skins in place when the deck changes mid-render (no remount, no reload)", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    expect(faceImage(screen.getByTestId("playing-card-KS"))).toHaveAttribute(
      "src",
      "/cards/french/KS.svg",
    );

    // The store write alone must be enough — this is what makes the in-match
    // Settings toggle change the table without interrupting play. No rerender()
    // here on purpose: the subscription is what has to do the work. `act` only
    // flushes React's queue; it is not a second render pass.
    act(() => signInWithDeck("croatian"));

    expect(faceImage(screen.getByTestId("playing-card-KS"))).toHaveAttribute(
      "src",
      "/cards/croatian/KS.webp",
    );
  });

  it("keeps the face artwork decorative so the wrapper label is the only announcement", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const image = faceImage(screen.getByTestId("playing-card-KS"));
    expect(image).toHaveAttribute("alt", "");
    expect(image).toHaveAttribute("aria-hidden", "true");
  });

  it.each([
    ["french", undefined],
    ["croatian", "croatian" as const],
  ])("degrades to a blank card when the %s face cannot load", (_deck, signIn) => {
    // A face that fails to resolve does NOT 404 in production — Caddy's SPA
    // fallback answers with index.html at 200, which browsers paint as a
    // broken-image glyph. Hiding the img is the only thing between a bad deck
    // path and a torn-looking hand, and the wrapper's parchment is what the
    // player sees instead. Asserted per deck because the two carry different
    // extensions, so a path bug can exist in one and not the other.
    if (signIn) signInWithDeck(signIn);
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    const image = faceImage(screen.getByTestId("playing-card-KS"))!;
    expect(image.style.visibility).toBe("");

    fireEvent.error(image);

    expect(image.style.visibility).toBe("hidden");
    // The card itself survives — blank, not absent.
    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
  });

  it("recovers a hidden face when the deck changes to one that loads", () => {
    // The regression: `onError` set style.visibility imperatively and React
    // reuses the same <img> when only `src` changes, so one transient miss on the
    // French face blanked the card for the page's lifetime — and survived
    // switching to the Croatian deck, whose asset was perfectly fine. The img is
    // keyed by its resolved URL now, so a new source gets a new node.
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    const before = faceImage(screen.getByTestId("playing-card-KS"))!;
    fireEvent.error(before);
    expect(before.style.visibility).toBe("hidden");

    act(() => signInWithDeck("croatian"));

    const after = faceImage(screen.getByTestId("playing-card-KS"))!;
    expect(after).toHaveAttribute("src", "/cards/croatian/KS.webp");
    expect(after.style.visibility).toBe("");
  });

  it("recovers the same node if a retried load succeeds", () => {
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    const image = faceImage(screen.getByTestId("playing-card-KS"))!;

    fireEvent.error(image);
    expect(image.style.visibility).toBe("hidden");

    // The onLoad twin of onError — the only thing that un-hides a node whose src
    // never changed.
    fireEvent.load(image);
    expect(image.style.visibility).toBe("");
  });

  it("renders face-down card with no artwork and no suit/rank visible", () => {
    render(<PlayingCard card={null} state="face-down" size="md" />);

    const card = screen.getByTestId("playing-card-facedown");
    expect(faceImage(card)).toBeNull();
    expect(card).not.toHaveTextContent("K");
    expect(card).not.toHaveTextContent("♠");
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

  it("names the suit as the ACTIVE DECK depicts it", () => {
    signInWithDeck("croatian");
    render(<PlayingCard card={queenOfDiamonds} state="default" size="md" />);

    // Bells on the Croatian deck, not diamonds — the same card ID, a different
    // announcement, because the artwork the player is looking at is different.
    expect(screen.getByTestId("playing-card-QD")).toHaveAttribute("aria-label", "Queen of Bells");
  });

  it("announces the card in the player's language", async () => {
    await i18n.changeLanguage("hr");
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    // hr puts the suit first ("Pik kralj"), which is why the label is a locale
    // template rather than a hardcoded "{rank} of {suit}" in the component.
    expect(screen.getByTestId("playing-card-KS")).toHaveAttribute("aria-label", "Pik kralj");
  });

  it("names the Croatian suit in the player's language", async () => {
    signInWithDeck("croatian");
    await i18n.changeLanguage("hr");
    render(<PlayingCard card={queenOfDiamonds} state="default" size="md" />);

    expect(screen.getByTestId("playing-card-QD")).toHaveAttribute("aria-label", "Bundeva dama");
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
