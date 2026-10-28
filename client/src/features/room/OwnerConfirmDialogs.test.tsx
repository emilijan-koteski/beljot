import "@/shared/i18n/i18n";

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { expectActionBeforeStatusQuo } from "@/test-utils";

import { KickPlayerDialog, RemoveBotDialog, TransferOwnershipDialog } from "./OwnerConfirmDialogs";

function renderDialog(
  Dialog: typeof KickPlayerDialog,
  overrides: Partial<Parameters<typeof KickPlayerDialog>[0]> = {},
) {
  const props = {
    open: true,
    onOpenChange: vi.fn(),
    onConfirm: vi.fn(),
    pending: false,
    name: "ena_h",
    ...overrides,
  };
  render(<Dialog {...props} />);
  return props;
}

describe("KickPlayerDialog", () => {
  it("titles the kick by name and says they can rejoin", async () => {
    renderDialog(KickPlayerDialog);
    expect(await screen.findByRole("dialog", { name: "Kick ena_h" })).toHaveAccessibleDescription(
      "They can rejoin any time.",
    );
    expect(screen.getByTestId("kick-confirm")).toHaveAttribute("data-variant", "danger");
  });

  it("drops the host-action eyebrow and the corner close", async () => {
    renderDialog(KickPlayerDialog);
    await screen.findByRole("dialog");
    expect(screen.queryByText("Host action")).toBeNull();
    expect(screen.queryByLabelText("Close")).toBeNull();
  });

  it("fires onConfirm on confirm and onOpenChange(false) on cancel", async () => {
    const props = renderDialog(KickPlayerDialog);
    fireEvent.click(await screen.findByTestId("kick-confirm"));
    expect(props.onConfirm).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("kick-cancel"));
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables the confirm button while the kick is pending", async () => {
    renderDialog(KickPlayerDialog, { pending: true });
    expect(await screen.findByTestId("kick-confirm")).toBeDisabled();
  });
});

describe("RemoveBotDialog", () => {
  it("titles the removal by the bot's name and says the seat opens", async () => {
    renderDialog(RemoveBotDialog, { name: "Bot 2" });
    expect(await screen.findByRole("dialog", { name: "Remove Bot 2" })).toHaveAccessibleDescription(
      "The seat opens right away.",
    );
    expect(screen.getByTestId("remove-bot-confirm")).toHaveAttribute("data-variant", "danger");
  });

  it("fires onConfirm on confirm and onOpenChange(false) on cancel", async () => {
    const props = renderDialog(RemoveBotDialog, { name: "Bot 2" });
    fireEvent.click(await screen.findByTestId("remove-bot-confirm"));
    expect(props.onConfirm).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("remove-bot-cancel"));
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("TransferOwnershipDialog", () => {
  it("names the new host and says only they can hand it back", async () => {
    renderDialog(TransferOwnershipDialog, { name: "filip" });
    expect(
      await screen.findByRole("dialog", { name: "Make filip host" }),
    ).toHaveAccessibleDescription("Only the new host can hand it back.");
    expect(screen.getByTestId("transfer-confirm")).toHaveTextContent("Make host");
    expect(screen.getByTestId("transfer-confirm")).toHaveAttribute("data-variant", "default");
  });

  it("drops the hand-over card and the list of host powers", async () => {
    renderDialog(TransferOwnershipDialog, { name: "filip" });
    await screen.findByRole("dialog");
    expect(screen.queryByText(/current host/i)).toBeNull();
    expect(screen.queryByText(/Kicking & promoting players/)).toBeNull();
  });

  it("fires onConfirm on confirm and onOpenChange(false) on cancel", async () => {
    const props = renderDialog(TransferOwnershipDialog, { name: "filip" });
    fireEvent.click(await screen.findByTestId("transfer-confirm"));
    expect(props.onConfirm).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("transfer-cancel"));
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("owner confirm dialog footer order", () => {
  it("places the kick confirm before its cancel", async () => {
    renderDialog(KickPlayerDialog);
    await screen.findByTestId("kick-dialog-title");
    expectActionBeforeStatusQuo("kick-confirm", "kick-cancel");
  });

  it("places the remove-bot confirm before its cancel", async () => {
    renderDialog(RemoveBotDialog, { name: "Bot 2" });
    await screen.findByTestId("remove-bot-dialog-title");
    expectActionBeforeStatusQuo("remove-bot-confirm", "remove-bot-cancel");
  });

  it("places the transfer confirm before its cancel", async () => {
    renderDialog(TransferOwnershipDialog);
    await screen.findByTestId("transfer-dialog-title");
    expectActionBeforeStatusQuo("transfer-confirm", "transfer-cancel");
  });
});
