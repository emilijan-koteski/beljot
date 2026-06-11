import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown> & { defaultValue?: string }) => {
      const translations: Record<string, string> = {
        "team.us": "Us",
        "team.them": "Them",
        "match.score.tricks": "tricks",
        "match.score.trick": "trick",
        "match.score.thisHand": "this hand",
        "match.score.heading": "Scoreboard",
      };
      // Honour the component's defaultValue contract so the test mock matches
      // production i18n behavior (interpolated strings used by score-meta etc.).
      return (
        translations[key] ?? (typeof opts?.defaultValue === "string" ? opts.defaultValue : key)
      );
    },
  }),
}));

import { ScorePanel } from "./ScorePanel";

describe("ScorePanel", () => {
  beforeEach(() => {
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

  it("renders team scores with Us/Them labels for a teamA viewer", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={450}
        teamBScore={320}
        teamATricks={3}
        teamBTricks={2}
      />,
    );

    expect(screen.getByTestId("score-panel")).toBeInTheDocument();
    expect(screen.getByTestId("score-a")).toHaveTextContent("450");
    expect(screen.getByTestId("score-b")).toHaveTextContent("320");
    // Viewer is teamA, so the team-A row says "Us" and team-B says "Them"
    expect(screen.getByTestId("score-label-a")).toHaveTextContent("Us");
    expect(screen.getByTestId("score-label-b")).toHaveTextContent("Them");
  });

  it("renders the 501 target next to scores when matchTarget is 501", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={450}
        teamBScore={320}
        teamATricks={3}
        teamBTricks={2}
        matchTarget={501}
      />,
    );

    expect(screen.getByTestId("score-a")).toHaveTextContent("450 / 501");
    expect(screen.getByTestId("score-b")).toHaveTextContent("320 / 501");
  });

  it("renders Us/Them labels inverted for a teamB viewer", () => {
    render(
      <ScorePanel
        viewerTeam="teamB"
        teamAScore={450}
        teamBScore={320}
        teamATricks={3}
        teamBTricks={2}
      />,
    );

    expect(screen.getByTestId("score-label-a")).toHaveTextContent("Them");
    expect(screen.getByTestId("score-label-b")).toHaveTextContent("Us");
  });

  it("attaches data-team attributes on each row", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={1}
        teamBScore={2}
        teamATricks={0}
        teamBTricks={0}
      />,
    );

    expect(screen.getByTestId("score-row-a")).toHaveAttribute("data-team", "teamA");
    expect(screen.getByTestId("score-row-b")).toHaveAttribute("data-team", "teamB");
  });

  it("does not render the legacy 'Tricks: A - B' footer (per-row chip is the source of truth)", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={5}
        teamBTricks={3}
      />,
    );
    expect(screen.queryByTestId("score-tricks")).not.toBeInTheDocument();
  });

  it("renders Hand N · Variant in the header when both are provided", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent("Hand 3 · Bitola");
  });

  it("hides the metadata pill when neither handNumber nor variantLabel is provided", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
      />,
    );
    expect(screen.queryByTestId("score-meta")).not.toBeInTheDocument();
  });

  it("renders the viewer's row first (Us above Them) for a teamB viewer", () => {
    const { container } = render(
      <ScorePanel
        viewerTeam="teamB"
        teamAScore={100}
        teamBScore={250}
        teamATricks={1}
        teamBTricks={2}
      />,
    );
    const rows = container.querySelectorAll('[data-testid^="score-row-"]');
    // Viewer is teamB, so the teamB ("Us") row must precede the teamA ("Them") row.
    expect(rows[0]?.getAttribute("data-testid")).toBe("score-row-b");
    expect(rows[1]?.getAttribute("data-testid")).toBe("score-row-a");
  });

  it("has aria-live polite for accessibility", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
      />,
    );

    expect(screen.getByTestId("score-panel")).toHaveAttribute("aria-live", "polite");
  });

  it("applies transition classes for counter animation on score elements", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={100}
        teamBScore={200}
        teamATricks={0}
        teamBTricks={0}
      />,
    );

    const teamAScore = screen.getByTestId("score-a");
    expect(teamAScore.className).toContain("motion-safe:transition-all");
    expect(teamAScore.className).toContain("motion-safe:duration-300");
  });

  it("shows float-up bonus when lastTrickBonus prop is provided", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={100}
        teamBScore={200}
        teamATricks={4}
        teamBTricks={4}
        lastTrickBonus={10}
        lastTrickTeam={0}
      />,
    );

    const bonus = screen.getByTestId("score-bonus");
    expect(bonus).toHaveTextContent("+10");
    expect(bonus).toHaveAttribute("data-team", "teamA");
  });

  it("hides both potential lines when hand potentials are zero", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={100}
        teamBScore={200}
        teamATricks={0}
        teamBTricks={0}
        teamAHandPotential={0}
        teamBHandPotential={0}
      />,
    );

    expect(screen.queryByTestId("score-a-potential")).not.toBeInTheDocument();
    expect(screen.queryByTestId("score-b-potential")).not.toBeInTheDocument();
  });

  it("shows team-A potential line with summed hand points + declarations", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={100}
        teamBScore={200}
        teamATricks={2}
        teamBTricks={1}
        teamAHandPotential={12}
        teamBHandPotential={0}
      />,
    );

    const potential = screen.getByTestId("score-a-potential");
    expect(potential).toHaveTextContent("+12 this hand");
    expect(screen.queryByTestId("score-b-potential")).not.toBeInTheDocument();
  });

  it("shows each team's potential independently", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={100}
        teamBScore={200}
        teamATricks={2}
        teamBTricks={1}
        teamAHandPotential={12}
        teamBHandPotential={64}
      />,
    );

    expect(screen.getByTestId("score-a-potential")).toHaveTextContent("+12 this hand");
    expect(screen.getByTestId("score-b-potential")).toHaveTextContent("+64 this hand");
  });

  it("omits potential lines when props are not passed at all (default zero)", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={50}
        teamBScore={50}
        teamATricks={0}
        teamBTricks={0}
      />,
    );

    expect(screen.queryByTestId("score-a-potential")).not.toBeInTheDocument();
    expect(screen.queryByTestId("score-b-potential")).not.toBeInTheDocument();
  });
});
