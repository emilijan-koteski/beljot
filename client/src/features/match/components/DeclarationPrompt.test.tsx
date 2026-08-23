import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MOTION } from "@/shared/lib/motion";
import type { Declaration } from "@/shared/types/matchTypes";

import { DeclarationPrompt } from "./DeclarationPrompt";

const mockDeclarations: Declaration[] = [
  {
    type: "sequence",
    cards: [
      { rank: "J", suit: "D" },
      { rank: "Q", suit: "D" },
      { rank: "K", suit: "D" },
      { rank: "A", suit: "D" },
    ],
    playerSeat: 0,
    value: 50,
  },
];

describe("DeclarationPrompt", () => {
  it("renders the declaration prompt overlay", () => {
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={vi.fn()} onSkip={vi.fn()} />,
    );
    expect(screen.getByTestId("declaration-prompt")).toBeInTheDocument();
  });

  it("shows DECLARE and SKIP buttons", () => {
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={vi.fn()} onSkip={vi.fn()} />,
    );
    expect(screen.getByTestId("declaration-prompt-declare")).toBeInTheDocument();
    expect(screen.getByTestId("declaration-prompt-skip")).toBeInTheDocument();
  });

  it("displays declaration value", () => {
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={vi.fn()} onSkip={vi.fn()} />,
    );
    // Value appears in the group row and again in the total footer — both are fine.
    expect(screen.getAllByText(/50/).length).toBeGreaterThan(0);
  });

  it("shows total equal to sum of declaration values", () => {
    const multi: Declaration[] = [
      {
        type: "sequence",
        cards: [
          { rank: "7", suit: "S" },
          { rank: "8", suit: "S" },
          { rank: "9", suit: "S" },
        ],
        playerSeat: 0,
        value: 20,
      },
      {
        type: "four_of_a_kind",
        cards: [
          { rank: "J", suit: "S" },
          { rank: "J", suit: "H" },
          { rank: "J", suit: "D" },
          { rank: "J", suit: "C" },
        ],
        playerSeat: 0,
        value: 200,
      },
    ];
    render(<DeclarationPrompt declarations={multi} onDeclare={vi.fn()} onSkip={vi.fn()} />);
    const totalRow = screen.getByTestId("declaration-prompt-total");
    expect(totalRow).toHaveTextContent(/220/);
  });

  it("total matches single-declaration value when only one group is present", () => {
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={vi.fn()} onSkip={vi.fn()} />,
    );
    const totalRow = screen.getByTestId("declaration-prompt-total");
    expect(totalRow).toHaveTextContent(/50/);
  });

  it("calls onDeclare when DECLARE button is clicked", async () => {
    const user = userEvent.setup();
    const onDeclare = vi.fn();
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={onDeclare} onSkip={vi.fn()} />,
    );
    await user.click(screen.getByTestId("declaration-prompt-declare"));
    expect(onDeclare).toHaveBeenCalledOnce();
  });

  it("calls onSkip when SKIP button is clicked", async () => {
    const user = userEvent.setup();
    const onSkip = vi.fn();
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={vi.fn()} onSkip={onSkip} />,
    );
    await user.click(screen.getByTestId("declaration-prompt-skip"));
    expect(onSkip).toHaveBeenCalledOnce();
  });

  it("has role dialog and aria-modal", () => {
    render(
      <DeclarationPrompt declarations={mockDeclarations} onDeclare={vi.fn()} onSkip={vi.fn()} />,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("wraps the Skip button with the button-timer ring when per-move", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <DeclarationPrompt
        declarations={mockDeclarations}
        onDeclare={vi.fn()}
        onSkip={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
      />,
    );
    const ring = screen.getByTestId("button-timer-ring");
    expect(ring).toBeInTheDocument();
    expect(ring.querySelector('[data-testid="declaration-prompt-skip"]')).toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring in relaxed mode", () => {
    render(
      <DeclarationPrompt
        declarations={mockDeclarations}
        onDeclare={vi.fn()}
        onSkip={vi.fn()}
        turnExpiresAt={null}
        timerDurationSec={0}
      />,
    );
    expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring when isActivePlayer is false", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <DeclarationPrompt
        declarations={mockDeclarations}
        onDeclare={vi.fn()}
        onSkip={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
        isActivePlayer={false}
      />,
    );
    expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
  });

  describe("auto-skip on timer expiry", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });
    afterEach(() => {
      vi.useRealTimers();
    });

    it("does NOT fire onSkip when the in-dialog timer ring reaches zero (server-authoritative auto-skip)", () => {
      const onSkip = vi.fn();
      const expiry = new Date(Date.now() + 5000).toISOString();
      render(
        <DeclarationPrompt
          declarations={mockDeclarations}
          onDeclare={vi.fn()}
          onSkip={onSkip}
          turnExpiresAt={expiry}
          timerDurationSec={5}
        />,
      );

      expect(onSkip).not.toHaveBeenCalled();
      act(() => {
        vi.advanceTimersByTime(6000);
      });
      // Server-authoritative: auto-skip arrives via state push, not from the client.
      expect(onSkip).not.toHaveBeenCalled();
    });
  });

  // Simultaneous mode is the Croatian dedicated phase: every seat is asked at
  // once, so the dialog runs its own mount-anchored window, does not close on
  // the answer, and looks identical whether or not the viewer holds anything.
  describe("simultaneous mode", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });
    afterEach(() => {
      vi.useRealTimers();
    });

    function renderSim(props: Partial<React.ComponentProps<typeof DeclarationPrompt>> = {}) {
      return render(
        <DeclarationPrompt
          declarations={mockDeclarations}
          onDeclare={vi.fn()}
          onSkip={vi.fn()}
          simultaneous
          {...props}
        />,
      );
    }

    it("runs its own countdown ring with no server deadline", () => {
      renderSim();
      // turnExpiresAt is null for the whole phase — the ring is mount-anchored.
      expect(screen.getByTestId("button-timer-ring")).toBeInTheDocument();
    });

    it("fires onSkip when its own window elapses", () => {
      const onSkip = vi.fn();
      renderSim({ onSkip });

      expect(onSkip).not.toHaveBeenCalled();
      act(() => {
        vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP + 100);
      });
      // Unlike Bitola's prompt, the client IS the actor here — the server's
      // force-close ceiling sits well beyond this window rather than racing it.
      expect(onSkip).toHaveBeenCalledTimes(1);
    });

    it("renders an empty state with Declare disabled when the seat holds nothing", () => {
      renderSim({ declarations: [] });

      expect(screen.getByTestId("declaration-prompt-none")).toBeInTheDocument();
      expect(screen.queryByTestId("declaration-prompt-total")).not.toBeInTheDocument();
      expect(screen.getByTestId("declaration-prompt-declare")).toBeDisabled();
      expect(screen.getByTestId("declaration-prompt-skip")).toBeEnabled();
    });

    // The two panels must be the same DIALOG — same role, same title slot, same
    // button row — so that having one tells an onlooker nothing. Asserting
    // rendered width would be worthless here: jsdom has no layout and returns 0
    // for every element, so `0 === 0` would pass for two totally different
    // panels. Structure is what is actually checkable.
    it("keeps the same dialog shape for a meld holder and a meld-less seat", () => {
      function shapeOf(declarations: Declaration[]) {
        const { unmount } = renderSim({ declarations });
        const root = screen.getByTestId("declaration-prompt");
        const dialog = root.querySelector('[role="dialog"]')!;
        const shape = {
          hasDialog: dialog !== null,
          ariaModal: dialog.getAttribute("aria-modal"),
          labelledBy: dialog.getAttribute("aria-labelledby"),
          hasTitle: root.querySelector("#declaration-prompt-title") !== null,
          buttons: Array.from(root.querySelectorAll("button")).map((b) =>
            b.getAttribute("data-testid"),
          ),
          ring: root.querySelector('[data-testid="button-timer-ring"]') !== null,
        };
        unmount();
        return shape;
      }

      expect(shapeOf([])).toEqual(shapeOf(mockDeclarations));
    });

    it("disables both buttons and reports progress once answered", () => {
      renderSim({ answered: true, answeredCount: 3 });

      expect(screen.getByTestId("declaration-prompt-declare")).toBeDisabled();
      const skip = screen.getByTestId("declaration-prompt-skip");
      expect(skip).toBeDisabled();
      expect(skip).toHaveTextContent("3/4");
      // The countdown is over for this viewer — they are waiting on the others.
      expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
    });

    // The window's onExpire runs on its own schedule, while `answered` only
    // arrives a round-trip after the click. Without a local latch, a Skip
    // clicked at 7.9s is followed by the ring's auto-skip at 8.0s, and the
    // engine rejects that second answer as ErrWrongPhase — a toast for
    // something the player did correctly.
    it("sends only the first answer, even if the window expires mid-flight", () => {
      const onSkip = vi.fn();
      const onDeclare = vi.fn();
      renderSim({ onSkip, onDeclare });

      // Click just before the window closes; the server has not echoed back yet,
      // so `answered` is still false.
      fireEvent.click(screen.getByTestId("declaration-prompt-declare"));
      expect(onDeclare).toHaveBeenCalledTimes(1);

      act(() => {
        vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP + 100);
      });

      expect(onSkip).not.toHaveBeenCalled();
      expect(onDeclare).toHaveBeenCalledTimes(1);
    });

    it("shows the waiting state immediately on click, before the server echoes", () => {
      renderSim();

      fireEvent.click(screen.getByTestId("declaration-prompt-skip"));

      // answered is still false — this is the LOCAL latch talking.
      const skip = screen.getByTestId("declaration-prompt-skip");
      expect(skip).toBeDisabled();
      expect(screen.getByTestId("declaration-prompt-declare")).toBeDisabled();
    });

    it("ignores a double-click on the same button", () => {
      const onSkip = vi.fn();
      renderSim({ onSkip });

      const skip = screen.getByTestId("declaration-prompt-skip");
      fireEvent.click(skip);
      fireEvent.click(skip);

      expect(onSkip).toHaveBeenCalledTimes(1);
    });

    it("does not auto-skip once answered", () => {
      const onSkip = vi.fn();
      renderSim({ onSkip, answered: true, answeredCount: 1 });

      act(() => {
        vi.advanceTimersByTime(MOTION.DECLARATION_PHASE_AUTO_SKIP + 100);
      });
      expect(onSkip).not.toHaveBeenCalled();
    });
  });
});
