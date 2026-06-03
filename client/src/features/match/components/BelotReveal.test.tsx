import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { BelotReveal } from "./BelotReveal";

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

describe("BelotReveal", () => {
  it("renders label and card for Queen-first (Belot)", () => {
    render(
      <BelotReveal
        playerSeat={0}
        myPlayerSeat={0}
        cardId="QC"
        isKing={false}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("belot-reveal")).toBeInTheDocument();
    expect(screen.getByTestId("belot-reveal-label")).toHaveTextContent("Belot");
    expect(screen.getByTestId("playing-card-QC")).toBeInTheDocument();
  });

  it("renders Re-belot label and King card for King-first", () => {
    render(
      <BelotReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="KC"
        isKing={true}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("belot-reveal-label")).toHaveTextContent("Re-belot");
    expect(screen.getByTestId("playing-card-KC")).toBeInTheDocument();
  });

  it("renders the announcer's name in the title when players is provided", () => {
    render(
      <BelotReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="KC"
        isKing={true}
        players={[
          {
            seat: 0,
            userId: 0,
            username: "Alice",
            team: "teamA",
            hand: [],
            declarations: [],
            connected: true,
          },
          {
            seat: 1,
            userId: 1,
            username: "Bob",
            team: "teamB",
            hand: [],
            declarations: [],
            connected: true,
          },
          {
            seat: 2,
            userId: 2,
            username: "Stefan",
            team: "teamA",
            hand: [],
            declarations: [],
            connected: true,
          },
          {
            seat: 3,
            userId: 3,
            username: "Mirela",
            team: "teamB",
            hand: [],
            declarations: [],
            connected: true,
          },
        ]}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("belot-reveal-title")).toHaveTextContent(
      "Stefan announced Re-belot.",
    );
  });

  it("falls back to the team label when players is not provided", () => {
    render(
      <BelotReveal
        playerSeat={0}
        myPlayerSeat={0}
        cardId="QC"
        isKing={false}
        onComplete={vi.fn()}
      />,
    );
    // Stand-alone test render — no players roster, viewer-relative team only.
    expect(screen.getByTestId("belot-reveal-title")).toHaveTextContent("Us");
  });

  it("auto-dismisses after 8 seconds", () => {
    vi.useFakeTimers();
    const onComplete = vi.fn();
    render(
      <BelotReveal
        playerSeat={0}
        myPlayerSeat={0}
        cardId="QC"
        isKing={false}
        onComplete={onComplete}
      />,
    );
    vi.advanceTimersByTime(7900);
    expect(onComplete).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });

  it("auto-dismisses faster with prefers-reduced-motion", () => {
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
      <BelotReveal
        playerSeat={0}
        myPlayerSeat={0}
        cardId="QC"
        isKing={false}
        onComplete={onComplete}
      />,
    );
    vi.advanceTimersByTime(1600);
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });
});
