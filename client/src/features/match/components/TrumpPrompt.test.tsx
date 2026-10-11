import "@/shared/i18n/i18n";

import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import { expectActionBeforeStatusQuo, makeUser } from "@/test-utils";

import { TrumpPrompt } from "./TrumpPrompt";

describe("TrumpPrompt card deck", () => {
  afterEach(() => {
    cleanup();
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
    expect(screen.getByTestId("trump-prompt-considering")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-considering-S")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-considering-D")).toBeInTheDocument();
    expect(screen.getByTestId("trump-prompt-considering-C")).toBeInTheDocument();
    const candidateChip = screen.getByTestId("trump-prompt-considering-H");
    expect(candidateChip).toBeInTheDocument();
    expect(candidateChip).toHaveAttribute("aria-disabled", "true");
    expect(candidateChip).toHaveAttribute("data-locked", "true");
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

  it("orders the picker tiles and the waiting chips spades, hearts, clubs, diamonds", () => {
    const suitsInDomOrder = (prefix: string) =>
      Array.from(document.querySelectorAll(`[data-testid^="${prefix}"]`)).map((el) =>
        el.getAttribute("data-testid")?.slice(prefix.length),
      );

    const { unmount } = render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(suitsInDomOrder("trump-prompt-suit-")).toEqual(["S", "H", "C", "D"]);
    unmount();

    render(
      <TrumpPrompt
        trumpCandidate={null}
        biddingRound={1}
        isActiveBidder={false}
        activePlayerName="Bob"
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expect(suitsInDomOrder("trump-prompt-considering-")).toEqual(["S", "H", "C", "D"]);
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
    expect(screen.getByTestId("trump-prompt-suit-S")).toBeInTheDocument();
  });

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
      for (const suit of ["S", "H", "D", "C"] as const) {
        expect(screen.getByTestId(`trump-prompt-suit-${suit}`)).toBeEnabled();
      }
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
    const dialog = screen.getByRole("dialog");
    const panel = dialog.firstElementChild as HTMLElement | null;
    expect(panel).not.toBeNull();
    expect(panel!.style.maxHeight).toBe("90vh");
    expect(panel!.style.overflowY).toBe("auto");
  });
});

describe("TrumpPrompt footer order", () => {
  it("places pick before pass so the action sits left of the status quo", () => {
    render(
      <TrumpPrompt
        trumpCandidate={trumpCandidate}
        biddingRound={1}
        isActiveBidder={true}
        onPick={vi.fn()}
        onPass={vi.fn()}
      />,
    );
    expectActionBeforeStatusQuo("trump-prompt-pick", "trump-prompt-pass");
  });
});
