import "@/shared/i18n/i18n";

import { act, render, screen } from "@testing-library/react";
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
  it("renders label and card for Queen-first (Belote)", () => {
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
    expect(screen.getByTestId("belot-reveal-label")).toHaveTextContent("Belote");
    expect(screen.getByTestId("playing-card-QC")).toBeInTheDocument();
  });

  it("renders Rebelote label and King card for King-first", () => {
    render(
      <BelotReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="KC"
        isKing={true}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("belot-reveal-label")).toHaveTextContent("Rebelote");
    expect(screen.getByTestId("playing-card-KC")).toBeInTheDocument();
  });

  it("credits the viewer's team when a partner announces", () => {
    render(
      <BelotReveal
        playerSeat={2}
        myPlayerSeat={0}
        cardId="QH"
        isKing={false}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("belot-reveal")).toHaveTextContent("+20 to your team's score");
  });

  it("credits the other team when an opponent announces", () => {
    render(<BelotReveal playerSeat={1} myPlayerSeat={0} cardId="KH" isKing onComplete={vi.fn()} />);
    expect(screen.getByTestId("belot-reveal")).toHaveTextContent("+20 to their team's score");
    expect(screen.getByTestId("belot-reveal")).not.toHaveTextContent("your team");
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
            avatarUrl: null,
            team: "teamA",
            hand: [],
            declarations: [],
            connected: true,
            isBot: false,
            level: 1,
            faceDownCount: 0,
            handCount: 0,
            declarationAnswered: false,
          },
          {
            seat: 1,
            userId: 1,
            username: "Bob",
            avatarUrl: null,
            team: "teamB",
            hand: [],
            declarations: [],
            connected: true,
            isBot: false,
            level: 1,
            faceDownCount: 0,
            handCount: 0,
            declarationAnswered: false,
          },
          {
            seat: 2,
            userId: 2,
            username: "Stefan",
            avatarUrl: null,
            team: "teamA",
            hand: [],
            declarations: [],
            connected: true,
            isBot: false,
            level: 1,
            faceDownCount: 0,
            handCount: 0,
            declarationAnswered: false,
          },
          {
            seat: 3,
            userId: 3,
            username: "Mirela",
            avatarUrl: null,
            team: "teamB",
            hand: [],
            declarations: [],
            connected: true,
            isBot: false,
            level: 1,
            faceDownCount: 0,
            handCount: 0,
            declarationAnswered: false,
          },
        ]}
        onComplete={vi.fn()}
      />,
    );
    expect(screen.getByTestId("belot-reveal-title")).toHaveTextContent("Stefan declared Rebelote.");
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
    act(() => {
      vi.advanceTimersByTime(7900);
    });
    expect(onComplete).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(200);
    });
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
    act(() => {
      vi.advanceTimersByTime(1600);
    });
    expect(onComplete).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });
});
