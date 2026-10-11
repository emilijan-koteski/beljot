import "@/shared/i18n/i18n";

import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Card } from "@/shared/types/matchTypes";
import { makeUser } from "@/test-utils";

const kingOfSpades: Card = { rank: "K", suit: "S" };

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
 * Imports the component and the auth store from the same fresh module graph after `vi.resetModules()`.
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
