import "@/shared/i18n/i18n";

import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { makeUser } from "@/test-utils";

import { TrumpPrompt } from "./TrumpPrompt";

// --- Scope Amendment 1: the suit picker shows the deck the player will see ---
describe("TrumpPrompt card deck", () => {
  afterEach(() => {
    useAuthStore.setState({ token: null, user: null, isLoading: false });
  });

  function signIn(deck: "french" | "croatian") {
    useAuthStore.setState({ user: makeUser({ cardDeckPreference: deck }), isLoading: false });
  }

  it("draws Croatian icons on all four picker tiles", () => {
    signIn("croatian");
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
      const mark = screen.getByTestId(`suit-mark-${suit}`);
      expect(mark.tagName).toBe("IMG");
      expect(mark).toHaveAttribute("src", `/suits/croatian/${suit}.webp`);
    }
    // Tile ink follows the accent too — a bells tile is gold, not French red.
    // rgb() because jsdom re-serialises hex in `color`.
    expect(screen.getByTestId("trump-prompt-suit-D").style.color).toBe("rgb(201, 162, 60)");
  });

  it("keeps the French tiles on glyphs, unchanged", () => {
    signIn("french");
    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );

    expect(screen.getByTestId("suit-mark-H")).toHaveTextContent("♥");
    expect(screen.getByTestId("suit-mark-H").tagName).toBe("SPAN");
    expect(screen.getByTestId("trump-prompt-suit-H").style.color).toBe("rgb(198, 40, 40)");
  });

  it("names each tile in the ACTIVE DECK's vocabulary, not the French one", () => {
    signIn("croatian");
    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );

    // The mark is decorative, so the button's aria-label is the ONLY thing a
    // screen-reader user gets — and a gold bell announced as "Diamonds" is the
    // defect this pins shut. Names follow the deck exactly as the artwork does.
    for (const suit of ["S", "H", "D", "C"] as const) {
      expect(screen.getByTestId(`trump-prompt-suit-${suit}`)).toBeInTheDocument();
    }
    expect(screen.getByLabelText("Bells")).toBeInTheDocument();
    expect(screen.getByLabelText("Leaves")).toBeInTheDocument();
    expect(screen.getByLabelText("Acorns")).toBeInTheDocument();
    expect(screen.queryByLabelText("Diamonds")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Spades")).not.toBeInTheDocument();
  });

  it("keeps the French names on the French deck", () => {
    signIn("french");
    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={2}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );

    expect(screen.getByLabelText("Diamonds")).toBeInTheDocument();
    expect(screen.getByLabelText("Spades")).toBeInTheDocument();
  });

  // Item 13: three of four seats see the WAITING branch during bidding, and its
  // "considering" chips are deck-aware too — every other deck test here renders
  // the active bidder.
  it("draws Croatian icons on the waiting view's considering chips", () => {
    signIn("croatian");
    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={2}
        isActiveBidder={false}
        activePlayerName="Bob"
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );

    for (const suit of ["S", "H", "D", "C"] as const) {
      const chip = screen.getByTestId(`trump-prompt-considering-${suit}`);
      expect(chip).toBeInTheDocument();
      const mark = screen.getByTestId(`suit-mark-${suit}`);
      expect(mark.tagName).toBe("IMG");
      expect(mark).toHaveAttribute("src", `/suits/croatian/${suit}.webp`);
    }
    // Bells chip ink is gold, not the French red.
    expect(screen.getByTestId("trump-prompt-considering-D").style.color).toBe("rgb(201, 162, 60)");
    expect(screen.getByLabelText("Bells")).toBeInTheDocument();
  });

  it("keeps the waiting view's considering chips on glyphs for the French deck", () => {
    signIn("french");
    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={2}
        isActiveBidder={false}
        activePlayerName="Bob"
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );

    expect(screen.getByTestId("suit-mark-D").tagName).toBe("SPAN");
    expect(screen.getByTestId("trump-prompt-considering-D").style.color).toBe("rgb(198, 40, 40)");
  });
});

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

  // --- Candidate-less bidding (a variant that names trump freely in its
  // single round). The Bitola assertions above must keep holding unchanged.
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

    it("is driven by candidate-absence, not the round number", () => {
      // The single-round variant never leaves round 1, but the component must
      // key off trumpCandidate === null alone — a stray round value from a
      // stale snapshot must not change the grid.
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
      // Passing is still offered: only the dealer's own (fourth) pass is
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
  // a taker, the dealer bidding last (fourth, in the single round) has no legal
  // pass: the server refuses it outright. Rendering the button anyway turned the
  // only visible "get out of this" control into a guaranteed error toast, so it
  // is hidden. canPass is server-derived (matchState.mustPickTrump) — this
  // component never infers the rule, which is why every case here sets it
  // explicitly.
  describe("forced pick (canPass=false)", () => {
    it("hides the Pass control from the active bidder", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
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
          biddingRound={1}
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
          biddingRound={1}
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
          biddingRound={1}
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
          biddingRound={1}
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
          biddingRound={1}
          isActiveBidder={false}
          activePlayerName="bidder"
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt")).toHaveTextContent(/or pass/i);
    });
  });

  // --- Footer round counter. Only a candidate variant has a second round, so
  // only a candidate render may count rounds; a candidate-less variant bids in
  // a single round and must not show "Round 1 / 2".
  describe("footer round counter", () => {
    it("shows the round label while a candidate is on the table", () => {
      render(
        <TrumpPrompt
          trumpCandidate={trumpCandidate}
          biddingRound={2}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt")).toHaveTextContent("Round 2 / 2");
    });

    it("shows the round label in round 1 too while a candidate is on the table", () => {
      // Pins the guard to trumpCandidate !== null, not biddingRound === 2 — a
      // regression to the round test would pass the other two cases here.
      render(
        <TrumpPrompt
          trumpCandidate={trumpCandidate}
          biddingRound={1}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt")).toHaveTextContent("Round 1 / 2");
    });

    it("hides the round label for candidate-less bidding", () => {
      render(
        <TrumpPrompt
          trumpCandidate={null}
          biddingRound={1}
          isActiveBidder={true}
          onPick={vi.fn()}
          onPass={vi.fn()}
        />,
      );
      expect(screen.getByTestId("trump-prompt")).not.toHaveTextContent(/Round \d/);
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
