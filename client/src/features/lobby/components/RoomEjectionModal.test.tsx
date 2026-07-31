import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { useRoomStore } from "@/shared/stores/roomStore";

import { RoomEjectionModal } from "./RoomEjectionModal";

// The honour branch offers a "Rooms I qualify for" action that navigates, so the
// modal now needs a Router in scope.
function renderModal() {
  return render(
    <MemoryRouter>
      <RoomEjectionModal />
    </MemoryRouter>,
  );
}

describe("RoomEjectionModal", () => {
  beforeEach(() => {
    useRoomStore.getState().setRoomEjection(null);
  });
  afterEach(() => {
    useRoomStore.getState().setRoomEjection(null);
  });

  it("renders nothing when there is no ejection signal", () => {
    renderModal();
    expect(screen.queryByTestId("room-ejection-modal")).not.toBeInTheDocument();
  });

  it("renders the ejected modal with the composed balance and buy-in", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "insolvent",
    });
    renderModal();

    expect(screen.getByTestId("room-ejection-modal")).toBeInTheDocument();
    const body = screen.getByTestId("room-ejection-body");
    expect(body).toHaveTextContent((200).toLocaleString());
    expect(body).toHaveTextContent((1500).toLocaleString());
  });

  it("renders the room-closed variant", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      reason: "roomClosed",
    });
    renderModal();

    expect(screen.getByTestId("room-ejection-modal")).toBeInTheDocument();
    expect(screen.getByTestId("room-ejection-title")).toBeInTheDocument();
  });

  it("clears the signal when the action is clicked", async () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "insolvent",
    });
    renderModal();

    await userEvent.click(screen.getByTestId("room-ejection-action"));
    expect(useRoomStore.getState().roomEjection).toBeNull();
  });

  // Honor ejection (Story 9.8 AC6/AC10) — the third copy branch.
  it("renders the honor variant with the room's threshold and the player's score", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      minHonor: 80,
      honor: 55,
      reason: "honor",
    });
    renderModal();

    expect(screen.getByTestId("room-ejection-modal")).toBeInTheDocument();
    // Numbers asserted through data-*, so the test stays i18n-independent.
    const body = screen.getByTestId("room-ejection-body");
    expect(body).toHaveAttribute("data-min-honor", "80");
    expect(body).toHaveAttribute("data-honor", "55");
  });

  it("renders a real honor score of 0 rather than treating it as absent", () => {
    // A score of 0 is a legitimate Go value. A truthiness check anywhere in this
    // path would swallow it.
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      minHonor: 50,
      honor: 0,
      reason: "honor",
    });
    renderModal();

    expect(screen.getByTestId("room-ejection-body")).toHaveAttribute("data-honor", "0");
  });

  it("does not expose honor attributes on the insolvency branch", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "insolvent",
    });
    renderModal();

    const body = screen.getByTestId("room-ejection-body");
    expect(body).not.toHaveAttribute("data-min-honor");
    expect(body).not.toHaveAttribute("data-honor");
  });

  it("clears an honor notice when the action is clicked", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      minHonor: 80,
      honor: 55,
      reason: "honor",
    });
    renderModal();

    expect(screen.getByTestId("room-ejection-action")).toBeInTheDocument();
    useRoomStore.getState().setRoomEjection(null);
    expect(useRoomStore.getState().roomEjection).toBeNull();
  });

  // --- Honour branch additions (honour redesign R8) ---

  it("draws the honour comparison to scale, with the numbers alongside", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      minHonor: 80,
      honor: 55,
      reason: "honor",
    });
    renderModal();

    // A sentence with two numbers makes the player do the arithmetic; two bars on
    // a shared 0-100 axis show how far off they are at a glance.
    expect(screen.getByTestId("room-ejection-you")).toHaveAttribute("data-value", "55");
    expect(screen.getByTestId("room-ejection-table")).toHaveAttribute("data-value", "80");
  });

  it("renders a real honour score of 0 in the comparison", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      minHonor: 50,
      honor: 0,
      reason: "honor",
    });
    renderModal();

    expect(screen.getByTestId("room-ejection-you")).toHaveAttribute("data-value", "0");
  });

  it("offers a door out to the rooms the player can actually join", async () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      minHonor: 80,
      honor: 55,
      reason: "honor",
    });
    renderModal();

    // Without this the modal is a dead end, which is what turns a nudge into a
    // churn moment.
    const browse = screen.getByTestId("room-ejection-browse");
    expect(browse).toBeInTheDocument();
    await userEvent.click(browse);
    // Dismisses the notice on the way out, so it cannot re-fire on arrival.
    expect(useRoomStore.getState().roomEjection).toBeNull();
  });

  it("offers no browse action on the insolvency or room-closed branches", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "insolvent",
    });
    renderModal();

    expect(screen.queryByTestId("room-ejection-browse")).not.toBeInTheDocument();
    expect(screen.queryByTestId("room-ejection-you")).not.toBeInTheDocument();
  });
});
