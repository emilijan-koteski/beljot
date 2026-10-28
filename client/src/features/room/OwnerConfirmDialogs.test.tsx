import "@/shared/i18n/i18n";

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { expectActionBeforeStatusQuo } from "@/test-utils";

import { KickPlayerDialog, TransferOwnershipDialog } from "./OwnerConfirmDialogs";

describe("KickPlayerDialog", () => {
  const seatedOpponent = { name: "ena_h", seat: 1, team: "B" as const };

  it("titles the kick by name and says they can rejoin", async () => {
    render(
      <KickPlayerDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        target={seatedOpponent}
      />,
    );
    expect(await screen.findByRole("dialog", { name: "Kick ena_h" })).toHaveAccessibleDescription(
      "They can rejoin any time.",
    );
    expect(screen.getByTestId("kick-confirm")).toHaveAttribute("data-variant", "danger");
  });

  it("drops the host-action eyebrow and the player card the title already covers", async () => {
    render(
      <KickPlayerDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        target={{ ...seatedOpponent, avatarUrl: "https://assets.test/avatars/ena/128.webp" }}
      />,
    );
    const dialog = await screen.findByRole("dialog");
    expect(screen.queryByText("Host action")).toBeNull();
    expect(screen.queryByText("Seat 2")).toBeNull();
    expect(screen.queryByText("Opponent")).toBeNull();
    expect(dialog.querySelector("img")).toBeNull();
  });

  it("fires onConfirm on confirm and onOpenChange(false) on cancel", async () => {
    const onConfirm = vi.fn();
    const onOpenChange = vi.fn();
    render(
      <KickPlayerDialog
        open
        onOpenChange={onOpenChange}
        onConfirm={onConfirm}
        pending={false}
        target={seatedOpponent}
      />,
    );
    fireEvent.click(await screen.findByTestId("kick-confirm"));
    expect(onConfirm).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("kick-cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables the confirm button while the kick is pending", async () => {
    render(
      <KickPlayerDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending
        target={seatedOpponent}
      />,
    );
    expect(await screen.findByTestId("kick-confirm")).toBeDisabled();
  });
});

describe("TransferOwnershipDialog", () => {
  const target = { name: "filip", seat: 3, team: "B" as const };

  it("shows the title, both hosts, the hand-over capabilities, and Make host", async () => {
    render(
      <TransferOwnershipDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        fromName="dejan_k"
        target={target}
      />,
    );
    expect(await screen.findByTestId("transfer-dialog-title")).toHaveTextContent(
      "Transfer room ownership to another player?",
    );
    expect(screen.getByText(/current host/i)).toBeInTheDocument();
    expect(screen.getAllByText(/filip/).length).toBeGreaterThan(0);
    expect(screen.getByText(/Kicking & promoting players/)).toBeInTheDocument();
    expect(screen.getByTestId("transfer-confirm")).toHaveTextContent("Make host");
  });

  it("fires onConfirm on confirm and onOpenChange(false) on cancel", async () => {
    const onConfirm = vi.fn();
    const onOpenChange = vi.fn();
    render(
      <TransferOwnershipDialog
        open
        onOpenChange={onOpenChange}
        onConfirm={onConfirm}
        pending={false}
        fromName="dejan_k"
        target={target}
      />,
    );
    fireEvent.click(await screen.findByTestId("transfer-confirm"));
    expect(onConfirm).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("transfer-cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("owner confirm dialog footer order", () => {
  it("places the kick confirm before its cancel", async () => {
    render(
      <KickPlayerDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        target={{ name: "ena_h", seat: 1, team: "B" }}
      />,
    );
    await screen.findByTestId("kick-dialog-title");
    expectActionBeforeStatusQuo("kick-confirm", "kick-cancel");
    expect(screen.queryByLabelText("Close")).toBeNull();
  });

  it("places the transfer confirm before its cancel", async () => {
    render(
      <TransferOwnershipDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        fromName="owner"
        target={{ name: "ena_h", seat: 1, team: "B" }}
      />,
    );
    await screen.findByTestId("transfer-dialog-title");
    expectActionBeforeStatusQuo("transfer-confirm", "transfer-cancel");
  });
});
