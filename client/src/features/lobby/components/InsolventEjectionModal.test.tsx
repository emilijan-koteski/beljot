import "@/shared/i18n/i18n";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { useRoomStore } from "@/shared/stores/roomStore";

import { InsolventEjectionModal } from "./InsolventEjectionModal";

describe("InsolventEjectionModal", () => {
  beforeEach(() => {
    useRoomStore.getState().setInsolventEjection(null);
  });
  afterEach(() => {
    useRoomStore.getState().setInsolventEjection(null);
  });

  it("renders nothing when there is no ejection signal", () => {
    render(<InsolventEjectionModal />);
    expect(screen.queryByTestId("insolvent-ejection-modal")).not.toBeInTheDocument();
  });

  it("renders the ejected modal with the composed balance and buy-in", () => {
    useRoomStore.getState().setInsolventEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "ejected",
    });
    render(<InsolventEjectionModal />);

    expect(screen.getByTestId("insolvent-ejection-modal")).toBeInTheDocument();
    const body = screen.getByTestId("insolvent-ejection-body");
    expect(body).toHaveTextContent((200).toLocaleString());
    expect(body).toHaveTextContent((1500).toLocaleString());
  });

  it("renders the room-closed variant", () => {
    useRoomStore.getState().setInsolventEjection({
      roomId: 7,
      buyIn: 0,
      balance: 0,
      reason: "roomClosed",
    });
    render(<InsolventEjectionModal />);

    expect(screen.getByTestId("insolvent-ejection-modal")).toBeInTheDocument();
    expect(screen.getByTestId("insolvent-ejection-title")).toBeInTheDocument();
  });

  it("clears the signal when the action is clicked", async () => {
    useRoomStore.getState().setInsolventEjection({
      roomId: 7,
      buyIn: 1500,
      balance: 200,
      reason: "ejected",
    });
    render(<InsolventEjectionModal />);

    await userEvent.click(screen.getByTestId("insolvent-ejection-action"));
    expect(useRoomStore.getState().insolventEjection).toBeNull();
  });
});
