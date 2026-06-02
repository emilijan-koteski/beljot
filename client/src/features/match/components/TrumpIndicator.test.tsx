import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) => {
      const translations: Record<string, string> = {
        "team.us": "Us",
        "team.them": "Them",
        "team.a": "Team A",
        "team.b": "Team B",
        "match.suits.spades": "Spades",
        "match.suits.hearts": "Hearts",
        "match.suits.diamonds": "Diamonds",
        "match.suits.clubs": "Clubs",
        "match.trumpIndicator.trump": "Trump",
      };
      if (key === "match.trumpIndicator.label" && opts) return `Trump: ${opts.suit}`;
      if (key === "match.trumpIndicator.labelWithTeam" && opts) {
        return `Trump: ${opts.suit} (${opts.team})`;
      }
      if (key === "match.trumpIndicator.labelWithCaller" && opts) {
        return `Trump: ${opts.suit} (${opts.team} — ${opts.name})`;
      }
      return translations[key] ?? key;
    },
  }),
}));

import { TrumpIndicator } from "./TrumpIndicator";

describe("TrumpIndicator", () => {
  it("renders spades suit symbol in primary text color", () => {
    render(<TrumpIndicator trumpSuit="S" />);
    const indicator = screen.getByTestId("trump-indicator");
    expect(indicator).toBeInTheDocument();
    expect(indicator.textContent).toContain("\u2660");
    expect(indicator.querySelector(".text-text-primary")?.textContent).toContain("\u2660");
  });

  it("renders hearts suit symbol in red", () => {
    render(<TrumpIndicator trumpSuit="H" />);
    const indicator = screen.getByTestId("trump-indicator");
    expect(indicator.textContent).toContain("\u2665");
    expect(indicator.querySelector(".text-red-500")?.textContent).toContain("\u2665");
  });

  it("renders diamonds suit symbol in red", () => {
    render(<TrumpIndicator trumpSuit="D" />);
    const indicator = screen.getByTestId("trump-indicator");
    expect(indicator.textContent).toContain("\u2666");
    expect(indicator.querySelector(".text-red-500")?.textContent).toContain("\u2666");
  });

  it("renders clubs suit symbol in primary text color", () => {
    render(<TrumpIndicator trumpSuit="C" />);
    const indicator = screen.getByTestId("trump-indicator");
    expect(indicator.textContent).toContain("\u2663");
    expect(indicator.querySelector(".text-text-primary")?.textContent).toContain("\u2663");
  });

  it("has aria-live polite for screen reader updates", () => {
    render(<TrumpIndicator trumpSuit="S" />);
    expect(screen.getByTestId("trump-indicator")).toHaveAttribute("aria-live", "polite");
  });

  it("has aria-label describing the trump suit", () => {
    render(<TrumpIndicator trumpSuit="H" />);
    expect(screen.getByTestId("trump-indicator")).toHaveAttribute("aria-label");
  });

  it("shows a teamA badge when the trump caller is on an even seat", () => {
    render(<TrumpIndicator trumpSuit="S" trumpCallerSeat={0} />);
    const badge = screen.getByTestId("trump-caller-team");
    expect(badge).toHaveAttribute("data-team", "teamA");
    expect(badge.className).toContain("text-team-a");
    const container = screen.getByTestId("trump-indicator");
    expect(container.className).toContain("border-team-a");
    expect(container).toHaveAttribute("data-team", "teamA");
  });

  it("shows a teamB badge when the trump caller is on an odd seat", () => {
    render(<TrumpIndicator trumpSuit="H" trumpCallerSeat={3} />);
    const badge = screen.getByTestId("trump-caller-team");
    expect(badge).toHaveAttribute("data-team", "teamB");
    expect(badge.className).toContain("text-team-b");
    const container = screen.getByTestId("trump-indicator");
    expect(container.className).toContain("border-team-b");
    expect(container).toHaveAttribute("data-team", "teamB");
  });

  it("omits the team badge when trump caller seat is null", () => {
    render(<TrumpIndicator trumpSuit="S" trumpCallerSeat={null} />);
    expect(screen.queryByTestId("trump-caller-team")).not.toBeInTheDocument();
    expect(screen.getByTestId("trump-indicator").className).not.toContain("border-team-");
  });

  it("renders the trump caller's name alongside the team badge", () => {
    render(<TrumpIndicator trumpSuit="S" trumpCallerSeat={0} trumpCallerName="Marko" />);
    const nameEl = screen.getByTestId("trump-caller-name");
    expect(nameEl).toBeInTheDocument();
    expect(nameEl.textContent).toBe("Marko");
    expect(screen.getByTestId("trump-indicator")).toHaveAttribute("aria-label");
  });

  it("omits the player name when trumpCallerName is empty or missing", () => {
    const { rerender } = render(<TrumpIndicator trumpSuit="S" trumpCallerSeat={0} />);
    expect(screen.queryByTestId("trump-caller-name")).not.toBeInTheDocument();

    rerender(<TrumpIndicator trumpSuit="S" trumpCallerSeat={0} trumpCallerName="" />);
    expect(screen.queryByTestId("trump-caller-name")).not.toBeInTheDocument();

    rerender(<TrumpIndicator trumpSuit="S" trumpCallerSeat={0} trumpCallerName={null} />);
    expect(screen.queryByTestId("trump-caller-name")).not.toBeInTheDocument();
  });

  it("omits the player name when no caller seat is set even if a name is provided", () => {
    render(<TrumpIndicator trumpSuit="S" trumpCallerSeat={null} trumpCallerName="Marko" />);
    expect(screen.queryByTestId("trump-caller-name")).not.toBeInTheDocument();
  });

  it("renders Us when viewerTeam matches the caller's team (AC4)", () => {
    render(<TrumpIndicator trumpSuit="S" trumpCallerSeat={0} viewerTeam="teamA" />);
    const badge = screen.getByTestId("trump-caller-team");
    expect(badge).toHaveTextContent("Us");
    expect(badge).toHaveAttribute("data-team", "teamA");
  });

  it("renders Them when viewerTeam differs from the caller's team (AC4)", () => {
    render(<TrumpIndicator trumpSuit="H" trumpCallerSeat={1} viewerTeam="teamA" />);
    const badge = screen.getByTestId("trump-caller-team");
    expect(badge).toHaveTextContent("Them");
    expect(badge).toHaveAttribute("data-team", "teamB");
  });

  it("falls back to neutral Team A / Team B when viewerTeam is null (AC4)", () => {
    const { rerender } = render(
      <TrumpIndicator trumpSuit="S" trumpCallerSeat={0} viewerTeam={null} />,
    );
    expect(screen.getByTestId("trump-caller-team")).toHaveTextContent("Team A");

    rerender(<TrumpIndicator trumpSuit="H" trumpCallerSeat={3} viewerTeam={null} />);
    expect(screen.getByTestId("trump-caller-team")).toHaveTextContent("Team B");
  });

  it("trump caller name span has truncate clamp (AC6)", () => {
    render(
      <TrumpIndicator
        trumpSuit="S"
        trumpCallerSeat={0}
        trumpCallerName="aVeryLongUsernameThatShouldClamp"
      />,
    );
    const nameEl = screen.getByTestId("trump-caller-name");
    expect(nameEl.className).toContain("max-w-32");
    expect(nameEl.className).toContain("truncate");
  });
});
