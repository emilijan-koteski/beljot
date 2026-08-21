import "@/shared/i18n/i18n";

import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Card } from "@/shared/types/matchTypes";
import { makeUser } from "@/test-utils";

/**
 * `warmDeck`'s latch, which the spec's Code Map called out as the subtlest bug in
 * the change: it was a single module-level boolean, so the FIRST deck mounted won
 * for the page's lifetime and a player who switched decks got no warm-up at all
 * for the deck they were actually looking at.
 *
 * The latch is module state, so every case re-imports the module fresh — a single
 * shared import would let the first test's warm-up satisfy the second's latch and
 * both would pass regardless of the fix.
 */
const kingOfSpades: Card = { rank: "K", suit: "S" };

/** Srcs assigned to every `new Image()` since the last reset. */
let warmed: string[] = [];

class RecordingImage {
  #src = "";
  set src(value: string) {
    this.#src = value;
    warmed.push(value);
  }
  get src() {
    return this.#src;
  }
}

/**
 * The component AND the store from the SAME fresh module graph.
 *
 * `vi.resetModules()` re-instantiates every module the dynamic import pulls in,
 * the auth store included — so a statically imported `useAuthStore` would be a
 * different store object than the one the component subscribes to, and every
 * deck switch here would be invisible to it. (Found the hard way: both renders
 * drew the French face while the test believed it had switched decks.)
 */
async function freshGraph() {
  vi.resetModules();
  const [{ PlayingCard }, { useAuthStore }] = await Promise.all([
    import("./PlayingCard"),
    import("@/shared/stores/authStore"),
  ]);
  return { PlayingCard, useAuthStore };
}

describe("warmDeck", () => {
  const realImage = globalThis.Image;

  beforeEach(() => {
    warmed = [];
    // jsdom defines Image but never fetches, so this substitution is what makes
    // the warm-up observable at all.
    globalThis.Image = RecordingImage as unknown as typeof Image;
  });

  afterEach(() => {
    globalThis.Image = realImage;
  });

  it("warms all 32 faces of the active deck on first mount", async () => {
    const { PlayingCard } = await freshGraph();
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    expect(warmed).toHaveLength(32);
    expect(new Set(warmed).size).toBe(32);
    expect(warmed).toContain("/cards/french/KS.svg");
    expect(warmed).toContain("/cards/french/7C.svg");
    // Every entry belongs to the active deck — no cross-deck bleed.
    expect(warmed.every((src) => src.startsWith("/cards/french/"))).toBe(true);
  });

  it("does not re-warm the same deck on later mounts", async () => {
    const { PlayingCard } = await freshGraph();
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    expect(warmed).toHaveLength(32);

    warmed = [];
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    expect(warmed).toHaveLength(0);
  });

  it("warms the SECOND deck when the player switches — the latch is per deck", async () => {
    const { PlayingCard, useAuthStore } = await freshGraph();
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);
    expect(warmed.every((src) => src.startsWith("/cards/french/"))).toBe(true);

    // This is the regression: with one boolean latch, nothing below happened and
    // the Croatian deck was fetched one face at a time as cards appeared.
    warmed = [];
    useAuthStore.setState({
      user: makeUser({ cardDeckPreference: "croatian" }),
      isLoading: false,
    });
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    expect(warmed).toHaveLength(32);
    expect(warmed.every((src) => src.startsWith("/cards/croatian/"))).toBe(true);
    expect(warmed).toContain("/cards/croatian/KS.webp");
  });

  it("does not re-warm a deck it already warmed once, after switching back", async () => {
    const { PlayingCard, useAuthStore } = await freshGraph();
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    useAuthStore.setState({
      user: makeUser({ cardDeckPreference: "croatian" }),
      isLoading: false,
    });
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    warmed = [];
    useAuthStore.setState({
      user: makeUser({ cardDeckPreference: "french" }),
      isLoading: false,
    });
    render(<PlayingCard card={kingOfSpades} state="default" size="md" />);

    expect(warmed).toHaveLength(0);
  });
});
