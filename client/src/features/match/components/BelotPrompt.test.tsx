import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectActionBeforeStatusQuo } from "@/test-utils";

import { BelotPrompt } from "./BelotPrompt";

describe("BelotPrompt", () => {
  it("renders the belot prompt overlay", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expect(screen.getByTestId("belot-prompt")).toBeInTheDocument();
  });

  it("shows ANNOUNCE and SKIP buttons", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expect(screen.getByTestId("belot-prompt-announce")).toBeInTheDocument();
    expect(screen.getByTestId("belot-prompt-decline")).toBeInTheDocument();
  });

  it("calls onAnnounce when ANNOUNCE is clicked", async () => {
    const user = userEvent.setup();
    const onAnnounce = vi.fn();
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={onAnnounce} onDecline={vi.fn()} />,
    );
    await user.click(screen.getByTestId("belot-prompt-announce"));
    expect(onAnnounce).toHaveBeenCalledOnce();
  });

  it("calls onDecline when SKIP is clicked", async () => {
    const user = userEvent.setup();
    const onDecline = vi.fn();
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={onDecline} />,
    );
    await user.click(screen.getByTestId("belot-prompt-decline"));
    expect(onDecline).toHaveBeenCalledOnce();
  });

  it("has role dialog and aria-modal", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("shows Belote copy when playing the Queen first", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expect(screen.getByText("Declare Belote")).toBeInTheDocument();
    expect(screen.getByText("You hold the trump King and Queen.")).toBeInTheDocument();
    expect(screen.getByTestId("belot-prompt-announce")).toHaveTextContent(/^Declare \+20$/);
  });

  it("shows Rebelote copy when the King comes first and the variant names it", () => {
    render(
      <BelotPrompt isKing={true} isRebelote={true} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expect(screen.getByText("Declare Rebelote")).toBeInTheDocument();
    expect(screen.getByTestId("belot-prompt-announce")).toHaveTextContent(/^Declare \+20$/);
  });

  it("keeps the Belote name on the King when the variant has no Rebelote", () => {
    render(
      <BelotPrompt isKing={true} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expect(screen.getByText("Declare Belote")).toBeInTheDocument();
    expect(screen.queryByText(/Rebelote/)).not.toBeInTheDocument();
  });

  it("says the +20 once, on the button", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expect(screen.getAllByText(/\+20/)).toHaveLength(1);
  });

  it("wraps the Decline button with the button-timer ring when per-move", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <BelotPrompt
        isKing={false}
        isRebelote={false}
        onAnnounce={vi.fn()}
        onDecline={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
      />,
    );
    const ring = screen.getByTestId("button-timer-ring");
    expect(ring).toBeInTheDocument();
    expect(ring.querySelector('[data-testid="belot-prompt-decline"]')).toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring in relaxed mode", () => {
    render(
      <BelotPrompt
        isKing={false}
        isRebelote={false}
        onAnnounce={vi.fn()}
        onDecline={vi.fn()}
        turnExpiresAt={null}
        timerDurationSec={0}
      />,
    );
    expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring when isActivePlayer is false", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <BelotPrompt
        isKing={false}
        isRebelote={false}
        onAnnounce={vi.fn()}
        onDecline={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
        isActivePlayer={false}
      />,
    );
    expect(screen.queryByTestId("button-timer-ring")).not.toBeInTheDocument();
  });

  it("Escape key triggers onDecline (AC7)", () => {
    const onDecline = vi.fn();
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={onDecline} />,
    );
    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onDecline).toHaveBeenCalledTimes(1);
  });

  it("outer wrapper uses fixed positioning (AC7)", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    const wrapper = screen.getByTestId("belot-prompt");
    expect(wrapper.className).toContain("fixed");
    expect(wrapper.className).not.toMatch(/(^|\s)absolute(\s|$)/);
  });

  describe("auto-skip on timer expiry", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });
    afterEach(() => {
      vi.useRealTimers();
    });

    it("does NOT fire onDecline when the in-dialog timer ring reaches zero (server-authoritative auto-skip)", () => {
      const onDecline = vi.fn();
      const expiry = new Date(Date.now() + 5000).toISOString();
      render(
        <BelotPrompt
          isKing={false}
          isRebelote={false}
          onAnnounce={vi.fn()}
          onDecline={onDecline}
          turnExpiresAt={expiry}
          timerDurationSec={5}
        />,
      );

      expect(onDecline).not.toHaveBeenCalled();
      act(() => {
        vi.advanceTimersByTime(6000);
      });
      expect(onDecline).not.toHaveBeenCalled();
    });
  });
});

describe("BelotPrompt footer order", () => {
  it("places announce before decline so the action sits left of the status quo", () => {
    render(
      <BelotPrompt isKing={false} isRebelote={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />,
    );
    expectActionBeforeStatusQuo("belot-prompt-announce", "belot-prompt-decline");
  });
});
