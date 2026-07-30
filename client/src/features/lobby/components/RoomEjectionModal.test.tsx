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
});
