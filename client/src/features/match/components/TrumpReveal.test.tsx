import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import type { PlayerState } from "@/shared/types/matchTypes";
import { makeUser } from "@/test-utils";

import { TrumpReveal } from "./TrumpReveal";

function makePlayers(): PlayerState[] {
  return [
    {
      hand: [],
      seat: 0,
      userId: 10,
      username: "Alice",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
    },
    {
      hand: [],
      seat: 1,
      userId: 20,
      username: "Bob",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
    },
    {
      hand: [],
      seat: 2,
      userId: 30,
      username: "Carol",
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
    },
    {
      hand: [],
      seat: 3,
      userId: 40,
      username: "Dave",
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
    },
  ];
}

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

describe("TrumpReveal — Wax Seal", () => {
  it("round 1 (took candidate): hero card, taker, eyebrow, '{suit} is trump this hand', seal, no candidate subline", () => {
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="7S"
        trumpSuit="S"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("trump-reveal")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7S")).toBeInTheDocument();
    expect(screen.getByTestId("trump-reveal-taker")).toHaveTextContent("Carol");
    expect(screen.getByTestId("trump-reveal-eyebrow")).toHaveTextContent("Trump taken");
    expect(screen.getByTestId("trump-reveal-eyebrow").textContent).not.toContain("free pick");
    expect(screen.getByTestId("trump-reveal-copy")).toHaveTextContent("Spades is trump this hand");
    const seal = screen.getByTestId("trump-reveal-seal");
    expect(seal.getAttribute("data-suit")).toBe("S");
    // With a hero card the seal stays pinned to its corner and takes no halo of
    // its own — the halo lives behind the card.
    expect(seal.className).toContain("absolute");
    expect(seal.style.boxShadow).not.toContain("0 0 32px");
    expect(screen.queryByTestId("trump-reveal-candidate")).toBeNull();
  });

  it("round 2 (free pick): STILL renders the candidate card, seal shows the chosen suit, copy 'chose {suit}' + candidate subline", () => {
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="9S"
        trumpSuit="D"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );
    // The passed candidate card is the hero in both rounds now.
    expect(screen.getByTestId("playing-card-9S")).toBeInTheDocument();
    // Seal carries the CHOSEN suit (Diamonds), not the candidate's (Spades).
    expect(screen.getByTestId("trump-reveal-seal").getAttribute("data-suit")).toBe("D");
    expect(screen.getByTestId("trump-reveal-eyebrow")).toHaveTextContent("free pick");
    const copy = screen.getByTestId("trump-reveal-copy");
    expect(copy.textContent).toContain("Diamonds");
    expect(copy.textContent).not.toContain("is trump this hand");
    const candidate = screen.getByTestId("trump-reveal-candidate");
    expect(candidate.textContent).toContain("Nine");
    expect(candidate.textContent).toContain("Spades");
  });

  it("candidate subline uses full English words — never glyphs or bare rank codes (T-rank)", () => {
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="TC"
        trumpSuit="H"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );
    const text = screen.getByTestId("trump-reveal-candidate").textContent ?? "";
    expect(text).toContain("Ten");
    expect(text).toContain("Clubs");
    expect(text).not.toContain("TC");
    for (const glyph of ["♠", "♥", "♦", "♣"]) {
      expect(text).not.toContain(glyph);
    }
    expect(/\b[TJQKA]\b/.test(text)).toBe(false);
    expect(screen.getByTestId("trump-reveal-seal").getAttribute("data-suit")).toBe("H");
  });

  it("falls back gracefully when the seat has no matching player (no leaked name, card still shown)", () => {
    render(
      <TrumpReveal
        playerSeat={5}
        myPlayerSeat={0}
        cardId="9D"
        trumpSuit="D"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("playing-card-9D")).toBeInTheDocument();
    const panel = screen.getByTestId("trump-reveal");
    expect(panel.textContent).not.toContain("Alice");
    expect(panel.textContent).not.toContain("Bob");
  });

  it("auto-dismisses after 8 seconds", () => {
    vi.useFakeTimers();
    const onComplete = vi.fn();
    render(
      <TrumpReveal
        playerSeat={0}
        myPlayerSeat={0}
        cardId="7S"
        trumpSuit="S"
        players={makePlayers()}
        onComplete={onComplete}
      />,
    );
    vi.advanceTimersByTime(7000);
    expect(onComplete).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1500);
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });

  it("auto-dismisses faster with prefers-reduced-motion (~1.5 s)", () => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });
    vi.useFakeTimers();
    const onComplete = vi.fn();
    render(
      <TrumpReveal
        playerSeat={0}
        myPlayerSeat={0}
        cardId="7S"
        trumpSuit="S"
        players={makePlayers()}
        onComplete={onComplete}
      />,
    );
    vi.advanceTimersByTime(2000);
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });

  it("can be dismissed early by clicking the X", () => {
    const onComplete = vi.fn();
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="7S"
        trumpSuit="S"
        players={makePlayers()}
        onComplete={onComplete}
      />,
    );
    fireEvent.click(screen.getByTestId("trump-reveal-close"));
    expect(onComplete).toHaveBeenCalledOnce();
  });

  // --- Candidate-less take (a variant that names trump freely and gives the
  // taker no card). The server sends an empty cardId rather than suppressing
  // the event, so the reveal must render without a hero card.
  describe("no trump candidate (empty cardId)", () => {
    it("renders the panel, the seal, and 'named {suit} as trump' with no hero card", () => {
      render(
        <TrumpReveal
          playerSeat={2}
          myPlayerSeat={0}
          cardId=""
          trumpSuit="C"
          players={makePlayers()}
          onComplete={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-reveal")).toBeInTheDocument();
      expect(screen.getByTestId("trump-reveal-taker")).toHaveTextContent("Carol");
      expect(screen.getByTestId("trump-reveal-seal").getAttribute("data-suit")).toBe("C");
      expect(screen.getByTestId("trump-reveal-copy")).toHaveTextContent("named Clubs as trump");
      // No candidate existed, so neither the hero card nor the "was on the
      // table" subline may appear.
      expect(screen.queryByTestId(/^playing-card-/)).not.toBeInTheDocument();
      expect(screen.queryByTestId("trump-reveal-candidate")).toBeNull();
    });

    it("uses the plain eyebrow, not the 'free pick' contrast (no suit was turned down)", () => {
      render(
        <TrumpReveal
          playerSeat={1}
          myPlayerSeat={0}
          cardId=""
          trumpSuit="S"
          players={makePlayers()}
          onComplete={vi.fn()}
        />,
      );
      const eyebrow = screen.getByTestId("trump-reveal-eyebrow");
      expect(eyebrow).toHaveTextContent("Trump taken");
      expect(eyebrow.textContent).not.toContain("free pick");
    });

    it("still auto-dismisses and still resolves the viewer-relative team glow", () => {
      vi.useFakeTimers();
      const onComplete = vi.fn();
      render(
        <TrumpReveal
          playerSeat={1}
          myPlayerSeat={0}
          cardId=""
          trumpSuit="D"
          players={makePlayers()}
          onComplete={onComplete}
        />,
      );
      const panel = screen.getByTestId("trump-reveal").querySelector("[data-team]");
      expect(panel?.getAttribute("data-team")).toBe("silver");
      act(() => {
        vi.advanceTimersByTime(8500);
      });
      expect(onComplete).toHaveBeenCalledOnce();
      vi.useRealTimers();
    });

    it("gives the seal its own team-coloured halo when it is the hero", () => {
      // With a hero card the halo sits behind the card; with none the seal
      // carries it, or it reads as an unglowing stamp inside a glowing panel.
      render(
        <TrumpReveal
          playerSeat={2}
          myPlayerSeat={0}
          cardId=""
          trumpSuit="S"
          players={makePlayers()}
          onComplete={vi.fn()}
        />,
      );
      const seal = screen.getByTestId("trump-reveal-seal");
      expect(seal.style.boxShadow).toContain("0 0 32px");
      // The seal also leaves the card's absolute corner anchor behind.
      expect(seal.className).not.toContain("absolute");
    });

    // Empty is the only non-2-character shape that renders. Everything else is
    // malformed and must be dropped rather than sliced into a plausible card —
    // "10S" is the dangerous one a bare length check lets through, where
    // parseCardId reads rank "1" and suit "0".
    it.each(["J", "10S", "JSX", "js", "XS", "JX"])(
      "returns null for the malformed cardId %s",
      (cardId) => {
        render(
          <TrumpReveal
            playerSeat={2}
            myPlayerSeat={0}
            cardId={cardId}
            trumpSuit="S"
            players={makePlayers()}
            onComplete={vi.fn()}
          />,
        );
        expect(screen.queryByTestId("trump-reveal")).toBeNull();
      },
    );
  });

  it("glows gold (Us) when the caller is on the viewer's team", () => {
    // caller seat 2, viewer seat 0 — same parity → gold
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="7S"
        trumpSuit="S"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );
    const panel = screen.getByTestId("trump-reveal").querySelector("[data-team]");
    expect(panel?.getAttribute("data-team")).toBe("gold");
  });

  it("glows silver (Them) when the caller is on the opposing team", () => {
    // caller seat 1, viewer seat 0 — opposite parity → silver
    render(
      <TrumpReveal
        playerSeat={1}
        myPlayerSeat={0}
        cardId="7S"
        trumpSuit="S"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );
    const panel = screen.getByTestId("trump-reveal").querySelector("[data-team]");
    expect(panel?.getAttribute("data-team")).toBe("silver");
  });
});

// --- Scope Amendment 1: the wax seal follows the active deck ---
//
// The seal is the largest suit mark in the app (30 px), which is what set the
// icon resolution, so it is also where a wrong accent is most visible.
describe("TrumpReveal card deck", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  function signIn(deck: "french" | "croatian") {
    useAuthStore.setState({ user: makeUser({ cardDeckPreference: deck }), isLoading: false });
  }

  it("draws the Croatian icon and rings the seal in that suit's accent", () => {
    signIn("croatian");
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId=""
        trumpSuit="S"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );

    const mark = screen.getByTestId("suit-mark-S");
    expect(mark.tagName).toBe("IMG");
    expect(mark).toHaveAttribute("src", "/suits/croatian/S.webp");

    // Leaves are green, so the wax ring must not be the French near-black.
    // rgb() because jsdom re-serialises hex in `border`.
    const seal = screen.getByTestId("trump-reveal-seal");
    expect(seal.style.borderColor).toBe("rgb(74, 122, 58)");
    expect(seal.style.borderColor).not.toBe("rgb(26, 26, 26)");
    // The testid and data-suit contract is unchanged by the deck.
    expect(seal).toHaveAttribute("data-suit", "S");
  });

  it("names the trump suit in the active deck's vocabulary", () => {
    signIn("croatian");
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId=""
        trumpSuit="D"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );

    // The seal's aria-label IS the suit name, and the body copy repeats it.
    expect(screen.getByTestId("trump-reveal-seal")).toHaveAttribute("aria-label", "Bells");
    expect(screen.getByTestId("trump-reveal").textContent).not.toContain("Diamonds");
  });

  // Item 13: SUIT_INK_ON_FELT was pinned as a table but nothing asserted that
  // TrumpReveal actually READS it — repointing the body copy at SUIT_ACCENT
  // would have wrecked legibility on both decks with no test failing.
  it("inks the suit word with the on-felt palette, not the parchment accent", () => {
    signIn("croatian");
    const { container } = render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId=""
        trumpSuit="D"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );

    // SUIT_INK_ON_FELT.croatian.D = #e8c766 (lifted), NOT SUIT_ACCENT's #c9a23c:
    // the parchment gold is unreadable on dark felt.
    const inked = Array.from(container.querySelectorAll("b")).map(
      (b) => (b as HTMLElement).style.color,
    );
    expect(inked).toContain("rgb(232, 199, 102)");
    expect(inked).not.toContain("rgb(201, 162, 60)");
  });

  it("inks the French suit word with the pre-existing lifted red", () => {
    signIn("french");
    const { container } = render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId=""
        trumpSuit="H"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );

    // #ff8585 — `--suit-red-up`, exactly what shipped before this story.
    const inked = Array.from(container.querySelectorAll("b")).map(
      (b) => (b as HTMLElement).style.color,
    );
    expect(inked).toContain("rgb(255, 133, 133)");
  });

  it("keeps the French seal exactly as it was", () => {
    signIn("french");
    render(
      <TrumpReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId=""
        trumpSuit="H"
        players={makePlayers()}
        onComplete={vi.fn()}
      />,
    );

    expect(screen.getByTestId("suit-mark-H")).toHaveTextContent("♥");
    expect(screen.getByTestId("trump-reveal-seal").style.borderColor).toBe("rgb(198, 40, 40)");
  });
});
