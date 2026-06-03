import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TrumpPrompt } from "./TrumpPrompt";

const trumpCandidate = { rank: "K" as const, suit: "H" as const };

describe("TrumpPrompt", () => {
  it("renders the prompt overlay", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.getByTestId("trump-prompt")).toBeInTheDocument();
  });

  it("shows PICK and PASS buttons when active bidder in round 1", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.getByTestId("trump-prompt-pick")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-pass")).toBeInTheDocument();
  });

  it("calls onPick when PICK button is clicked in round 1", async () => {
    const user = userEvent.setup();
    const onPick = vi.fn();
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={onPick}
        onPass={vi.fn()}
      />,
    );
    await user.click(screen.getByTestId("trump-prompt-pick"));
    expect(onPick).toHaveBeenCalledWith();
  });

  it("calls onPass when PASS button is clicked", async () => {
    const user = userEvent.setup();
    const onPass = vi.fn();
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={onPass}
      />,
    );
    await user.click(screen.getByTestId("trump-prompt-pass"));
    expect(onPass).toHaveBeenCalledOnce();
  });

  it("shows waiting text for non-active bidder", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={false}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("trump-prompt-pick")).not.toBeInTheDocument();
    expect(screen.queryByTestId("trump-prompt-pass")).not.toBeInTheDocument();
  });

  it("shows the candidate card to a round 2 non-active bidder but no pickable suit buttons", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={false}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    // Waiting players now see the candidate card so they know what the active
    // bidder turned down — but never the interactive PICK/PASS suit buttons.
    expect(screen.getByTestId("playing-card-KH")).toBeInTheDocument();
    expect(screen.queryByTestId("trump-prompt-suit-S")).not.toBeInTheDocument();
  });

  it("shows all four suit chips to a round 2 non-active bidder, candidate locked", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={false}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    // All four suits render so waiting players see the full set; the candidate
    // (hearts) is shown disabled because it can't be picked in round 2.
    expect(screen.getByTestId("trump-prompt-considering")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-considering-S")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-considering-D")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-considering-C")).toBeInTheDocument();
    const candidateChip = screen.getByTestId("trump-prompt-considering-H");
    expect(candidateChip).toBeInTheDocument();
    expect(candidateChip).toHaveAttribute("aria-disabled", "true");
    expect(candidateChip).toHaveAttribute("data-locked", "true");
    // The three pickable suits are not flagged locked.
    expect(screen.getByTestId("trump-prompt-considering-S")).not.toHaveAttribute("data-locked");
  });

  it("shows all four suit buttons in round 2 for active bidder (candidate locked)", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.getByTestId("trump-prompt-suit-S")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-suit-H")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-suit-D")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-suit-C")).toBeInTheDocument();
  });

  it("calls onPick with suit when suit button clicked in round 2", async () => {
    const user = userEvent.setup();
    const onPick = vi.fn();
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={onPick}
        onPass={vi.fn()}
      />,
    );
    // Use spades — hearts is the candidate suit (KH) and is locked out in round 2.
    await user.click(screen.getByTestId("trump-prompt-suit-S"));
    expect(onPick).toHaveBeenCalledWith("S");
  });

  it("disables the candidate-suit button in round 2 and keeps the others enabled", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    // Candidate is KH — H stays in the grid as a visibly disabled tile so
    // the layout is stable and the lock-out is explicit.
    const lockedButton = screen.getByTestId("trump-prompt-suit-H");
    expect(lockedButton).toBeDisabled();
    expect(lockedButton).toHaveAttribute("aria-disabled", "true");

    expect(screen.getByTestId("trump-prompt-suit-S")).toBeEnabled();
    expect(screen.getByTestId("trump-prompt-suit-D")).toBeEnabled();
    expect(screen.getByTestId("trump-prompt-suit-C")).toBeEnabled();
  });

  it("does not call onPick when the disabled candidate-suit button is clicked", async () => {
    const user = userEvent.setup();
    const onPick = vi.fn();
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={onPick}
        onPass={vi.fn()}
      />,
    );
    await user.click(screen.getByTestId("trump-prompt-suit-H"));
    expect(onPick).not.toHaveBeenCalled();
  });

  it("renders the trump candidate card in round 2 for active bidder", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    // The originally face-up candidate (KH) is given to the picker as their
    // 8th card after they choose a suit, so it must stay visible alongside
    // the suit-selection grid.
    expect(screen.getByTestId("playing-card-KH")).toBeInTheDocument();
  });

  it("renders the trump candidate card in round 1 for active bidder", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.getByTestId("playing-card-KH")).toBeInTheDocument();
  });

  it("does not render a candidate card when trumpCandidate is null", () => {
    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.queryByTestId(/^playing-card-/)).not.toBeInTheDocument();
    // Suit buttons still render so the player isn't blocked from picking.
    expect(screen.getByTestId("trump-prompt-suit-S")).toBeInTheDocument();
  });

  it("wraps the Pass button with the rounded-rect button-timer ring when active and per-move", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
      />,
    );
    const ring = screen.getByTestId("button-timer-ring");
    expect(ring).toBeInTheDocument();
    // Ring should wrap the Pass button so a Tab from the dialog still reaches Pass.
    expect(ring.querySelector('[data-testid="trump-prompt-pass"]')).toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring in relaxed mode (no turnExpiresAt)", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
        turnExpiresAt={null}
        timerDurationSec={0}
      />,
    );
    expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring for non-active bidders", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={false}
        onPick={vi.fn()}
        onPass={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
      />,
    );
    expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
  });

  it("has role dialog and aria-modal", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  describe("auto-pass on timer expiry", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });
    afterEach(() => {
      vi.useRealTimers();
    });

    it("does NOT fire onPass when the in-dialog timer ring reaches zero (server-authoritative auto-pass)", () => {
      const onPass = vi.fn();
      const expiry = new Date(Date.now() + 5000).toISOString();
      render(
        <TrumpPrompt
          trumpCandidate={trumpCandidate}
          biddingRound={1}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={onPass}
          turnExpiresAt={expiry}
          timerDurationSec={5}
        />,
      );

      expect(onPass).not.toHaveBeenCalled();
      act(() => {
        vi.advanceTimersByTime(6000);
      });
      // Server-authoritative: auto-pass arrives via state push, not from the client.
      expect(onPass).not.toHaveBeenCalled();
    });
  });

  it("inner dialog has overflow guard for short viewports (AC5)", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    // Constraint lives on the panel itself, not the dialog wrapper — wrapping
    // the panel in an overflow-clipping div clips the brass halo at the
    // wrapper's rectangular bounds. Putting the constraint on the panel
    // leaves its own box-shadow intact.
    const dialog = screen.getByRole("dialog");
    const panel = dialog.firstElementChild as HTMLElement | null;
    expect(panel).not.toBeNull();
    expect(panel!.style.maxHeight).toBe("90vh");
    expect(panel!.style.overflowY).toBe("auto");
  });
});
