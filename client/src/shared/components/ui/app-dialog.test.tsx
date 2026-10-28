import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Lock, UserX } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { AppDialog } from "./app-dialog";

function renderDialog(props: Partial<Parameters<typeof AppDialog>[0]> = {}) {
  const onClose = vi.fn();
  const onConfirm = vi.fn();
  render(
    <AppDialog
      open
      onClose={onClose}
      icon={<Lock />}
      title="Private room"
      description="Enter the password for Friday four."
      primary={{ label: "Join", onClick: onConfirm, testId: "primary" }}
      secondary={{ label: "Cancel", onClick: onClose, testId: "secondary" }}
      testId="app-dialog"
      {...props}
    />,
  );
  return { onClose, onConfirm };
}

describe("AppDialog", () => {
  it("names the dialog by its title and describes it by its description", () => {
    renderDialog();

    const dialog = screen.getByRole("dialog", { name: "Private room" });
    expect(dialog).toHaveAccessibleDescription("Enter the password for Friday four.");
  });

  it("puts the primary action before the way out", () => {
    renderDialog();

    const primary = screen.getByTestId("primary");
    const secondary = screen.getByTestId("secondary");
    expect(
      primary.compareDocumentPosition(secondary) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("gives the danger tone a solid danger primary", () => {
    renderDialog({
      tone: "danger",
      icon: <UserX />,
      primary: { label: "Kick", testId: "primary" },
    });

    expect(screen.getByTestId("app-dialog")).toHaveAttribute("data-tone", "danger");
    expect(screen.getByTestId("primary")).toHaveAttribute("data-variant", "danger");
    expect(screen.getByTestId("secondary")).toHaveAttribute("data-variant", "ghost");
  });

  it("closes on Escape", () => {
    const { onClose } = renderDialog();

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(onClose).toHaveBeenCalledOnce();
  });

  it("stays open and locks both actions while pending", () => {
    const { onClose } = renderDialog({ pending: true });

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByTestId("primary")).toBeDisabled();
    expect(screen.getByTestId("secondary")).toBeDisabled();
  });

  it("shows one error line as an alert", () => {
    renderDialog({ error: "Wrong password.", errorTestId: "error" });

    expect(screen.getByRole("alert")).toHaveTextContent("Wrong password.");
    expect(screen.getByTestId("error")).toBeInTheDocument();
  });

  it("submits through the primary when it wraps a form", async () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });

    await userEvent.click(screen.getByTestId("primary"));

    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it("does not submit while the primary is disabled", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit, primary: { label: "Join", disabled: true, testId: "primary" } });

    fireEvent.submit(screen.getByTestId("primary").closest("form")!);

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("ends a celebration in its one primary, without a way-out button", () => {
    renderDialog({
      tone: "celebration",
      art: <span data-testid="art">B</span>,
      secondary: undefined,
      primary: { label: "Continue", testId: "primary" },
    });

    expect(screen.getByTestId("art")).toBeInTheDocument();
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(screen.getByTestId("primary")).toHaveTextContent("Continue");
  });
});
