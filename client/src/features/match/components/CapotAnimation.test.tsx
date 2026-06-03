import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      const translations: Record<string, string> = {
        "match.capot.title": "CAPOT!",
        "match.capot.eyebrowUs": "Every trick · we swept the hand",
        "match.capot.eyebrowThem": "Every trick · they swept the hand",
        "match.capot.bonusUs": "+{{points}} to us",
        "match.capot.bonusThem": "+{{points}} to them",
      };
      let out = translations[key] ?? key;
      if (opts) {
        for (const [k, v] of Object.entries(opts)) {
          out = out.replace(new RegExp(`{{${k}}}`, "g"), String(v));
        }
      }
      return out;
    },
  }),
}));

import { CapotAnimation } from "./CapotAnimation";

function mockMatchMedia(reducedMotion: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: reducedMotion && query === "(prefers-reduced-motion: reduce)",
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

describe("CapotAnimation", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockMatchMedia(false);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the capot title", () => {
    render(<CapotAnimation capotTeam={0} viewerSeat={0} capotBonus={100} onComplete={vi.fn()} />);

    expect(screen.getByTestId("capot-animation")).toBeInTheDocument();
    expect(screen.getByTestId("capot-text")).toHaveTextContent("CAPOT!");
  });

  it("reads gold / first-person when the viewer's own team swept", () => {
    // viewer on seat 0 (team A), capot by team 0 (team A) → viewer's team
    render(<CapotAnimation capotTeam={0} viewerSeat={0} capotBonus={100} onComplete={vi.fn()} />);

    expect(screen.getByTestId("capot-text")).toHaveAttribute("data-team", "gold");
    expect(screen.getByTestId("capot-eyebrow")).toHaveTextContent(
      "Every trick · we swept the hand",
    );
    expect(screen.getByTestId("capot-bonus")).toHaveTextContent("+100 to us");
  });

  it("reads silver / third-person when the opponents swept", () => {
    // viewer on seat 1 (team B), capot by team 0 (team A) → opponents
    render(<CapotAnimation capotTeam={0} viewerSeat={1} capotBonus={100} onComplete={vi.fn()} />);

    expect(screen.getByTestId("capot-text")).toHaveAttribute("data-team", "silver");
    expect(screen.getByTestId("capot-eyebrow")).toHaveTextContent(
      "Every trick · they swept the hand",
    );
    expect(screen.getByTestId("capot-bonus")).toHaveTextContent("+100 to them");
  });

  it("is viewer-relative regardless of the absolute capot team", () => {
    // viewer on seat 1 (team B), capot by team 1 (team B) → still the viewer's team → gold/first-person
    render(<CapotAnimation capotTeam={1} viewerSeat={1} capotBonus={100} onComplete={vi.fn()} />);

    expect(screen.getByTestId("capot-text")).toHaveAttribute("data-team", "gold");
    expect(screen.getByTestId("capot-bonus")).toHaveTextContent("+100 to us");
  });

  it("renders the actual capot bonus from props, not a hard-coded value", () => {
    render(<CapotAnimation capotTeam={0} viewerSeat={0} capotBonus={90} onComplete={vi.fn()} />);

    expect(screen.getByTestId("capot-bonus")).toHaveTextContent("+90 to us");
  });

  it("calls onComplete after the banner dwell (2500ms)", () => {
    const onComplete = vi.fn();
    render(
      <CapotAnimation capotTeam={0} viewerSeat={0} capotBonus={100} onComplete={onComplete} />,
    );

    expect(onComplete).not.toHaveBeenCalled();
    vi.advanceTimersByTime(2499);
    expect(onComplete).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onComplete).toHaveBeenCalledOnce();
  });

  it("calls onComplete after 500ms when reduced motion is preferred", () => {
    mockMatchMedia(true);
    const onComplete = vi.fn();
    render(
      <CapotAnimation capotTeam={0} viewerSeat={0} capotBonus={100} onComplete={onComplete} />,
    );

    vi.advanceTimersByTime(500);
    expect(onComplete).toHaveBeenCalledOnce();
  });

  it("skips early to onComplete on tap/click", () => {
    const onComplete = vi.fn();
    render(
      <CapotAnimation capotTeam={0} viewerSeat={0} capotBonus={100} onComplete={onComplete} />,
    );

    fireEvent.click(screen.getByTestId("capot-animation"));
    expect(onComplete).toHaveBeenCalledOnce();
  });
});
