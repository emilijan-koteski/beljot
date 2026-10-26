import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { TrickCard } from "@/shared/types/matchTypes";

import { TrickArea } from "./TrickArea";

const trickCards: TrickCard[] = [
  { card: { rank: "K", suit: "S" }, playerSeat: 0 },
  { card: { rank: "7", suit: "H" }, playerSeat: 1 },
  { card: { rank: "A", suit: "D" }, playerSeat: 2 },
  { card: { rank: "9", suit: "C" }, playerSeat: 3 },
];

describe("TrickArea", () => {
  it("renders empty state with placeholders only", () => {
    render(<TrickArea trick={[]} winnerSeat={null} myPlayerSeat={0} />);

    expect(screen.getByTestId("trick-area")).toBeInTheDocument();
    expect(screen.getByTestId("trick-slot-0")).toBeInTheDocument();
    expect(screen.getByTestId("trick-slot-1")).toBeInTheDocument();
    expect(screen.getByTestId("trick-slot-2")).toBeInTheDocument();
    expect(screen.getByTestId("trick-slot-3")).toBeInTheDocument();
    expect(screen.queryByTestId("playing-card-KS")).not.toBeInTheDocument();
  });

  it("renders played cards in correct compass positions", () => {
    render(<TrickArea trick={trickCards.slice(0, 2)} winnerSeat={null} myPlayerSeat={0} />);

    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
  });

  it("clears cards immediately when trick prop becomes empty (overlay owns motion)", () => {
    const { rerender } = render(
      <TrickArea trick={trickCards.slice(0, 2)} winnerSeat={null} myPlayerSeat={0} />,
    );

    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();

    rerender(<TrickArea trick={[]} winnerSeat={null} myPlayerSeat={0} />);

    expect(screen.queryByTestId("playing-card-KS")).not.toBeInTheDocument();
  });

  it("shows snapshot cards with winner glow when pendingResolvedTrick is set", () => {
    render(
      <TrickArea
        trick={[]}
        winnerSeat={null}
        myPlayerSeat={0}
        pendingResolvedTrick={{ trick: trickCards, winnerSeat: 2 }}
      />,
    );

    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-AD")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-9C")).toBeInTheDocument();

    const trickArea = screen.getByTestId("trick-area");
    const glowEl = trickArea.querySelector('[class*="shadow-[0_0_24px_var(--color-accent)]"]');
    expect(glowEl).toBeInTheDocument();
  });

  it("filters out cards present in suppressedCardIds (overlay paints them)", () => {
    render(
      <TrickArea
        trick={trickCards}
        winnerSeat={null}
        myPlayerSeat={0}
        suppressedCardIds={new Set(["KS", "AD"])}
      />,
    );

    expect(screen.queryByTestId("playing-card-KS")).not.toBeInTheDocument();
    expect(screen.queryByTestId("playing-card-AD")).not.toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-9C")).toBeInTheDocument();
  });

  it("renders the union of snapshot and live trick while the collect window is open", () => {
    render(
      <TrickArea
        trick={[{ card: { rank: "T", suit: "S" }, playerSeat: 1 }]}
        winnerSeat={null}
        myPlayerSeat={0}
        pendingResolvedTrick={{ trick: trickCards, winnerSeat: 0 }}
      />,
    );

    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-TS")).toBeInTheDocument();
  });

  it("scopes the winner glow to the snapshot card, not a next-trick card at the same compass", () => {
    render(
      <TrickArea
        trick={[{ card: { rank: "T", suit: "S" }, playerSeat: 1 }]}
        winnerSeat={null}
        myPlayerSeat={0}
        pendingResolvedTrick={{ trick: trickCards, winnerSeat: 1 }}
      />,
    );

    const resolved = screen.getByTestId("trick-slot-card-1-resolved");
    expect(resolved.className).toContain("shadow-");
    const live = screen.getByTestId("trick-slot-card-1");
    expect(live.className).not.toContain("shadow-");
  });

  it("suppresses a live card mid-flight without hiding the snapshot card at its compass", () => {
    render(
      <TrickArea
        trick={[{ card: { rank: "T", suit: "S" }, playerSeat: 1 }]}
        winnerSeat={null}
        myPlayerSeat={0}
        pendingResolvedTrick={{ trick: trickCards, winnerSeat: 1 }}
        suppressedCardIds={new Set(["TS"])}
      />,
    );

    expect(screen.queryByTestId("playing-card-TS")).not.toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
  });
});
