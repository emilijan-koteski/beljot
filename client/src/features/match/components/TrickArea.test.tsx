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
    // All four compass placeholders present, no cards.
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
    // Simulates the post-resolve frame: server has cleared currentTrick to []
    // but the dispatcher captured the four cards into pendingResolvedTrick so
    // the collect animation can run.
    render(
      <TrickArea
        trick={[]}
        winnerSeat={null}
        myPlayerSeat={0}
        pendingResolvedTrick={{ trick: trickCards, winnerSeat: 2 }}
      />,
    );

    // All four cards rendered from the snapshot.
    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-AD")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-9C")).toBeInTheDocument();

    // Winner glow on the partner's compass (seat 2 from viewer seat 0 → north).
    const trickArea = screen.getByTestId("trick-area");
    const glowEl = trickArea.querySelector('[class*="shadow-[0_0_20px_var(--color-accent)]"]');
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

    // Suppressed cards: hidden so the overlay can paint the flight.
    expect(screen.queryByTestId("playing-card-KS")).not.toBeInTheDocument();
    expect(screen.queryByTestId("playing-card-AD")).not.toBeInTheDocument();
    // Non-suppressed cards still render at their slots.
    expect(screen.getByTestId("playing-card-7H")).toBeInTheDocument();
    expect(screen.getByTestId("playing-card-9C")).toBeInTheDocument();
  });

  it("renders the union of snapshot and live trick while the collect window is open", () => {
    // Reproduces the vanish-and-pop bug seen in live play: the next trick's
    // lead arrives while pendingResolvedTrick still owns the display. When the
    // lead's throw flight completes (cardId leaves suppressedCardIds), the
    // card must paint statically — with snapshot-exclusive rendering it had no
    // painter and vanished until the snapshot cleared, then popped back in
    // with no animation.
    render(
      <TrickArea
        trick={[{ card: { rank: "T", suit: "S" }, playerSeat: 1 }]}
        winnerSeat={null}
        myPlayerSeat={0}
        pendingResolvedTrick={{ trick: trickCards, winnerSeat: 0 }}
      />,
    );

    // Snapshot cards still shown (they own the glow + collect sweep)...
    expect(screen.getByTestId("playing-card-KS")).toBeInTheDocument();
    // ...AND the next trick's landed lead is painted, not masked.
    expect(screen.getByTestId("playing-card-TS")).toBeInTheDocument();
  });

  it("scopes the winner glow to the snapshot card, not a next-trick card at the same compass", () => {
    // The next trick's leader IS the previous trick's winner, so the new lead
    // lands at the winner's compass. The glow must stay on the resolved card.
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
    // While the new lead is still flying (overlay paints it), its static copy
    // stays hidden — but the resolving trick's card at the same compass keeps
    // rendering beneath the flight.
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
