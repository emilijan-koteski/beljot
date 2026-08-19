import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PlayerState } from "@/shared/types/matchTypes";

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

    it("still returns null for a malformed 1-character cardId", () => {
      render(
        <TrumpReveal
          playerSeat={2}
          myPlayerSeat={0}
          cardId="J"
          trumpSuit="S"
          players={makePlayers()}
          onComplete={vi.fn()}
        />,
      );
      expect(screen.queryByTestId("trump-reveal")).toBeNull();
    });
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
