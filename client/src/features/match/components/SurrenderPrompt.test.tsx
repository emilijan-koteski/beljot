import "@/shared/i18n/i18n";

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { expectActionBeforeStatusQuo } from "@/test-utils";

import { SurrenderPrompt } from "./SurrenderPrompt";

describe("SurrenderPrompt", () => {
  it("renders the prompt overlay", () => {
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByTestId("surrender-prompt")).toBeInTheDocument();
  });

  it("shows accept and decline buttons", () => {
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByTestId("surrender-prompt-accept")).toBeInTheDocument();
    expect(screen.getByTestId("surrender-prompt-decline")).toBeInTheDocument();
  });

  it("interpolates the proposer username into the body copy", () => {
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={vi.fn()} />);
    expect(screen.getByText(/alice/)).toBeInTheDocument();
  });

  it("calls onAccept when accept is clicked", async () => {
    const user = userEvent.setup();
    const onAccept = vi.fn();
    render(<SurrenderPrompt proposerUsername="alice" onAccept={onAccept} onDecline={vi.fn()} />);
    await user.click(screen.getByTestId("surrender-prompt-accept"));
    expect(onAccept).toHaveBeenCalledOnce();
  });

  it("calls onDecline when decline is clicked", async () => {
    const user = userEvent.setup();
    const onDecline = vi.fn();
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={onDecline} />);
    await user.click(screen.getByTestId("surrender-prompt-decline"));
    expect(onDecline).toHaveBeenCalledOnce();
  });

  it("renders as an aria-modal dialog", () => {
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={vi.fn()} />);
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("Escape key triggers onDecline (AC7)", () => {
    const onDecline = vi.fn();
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={onDecline} />);
    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onDecline).toHaveBeenCalledTimes(1);
  });

  it("outer wrapper uses fixed positioning (AC7)", () => {
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={vi.fn()} />);
    const wrapper = screen.getByTestId("surrender-prompt");
    expect(wrapper.className).toContain("fixed");
    expect(wrapper.className).not.toMatch(/(^|\s)absolute(\s|$)/);
  });
});

describe("SurrenderPrompt footer order", () => {
  it("places accept before decline so the action sits left of the status quo", () => {
    render(<SurrenderPrompt proposerUsername="alice" onAccept={vi.fn()} onDecline={vi.fn()} />);
    expectActionBeforeStatusQuo("surrender-prompt-accept", "surrender-prompt-decline");
  });
});
