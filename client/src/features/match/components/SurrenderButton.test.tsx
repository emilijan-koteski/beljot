import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { expectActionBeforeStatusQuo } from "@/test-utils";

import { SurrenderButton } from "./SurrenderButton";

describe("SurrenderButton", () => {
  it("renders enabled with the default surrender label when canRequest is true", () => {
    render(
      <SurrenderButton
        canRequest={true}
        isExhausted={false}
        isPending={false}
        onConfirm={vi.fn()}
      />,
    );
    const button = screen.getByTestId("surrender-button");
    expect(button).toBeEnabled();
    // The HUD button is icon-only; the verbose status reads through aria-label.
    expect(button.getAttribute("aria-label")).toMatch(/Surrender/i);
  });

  it("is disabled and exposes the exhausted status via aria-label when isExhausted is true", () => {
    render(
      <SurrenderButton
        canRequest={false}
        isExhausted={true}
        isPending={false}
        onConfirm={vi.fn()}
      />,
    );
    const button = screen.getByTestId("surrender-button");
    expect(button).toBeDisabled();
    expect(button.getAttribute("aria-label")).toMatch(/used/i);
  });

  it("is disabled and exposes the pending status via aria-label when isPending is true", () => {
    render(
      <SurrenderButton
        canRequest={false}
        isExhausted={false}
        isPending={true}
        onConfirm={vi.fn()}
      />,
    );
    const button = screen.getByTestId("surrender-button");
    expect(button).toBeDisabled();
    expect(button.getAttribute("aria-label")).toMatch(/pending/i);
  });

  it("opens the confirm dialog on click and fires onConfirm when confirmed", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <SurrenderButton
        canRequest={true}
        isExhausted={false}
        isPending={false}
        onConfirm={onConfirm}
      />,
    );

    await user.click(screen.getByTestId("surrender-button"));

    expect(screen.getByTestId("surrender-confirm")).toBeInTheDocument();
    expect(screen.getByTestId("surrender-cancel")).toBeInTheDocument();

    await user.click(screen.getByTestId("surrender-confirm"));
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it("does NOT fire onConfirm when the cancel button is clicked", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <SurrenderButton
        canRequest={true}
        isExhausted={false}
        isPending={false}
        onConfirm={onConfirm}
      />,
    );

    await user.click(screen.getByTestId("surrender-button"));
    await user.click(screen.getByTestId("surrender-cancel"));
    expect(onConfirm).not.toHaveBeenCalled();
  });
});

describe("SurrenderConfirmDialog footer order", () => {
  it("places confirm before cancel so the action sits left of the status quo", async () => {
    const user = userEvent.setup();
    render(
      <SurrenderButton
        canRequest={true}
        isExhausted={false}
        isPending={false}
        onConfirm={vi.fn()}
      />,
    );

    await user.click(screen.getByTestId("surrender-button"));
    expectActionBeforeStatusQuo("surrender-confirm", "surrender-cancel");
  });
});
