import "@/shared/i18n/i18n";

import { fireEvent, render, screen } from "@testing-library/react";
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
    expect(screen.getByTestId("trump-reveal-seal").getAttribute("data-suit")).toBe("S");
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
