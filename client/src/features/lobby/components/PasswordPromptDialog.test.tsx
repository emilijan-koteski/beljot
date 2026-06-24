import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";

function renderDialog(overrides: Partial<React.ComponentProps<typeof PasswordPromptDialog>> = {}) {
  const props = {
    open: true,
    roomName: "Private Table",
    pending: false,
    errorKey: null as string | null,
    onSubmit: vi.fn(),
    onClose: vi.fn(),
    ...overrides,
  };
  render(<PasswordPromptDialog {...props} />);
  return props;
}

describe("PasswordPromptDialog", () => {
  it("renders the prompt with the room name when open", () => {
    renderDialog();
    expect(screen.getByTestId("password-prompt-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("password-prompt-input")).toBeInTheDocument();
  });

  it("disables submit until a password is entered", async () => {
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByTestId("password-prompt-submit")).toBeDisabled();
    await user.type(screen.getByTestId("password-prompt-input"), "secret");
    expect(screen.getByTestId("password-prompt-submit")).toBeEnabled();
  });

  it("calls onSubmit with the entered password", async () => {
    const user = userEvent.setup();
    const props = renderDialog();

    await user.type(screen.getByTestId("password-prompt-input"), "letmein");
    await user.click(screen.getByTestId("password-prompt-submit"));

    expect(props.onSubmit).toHaveBeenCalledWith("letmein");
  });

  it("shows the error message when errorKey is set", () => {
    renderDialog({ errorKey: "room.errors.wrongPassword" });
    expect(screen.getByTestId("password-prompt-error")).toHaveTextContent(
      "Incorrect room password.",
    );
  });

  it("hides the error as soon as the player edits the password", async () => {
    const user = userEvent.setup();
    renderDialog({ errorKey: "room.errors.wrongPassword" });
    expect(screen.getByTestId("password-prompt-error")).toBeInTheDocument();

    await user.type(screen.getByTestId("password-prompt-input"), "x");

    // The stale "wrong password" message clears on edit — no need to resubmit.
    expect(screen.queryByTestId("password-prompt-error")).not.toBeInTheDocument();
  });

  it("calls onClose when cancel is clicked", async () => {
    const user = userEvent.setup();
    const props = renderDialog();
    await user.click(screen.getByTestId("password-prompt-cancel"));
    expect(props.onClose).toHaveBeenCalled();
  });
});
