import { act, render, screen } from "@testing-library/react";
import {
  createMemoryRouter,
  MemoryRouter,
  Route,
  RouterProvider,
  Routes,
  useLocation,
} from "react-router";
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
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
    expect(useRoomStore.getState().matchStarted).toBe(false);
  });

  it("clears the sticky matchStarted flag without navigating when already on the match page", () => {
    renderAt("/match/7");

    act(() => {
      useRoomStore.getState().setMatchStarted(true);
      useRoomStore.getState().setMatchStartedRoomId(7);
    });

    expect(screen.getByTestId("pathname")).toHaveTextContent("/match/7");
    expect(useRoomStore.getState().matchStartedRoomId).toBeNull();
    expect(useRoomStore.getState().matchStarted).toBe(false);
  });

  it("does nothing while no match has started", () => {
    renderAt("/lobby");
    expect(screen.getByTestId("pathname")).toHaveTextContent("/lobby");
  });

  function renderWithRouter(initialPath: string) {
    const router = createMemoryRouter([{ path: "*", element: <Harness /> }], {
      initialEntries: ["/somewhere-before", initialPath],
      initialIndex: 1,
    });
    render(<RouterProvider router={router} />);
    return router;
  }

  it("pushes onto the history stack when redirecting from the lobby", async () => {
    const router = renderWithRouter("/lobby");

    act(() => {
      useRoomStore.getState().setMatchStarted(true);
      useRoomStore.getState().setMatchStartedRoomId(7);
    });
    expect(router.state.location.pathname).toBe("/match/7");

    await act(async () => {
      await router.navigate(-1);
    });
    expect(router.state.location.pathname).toBe("/lobby");
  });

  it("replaces the current entry when redirecting from anywhere else", async () => {
    const router = renderWithRouter("/rooms/7");

    act(() => {
      useRoomStore.getState().setMatchStarted(true);
      useRoomStore.getState().setMatchStartedRoomId(7);
    });
    expect(router.state.location.pathname).toBe("/match/7");

    await act(async () => {
      await router.navigate(-1);
    });
    expect(router.state.location.pathname).toBe("/somewhere-before");
  });
});
