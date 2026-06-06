import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) => {
      const translations: Record<string, string> = {
        "match.scoreReveal.title": "Hand Score",
        "match.scoreReveal.cardPoints": "Card Points",
        "match.scoreReveal.declarationPoints": "Declarations",
        "match.scoreReveal.lastTrickBonus": "Last Trick Bonus",
        "match.scoreReveal.capotBonus": "Capot Bonus",
        "match.scoreReveal.failedContract": "Failed Contract",
        "match.scoreReveal.handTotal": "Hand Total",
        "match.scoreReveal.matchTotal": "Match Score",
        "match.scoreReveal.continue": "Continue",
        "team.us": "Us",
        "team.them": "Them",
        "match.scoreReveal.subtitleFailedUs": "Went down · all points to us",
        "match.scoreReveal.subtitleFailedThem": "Went down · all points to them",
      };
      if (key === "match.scoreReveal.subtitleHeldYour" && opts) {
        return `Contract held · your team called ${opts.suit}`;
      }
      if (key === "match.scoreReveal.subtitleHeldTheir" && opts) {
        return `Contract held · their team called ${opts.suit}`;
      }
      return translations[key] ?? key;
    },
  }),
}));

import type { HandScoredPayload } from "@/shared/types/wsEvents";

import { ScoreReveal } from "./ScoreReveal";

const normalData: HandScoredPayload = {
  teamACardPoints: 70,
  teamBCardPoints: 82,
  teamADeclPoints: 0,
  teamBDeclPoints: 0,
  lastTrickTeam: 1,
  lastTrickBonus: 10,
  capot: false,
  capotTeam: null,
  capotBonus: 0,
  failedContract: false,
  contractingTeam: 1,
  teamAHandTotal: 70,
  teamBHandTotal: 92,
  teamAMatchScore: 70,
  teamBMatchScore: 92,
};

describe("ScoreReveal", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Mock matchMedia for reduced motion
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders score breakdown", () => {
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={vi.fn()} />);

    expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
    expect(screen.getByTestId("score-reveal-title")).toHaveTextContent("Hand Score");
    expect(screen.getByTestId("row-card-points")).toBeInTheDocument();
    expect(screen.getByTestId("row-hand-total")).toBeInTheDocument();
    expect(screen.getByTestId("row-match-total")).toBeInTheDocument();
  });

  it("attaches data-team attributes on each numeric column", () => {
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={vi.fn()} />);

    const handTotalRow = screen.getByTestId("row-hand-total");
    const teamAValue = handTotalRow.querySelector("[data-team='teamA']");
    const teamBValue = handTotalRow.querySelector("[data-team='teamB']");
    expect(teamAValue).not.toBeNull();
    expect(teamBValue).not.toBeNull();
  });

  it("shows last trick bonus when applicable", () => {
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={vi.fn()} />);

    expect(screen.getByTestId("row-last-trick")).toBeInTheDocument();
    expect(screen.getByTestId("row-last-trick")).toHaveTextContent("+10");
  });

  it("hides declaration row when points are zero", () => {
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={vi.fn()} />);

    expect(screen.queryByTestId("row-decl-points")).not.toBeInTheDocument();
  });

  it("shows declaration row when points are present", () => {
    const withDecls = { ...normalData, teamADeclPoints: 50, teamBDeclPoints: 20 };
    render(<ScoreReveal data={withDecls} viewerTeam="teamA" onContinue={vi.fn()} />);

    expect(screen.getByTestId("row-decl-points")).toBeInTheDocument();
  });

  it("shows capot bonus when capot", () => {
    const capotData = {
      ...normalData,
      capot: true,
      capotTeam: 0,
      capotBonus: 100,
      lastTrickBonus: 0,
    };
    render(<ScoreReveal data={capotData} viewerTeam="teamA" onContinue={vi.fn()} />);

    expect(screen.getByTestId("row-capot-bonus")).toBeInTheDocument();
    expect(screen.getByTestId("row-capot-bonus")).toHaveTextContent("+100");
  });

  it("shows the failed-contract subtitle — points go to the viewer when contracting team is Them", () => {
    const failedData = { ...normalData, failedContract: true, contractingTeam: 1 };
    // contracting team = teamB; viewer on teamA → beneficiary (teamA) is "Us"
    const { container } = render(
      <ScoreReveal data={failedData} viewerTeam="teamA" onContinue={vi.fn()} />,
    );
    expect(container.textContent).toContain("Went down · all points to us");
    // The legacy middle paragraph is gone — the message lives in the subtitle.
    expect(screen.queryByTestId("row-failed-contract")).not.toBeInTheDocument();
  });

  it("shows the failed-contract subtitle — points go to opponents when viewer's team failed", () => {
    const failedData = { ...normalData, failedContract: true, contractingTeam: 1 };
    // contracting team = teamB (viewer); beneficiary (teamA) is "Them"
    const { container } = render(
      <ScoreReveal data={failedData} viewerTeam="teamB" onContinue={vi.fn()} />,
    );
    expect(container.textContent).toContain("Went down · all points to them");
  });

  it("shows the contract-held subtitle when the viewer's team called trump", () => {
    const { container } = render(
      <ScoreReveal
        data={normalData}
        viewerTeam="teamA"
        onContinue={vi.fn()}
        trumpSuit="H"
        trumpCallerSeat={0}
      />,
    );
    expect(container.textContent).toContain("Contract held · your team called");
  });

  it("Continue button is disabled initially and enabled after 2 seconds", () => {
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={vi.fn()} />);

    const btn = screen.getByTestId("score-reveal-continue");
    expect(btn).toBeDisabled();

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(btn).toBeEnabled();
  });

  it("calls onContinue when Continue button is clicked", async () => {
    const onContinue = vi.fn();
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={onContinue} />);

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    vi.useRealTimers();

    await userEvent.click(screen.getByTestId("score-reveal-continue"));
    expect(onContinue).toHaveBeenCalledOnce();
  });

  it("does NOT auto-dismiss — stays open until the player clicks Continue", () => {
    const onContinue = vi.fn();
    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={onContinue} />);

    // Walk well past the legacy 8s window — onContinue must stay un-fired
    // because dismissal is now manual-only (mirrors MatchResult / SurrenderPrompt).
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(onContinue).not.toHaveBeenCalled();
    expect(screen.getByTestId("score-reveal")).toBeInTheDocument();
  });

  it("enables Continue button after 500ms when reduced motion is preferred", () => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === "(prefers-reduced-motion: reduce)",
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    render(<ScoreReveal data={normalData} viewerTeam="teamA" onContinue={vi.fn()} />);

    const btn = screen.getByTestId("score-reveal-continue");
    expect(btn).toBeDisabled();

    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(btn).toBeEnabled();
  });
});
