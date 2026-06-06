import "@/shared/i18n/i18n";

import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { BelotPrompt } from "./BelotPrompt";

describe("BelotPrompt", () => {
  it("renders the belot prompt overlay", () => {
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByTestId("belot-prompt")).toBeInTheDocument();
  });

  it("shows ANNOUNCE and SKIP buttons", () => {
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByTestId("belot-prompt-announce")).toBeInTheDocument();
    expect(screen.getByTestId("belot-prompt-decline")).toBeInTheDocument();
  });

  it("calls onAnnounce when ANNOUNCE is clicked", async () => {
    const user = userEvent.setup();
    const onAnnounce = vi.fn();
    render(<BelotPrompt isKing={false} onAnnounce={onAnnounce} onDecline={vi.fn()} />);
    await user.click(screen.getByTestId("belot-prompt-announce"));
    expect(onAnnounce).toHaveBeenCalledOnce();
  });

  it("calls onDecline when SKIP is clicked", async () => {
    const user = userEvent.setup();
    const onDecline = vi.fn();
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={onDecline} />);
    await user.click(screen.getByTestId("belot-prompt-decline"));
    expect(onDecline).toHaveBeenCalledOnce();
  });

  it("has role dialog and aria-modal", () => {
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />);
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("shows Belote copy when playing the Queen first", () => {
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByText("Declare Belote?")).toBeInTheDocument();
    expect(screen.getByTestId("belot-prompt-announce")).toHaveTextContent("Declare Belote");
  });

  it("shows Rebelote copy when playing the King first", () => {
    render(<BelotPrompt isKing={true} onAnnounce={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByText("Declare Rebelote?")).toBeInTheDocument();
    expect(screen.getByTestId("belot-prompt-announce")).toHaveTextContent("Declare Rebelote");
  });

  it("wraps the Decline button with the button-timer ring when per-move", () => {
    const expiry = new Date(Date.now() + 20000).toISOString();
    render(
      <BelotPrompt
        isKing={false}
        onAnnounce={vi.fn()}
        onDecline={vi.fn()}
        turnExpiresAt={expiry}
        timerDurationSec={30}
      />,
    );
    const ring = screen.getByTestId("button-timer-ring");
    expect(ring).toBeInTheDocument();
    // Auto-action target on expiry is Decline, so the ring must wrap it.
    expect(ring.querySelector('[data-testid="belot-prompt-decline"]')).toBeInTheDocument();
  });

  it("does not render the in-dialog timer ring in relaxed mode", () => {
    render(
      <BelotPrompt
        isKing={false}
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
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={onDecline} />);
    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onDecline).toHaveBeenCalledTimes(1);
  });

  it("outer wrapper uses fixed positioning (AC7)", () => {
    render(<BelotPrompt isKing={false} onAnnounce={vi.fn()} onDecline={vi.fn()} />);
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
      // Client never fires the action — the server runs its own per-move
      // timer and pushes auto-skip state, dismissing this prompt. A client
      // onExpire would race the server and surface a wrong-phase error toast.
      expect(onDecline).not.toHaveBeenCalled();
    });
  });
});
