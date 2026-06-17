import { act, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it } from "vitest";

import { useRoomStore } from "@/shared/stores/roomStore";

import { useMatchStartRedirect } from "./useMatchStartRedirect";

function Harness() {
  useMatchStartRedirect();
  const location = useLocation();
  return <div data-testid="pathname">{location.pathname}</div>;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="*" element={<Harness />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("useMatchStartRedirect (D145b)", () => {
  beforeEach(() => {
    useRoomStore.getState().reset();
  });

  it("navigates a seated player into /match/:roomId when match_started fires off RoomPage", () => {
    renderAt("/lobby");

    act(() => {
      useRoomStore.getState().setMatchStarted(true);
      useRoomStore.getState().setMatchStartedRoomId(7);
    });

    expect(screen.getByTestId("pathname")).toHaveTextContent("/match/7");
    // Signal consumed so it cannot fire again.
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
    expect(useRoomStore.getState().matchStarted).toBe(false);
  });

  it("clears the sticky matchStarted flag without navigating when already on the match page", () => {
    renderAt("/match/7");

    act(() => {
      useRoomStore.getState().setMatchStarted(true);
      useRoomStore.getState().setMatchStartedRoomId(7);
    });

    // Already on target → no navigation, but the sticky flag is cleared so a
    // later RoomPage mount is not bounced straight back to /match (edge fix).
    expect(screen.getByTestId("pathname")).toHaveTextContent("/match/7");
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
    expect(useRoomStore.getState().matchStarted).toBe(false);
  });

  it("does nothing while no match has started", () => {
    renderAt("/lobby");
    expect(screen.getByTestId("pathname")).toHaveTextContent("/lobby");
  });
});
