import "@/shared/i18n/i18n";

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

  // --- Candidate-less bidding (a variant that names trump freely in BOTH
  // rounds). The Bitola assertions above must keep holding unchanged.
  describe("no trump candidate (free-suit variant)", () => {
    it("renders the four-suit grid in ROUND 1 with nothing locked and no candidate card", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      for (const suit of ["S", "H", "D", "C"] as const) {
        const button = screen.getByTestId(`trump-prompt-suit-${suit}`);
        expect(button).toBeInTheDocument();
        expect(button).toBeEnabled();
        expect(button).toHaveAttribute("aria-disabled", "false");
      }
      expect(screen.queryByTestId(/^playing-card-/)).not.toBeInTheDocument();
    });

    it("renders the four-suit grid in ROUND 2 with nothing locked", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      for (const suit of ["S", "H", "D", "C"] as const) {
        expect(screen.getByTestId(`trump-prompt-suit-${suit}`)).toBeEnabled();
      }
    });

    it("hides the suitless Take button in round 1 — the server would reject it", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.queryByTestId("trump-prompt-pick")).not.toBeInTheDocument();
      // Passing is still offered: only the dealer's last round-2 pass is
      // refused, which the server reports through canPass (see the
      // forced-dealer block below), not something the prompt infers.
      expect(screen.getByTestId("trump-prompt-pass")).toBeInTheDocument();
    });

    it("calls onPick WITH a suit from the round-1 grid", async () => {
      const user = userEvent.setup();
      const onPick = vi.fn();
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
          isActiveBidder={true}
          onPick={onPick}
          onPass={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("trump-prompt-suit-H"));
      expect(onPick).toHaveBeenCalledWith("H");
    });

    it("renders the free-pick title and subtitle, not a raw i18n key", () => {
      // i18n.parity.test.ts only proves the key EXISTS in all four locales — a
      // typo in the key name here would render the key string itself, and only
      // an assertion on the rendered copy catches that.
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByText("Take trump — any suit")).toBeInTheDocument();
      expect(screen.getByText("All four suits are yours to take.")).toBeInTheDocument();
      // Neither the candidate-bound round-1 copy nor a bare key may appear.
      const panel = screen.getByTestId("trump-prompt");
      expect(panel.textContent).not.toContain("match.trumpPrompt");
      expect(panel.textContent).not.toContain("Take trump or pass");
    });

    it("uses the same free-pick copy in round 2", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByText("Take trump — any suit")).toBeInTheDocument();
      expect(screen.getByText("All four suits are yours to take.")).toBeInTheDocument();
    });

    it("shows waiting players all four suit chips in round 1, none locked", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
          isActiveBidder={false}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt-considering")).toBeInTheDocument();
      for (const suit of ["S", "H", "D", "C"] as const) {
        const chip = screen.getByTestId(`trump-prompt-considering-${suit}`);
        expect(chip).toBeInTheDocument();
        expect(chip).not.toHaveAttribute("data-locked");
      }
      expect(screen.queryByTestId(/^playing-card-/)).not.toBeInTheDocument();
    });
  });

  // Bitola regression: round 1 WITH a candidate must still be the
  // take-it-or-pass layout — one card, one Take button, no suit grid.
  it("keeps Bitola's candidate-bound titles unchanged in both rounds", () => {
    const { unmount } = render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.getByText("Take trump or pass")).toBeInTheDocument();
    expect(screen.getByText("Adopt this suit, or pass to the next player.")).toBeInTheDocument();
    unmount();

    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(screen.getByText("Choose any suit — or pass")).toBeInTheDocument();
    expect(screen.getByText("Any suit except the candidate.")).toBeInTheDocument();
  });

  it("round 1 with a candidate still renders the Take button and NO suit grid", () => {
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
    expect(screen.queryByTestId("trump-prompt-suit-S")).not.toBeInTheDocument();
    expect(screen.getByTestId("playing-card-KH")).toBeInTheDocument();
  });

  // --- The forced dealer (Story 12.8). Under a variant where the hand must find
  // a taker, the dealer bidding last in round 2 has no legal pass: the server
  // refuses it outright. Rendering the button anyway turned the only visible
  // "get out of this" control into a guaranteed error toast, so it is hidden.
  // canPass is server-derived (matchState.mustPickTrump) — this component never
  // infers the rule, which is why every case here sets it explicitly.
  describe("forced pick (canPass=false)", () => {
    it("hides the Pass control from the active bidder", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={true}
          canPass={false}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.queryByTestId("trump-prompt-pass")).not.toBeInTheDocument();
      // The four suits remain — they are the complete set of legal moves.
      for (const suit of ["S", "H", "D", "C"] as const) {
        expect(screen.getByTestId(`trump-prompt-suit-${suit}`)).toBeEnabled();
      }
      // And the player is told why, rather than left wondering.
      expect(screen.getByTestId("trump-prompt-must-pick")).toBeInTheDocument();
    });

    it("keeps the Pass control when canPass is true or omitted", () => {
      const { unmount } = render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={true}
          canPass={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt-pass")).toBeInTheDocument();
      unmount();

      // Omitted defaults to true, so every existing caller keeps its Pass.
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt-pass")).toBeInTheDocument();
      expect(screen.queryByTestId("trump-prompt-must-pick")).not.toBeInTheDocument();
    });

    it("keeps the countdown ring when the Pass button is gone", () => {
      const expiry = new Date(Date.now() + 20000).toISOString();
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={true}
          canPass={false}
          onPick={vi.fn()}
          onPass={vi.fn()}
          turnExpiresAt={expiry}
          timerDurationSec={30}
        />,
      );
      // The clock still runs — an expiry here auto-PICKS server-side — so the
      // ring must not disappear with the button it used to wrap.
      const ring = screen.getByTestId("button-timer-ring");
      expect(ring.querySelector('[data-testid="trump-prompt-must-pick"]')).toBeInTheDocument();
      expect(screen.queryByTestId("trump-prompt-pass")).not.toBeInTheDocument();
    });

    it("does not promise the sibling seats a pass that cannot happen", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={false}
          canPass={false}
          activePlayerName="dealer"
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      const prompt = screen.getByTestId("trump-prompt");
      expect(prompt).toHaveTextContent("dealer");
      expect(prompt).toHaveTextContent(/name a suit/i);
      expect(prompt).not.toHaveTextContent(/or pass/i);
    });

    it("still promises a pass to the sibling seats when one is legal", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={2}
          isActiveBidder={false}
          activePlayerName="bidder"
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt")).toHaveTextContent(/or pass/i);
    });
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
