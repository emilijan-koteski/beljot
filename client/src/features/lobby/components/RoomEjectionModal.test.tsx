import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { useRoomStore } from "@/shared/stores/roomStore";

import { RoomEjectionModal } from "./RoomEjectionModal";

describe("RoomEjectionModal", () => {
  beforeEach(() => {
    useRoomStore.getState().setRoomEjection(null);
  });
  afterEach(() => {
    useRoomStore.getState().setRoomEjection(null);
  });

  it("renders nothing when there is no ejection signal", () => {
    render(<RoomEjectionModal />);
    expect(screen.queryByTestId("room-ejection-modal")).not.toBeInTheDocument();
  });

  it("renders the ejected modal with the composed balance and buy-in", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "insolvent",
    });
    render(<RoomEjectionModal />);

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
    render(<RoomEjectionModal />);

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
    render(<RoomEjectionModal />);

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
    render(<RoomEjectionModal />);

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
    render(<RoomEjectionModal />);

    expect(screen.getByTestId("room-ejection-body")).toHaveAttribute("data-honor", "0");
  });

  it("does not expose honor attributes on the insolvency branch", () => {
    useRoomStore.getState().setRoomEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "insolvent",
    });
    render(<RoomEjectionModal />);

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
    render(<RoomEjectionModal />);

    expect(screen.getByTestId("room-ejection-action")).toBeInTheDocument();
    useRoomStore.getState().setRoomEjection(null);
    expect(useRoomStore.getState().roomEjection).toBeNull();
  });
});
