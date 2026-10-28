import "@/shared/i18n/i18n";

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
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
      avatarUrl: null,
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
      declarationAnswered: false,
    },
    {
      hand: [],
      seat: 1,
      userId: 20,
      username: "Bob",
      avatarUrl: null,
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
      declarationAnswered: false,
    },
    {
      hand: [],
      seat: 2,
      userId: 30,
      username: "Carol",
      avatarUrl: null,
      team: "teamA",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
      declarationAnswered: false,
    },
    {
      hand: [],
      seat: 3,
      userId: 40,
      username: "Dave",
      avatarUrl: null,
      team: "teamB",
      declarations: [],
      connected: true,
      isBot: false,
      level: 1,
      faceDownCount: 0,
      handCount: 0,
      declarationAnswered: false,
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
  it("round 1: hero card, taker, eyebrow, '{suit} is trump' and the seal", () => {
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
    expect(screen.getByTestId("trump-reveal-copy").textContent).toBe("Spades is trump");
    const seal = screen.getByTestId("trump-reveal-seal");
    expect(seal.getAttribute("data-suit")).toBe("S");
    expect(seal.className).toContain("absolute");
    expect(seal.style.boxShadow).not.toContain("0 0 32px");
    expect(screen.queryByTestId("trump-reveal-candidate")).toBeNull();
  });

  it("round 2: still shows the revealed card, while the seal and the one line name the chosen suit", () => {
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
    expect(screen.getByTestId("playing-card-9S")).toBeInTheDocument();
    expect(screen.getByTestId("trump-reveal-seal").getAttribute("data-suit")).toBe("D");
    expect(screen.getByTestId("trump-reveal-eyebrow").textContent).toBe("Trump taken");
    expect(screen.getByTestId("trump-reveal-copy").textContent).toBe("Diamonds is trump");
    expect(screen.queryByTestId("trump-reveal-candidate")).toBeNull();
  });

  it("names the suit in full words, never a glyph", () => {
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
    const text = screen.getByTestId("trump-reveal-copy").textContent ?? "";
    expect(text).toBe("Hearts is trump");
    for (const glyph of ["♠", "♥", "♦", "♣"]) {
      expect(text).not.toContain(glyph);
    }
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
    act(() => {
      vi.advanceTimersByTime(7000);
    });
    expect(onComplete).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(1500);
    });
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
    act(() => {
      vi.advanceTimersByTime(2000);
    });
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

  describe("no trump candidate (empty cardId)", () => {
    it("renders the panel, the seal, and '{suit} is trump' with no hero card", () => {
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
      expect(screen.getByTestId("trump-reveal-copy").textContent).toBe("Clubs is trump");
      expect(screen.queryByTestId(/^playing-card-/)).not.toBeInTheDocument();
      expect(screen.queryByTestId("trump-reveal-candidate")).toBeNull();
    });

    it("uses the same eyebrow as the Bitola rounds", () => {
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
      expect(screen.getByTestId("trump-reveal-eyebrow").textContent).toBe("Trump taken");
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
      expect(seal.className).not.toContain("absolute");
    });

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

describe("TrumpReveal card deck", () => {
  afterEach(() => {
    cleanup();
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

    const seal = screen.getByTestId("trump-reveal-seal");
    expect(seal.style.borderColor).toBe("rgb(74, 122, 58)");
    expect(seal.style.borderColor).not.toBe("rgb(26, 26, 26)");
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

    expect(screen.getByTestId("trump-reveal-seal")).toHaveAttribute("aria-label", "Bells");
    expect(screen.getByTestId("trump-reveal").textContent).not.toContain("Diamonds");
  });

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
