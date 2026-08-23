import "@/shared/i18n/i18n";

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { expectActionBeforeStatusQuo } from "@/test-utils";

import { KickPlayerDialog, TransferOwnershipDialog } from "./OwnerConfirmDialogs";

describe("KickPlayerDialog", () => {
  const seatedOpponent = { name: "ena_h", seat: 1, team: "B" as const };

  it("titles the kick with the target name and shows seat + Opponent relation", async () => {
    render(
      <KickPlayerDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        target={seatedOpponent}
      />,
    );
    expect(await screen.findByTestId("kick-dialog-title")).toHaveTextContent(
      "Kick ena_h from the room?",
    );
    expect(screen.getByText("Seat 2")).toBeInTheDocument();
    expect(screen.getByText("Opponent")).toBeInTheDocument();
  });

  it("shows 'Standing' and no relation badge for an unseated target", async () => {
    render(
      <KickPlayerDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        pending={false}
        target={{ name: "wanderer", seat: null, team: null }}
      />,
    );
    expect(await screen.findByText("Standing")).toBeInTheDocument();
    expect(screen.queryByText("Opponent")).toBeNull();
    expect(screen.queryByText("Partner")).toBeNull();
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
    // The current host reads as "You · current host" (name conveyed by the avatar initial).
    expect(screen.getByText(/current host/i)).toBeInTheDocument();
    // The new host is named in both the lede and the handoff caption.
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

    // Swapping the footer pair must not disturb the X, which stays pinned to
    // the top-right corner rather than joining the footer row.
    const close = screen.getByLabelText("Close");
    expect(close.style.position).toBe("absolute");
    expect(
      close.compareDocumentPosition(screen.getByTestId("kick-confirm")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
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
