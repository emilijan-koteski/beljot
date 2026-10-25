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
        "lobby.card.noDeclarations": "No declarations",
        "lobby.card.stopAtTarget": "Stop at target",
        "lobby.card.noDeclarationsAriaLabel":
          "This room plays without declarations and without Belote",
        "lobby.card.stopAtTargetAriaLabel":
          "This room ends the match the moment a team reaches the target, without finishing the hand",
        "lobby.card.noDeclarationsTransfer": "No declaration transfer",
        "lobby.card.noDeclarationsTransferAriaLabel":
          "In this room, the declarations and Belote of a team that goes down or is swept are voided instead of going to the other team",
      };
      return (
        translations[key] ?? (typeof opts?.defaultValue === "string" ? opts.defaultValue : key)
      );
    },
  }),
}));

import { Z } from "@/shared/lib/zLayers";

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
    expect(screen.getByTestId("score-label-a")).toHaveTextContent("Us");
    expect(screen.getByTestId("score-label-b")).toHaveTextContent("Them");
  });

  it("renders the mobile scoreboard above the seat avatars (HUD z-tier)", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={100}
        teamBScore={80}
        teamATricks={1}
        teamBTricks={0}
      />,
    );

    expect(screen.getByTestId("score-panel-mobile").style.zIndex).toBe(String(Z.HUD));
    expect(Z.HUD).toBeGreaterThan(Z.SEATS);
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

  it("appends the no-declarations segment when the room plays without them", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
        declarationsEnabled={false}
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent("Hand 3 · Bitola · No declarations");
  });

  it("says nothing about declarations on an ordinary table", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
        declarationsEnabled
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent("Hand 3 · Bitola");
    expect(screen.queryByTestId("score-meta-no-declarations")).not.toBeInTheDocument();
  });

  it("defaults to declarations on, so a bare render is never mislabelled", () => {
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
    expect(screen.queryByTestId("score-meta-no-declarations")).not.toBeInTheDocument();
  });

  it("shows the no-declarations segment alone when there is no hand or variant", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        declarationsEnabled={false}
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent(/^No declarations$/);
  });

  it("appends the stop-at-target segment when the room plays dosta", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
        stopAtTarget
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent("Hand 3 · Bitola · Stop at target");
  });

  it("shows both rule segments when the table plays dosta without declarations", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
        declarationsEnabled={false}
        stopAtTarget
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent(
      "Hand 3 · Bitola · No declarations · Stop at target",
    );
  });

  it("defaults to finishing the hand, so a bare render is never mislabelled", () => {
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
    expect(screen.queryByTestId("score-meta-stop-at-target")).not.toBeInTheDocument();
  });

  it("shows the stop-at-target segment alone when there is no hand or variant", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        stopAtTarget
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent(/^Stop at target$/);
  });

  it("keeps the segment separator out of each rule chip's accessible name", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
        declarationsEnabled={false}
        stopAtTarget
      />,
    );

    const declarations = screen.getByTestId("score-meta-no-declarations");
    const stop = screen.getByTestId("score-meta-stop-at-target");

    expect(declarations).toHaveTextContent(/^No declarations$/);
    expect(stop).toHaveTextContent(/^Stop at target$/);

    expect(screen.getByTestId("score-meta")).toHaveTextContent(
      "Hand 3 · Bitola · No declarations · Stop at target",
    );

    expect(declarations).toHaveAccessibleName(/without declarations and without Belote/i);
    expect(stop).toHaveAccessibleName(/without finishing the hand/i);
  });

  it("defaults to transferring declarations, so a bare render carries no transfer segment", () => {
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
    expect(screen.queryByTestId("score-meta-no-declarations-transfer")).not.toBeInTheDocument();
  });

  it("appends the no-transfer segment after every other rule segment", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        handNumber={3}
        variantLabel="Bitola"
        declarationsEnabled={false}
        stopAtTarget
        declarationsTransfer={false}
      />,
    );
    const chip = screen.getByTestId("score-meta-no-declarations-transfer");
    expect(chip).toHaveTextContent(/^No declaration transfer$/);
    expect(chip).toHaveAccessibleName(/voided instead of going to the other team/i);
    expect(screen.getByTestId("score-meta")).toHaveTextContent(
      "Hand 3 · Bitola · No declarations · Stop at target · No declaration transfer",
    );
  });

  it("shows the no-transfer segment alone when there is no hand or variant", () => {
    render(
      <ScorePanel
        viewerTeam="teamA"
        teamAScore={0}
        teamBScore={0}
        teamATricks={0}
        teamBTricks={0}
        declarationsTransfer={false}
      />,
    );
    expect(screen.getByTestId("score-meta")).toHaveTextContent(/^No declaration transfer$/);
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
