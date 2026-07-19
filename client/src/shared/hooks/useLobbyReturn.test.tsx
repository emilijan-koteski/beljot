import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  canPopToLobby,
  resetLobbyReturnGuardForTests,
  useLobbyReturn,
  useMarkLobbyRoot,
} from "./useLobbyReturn";

const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

/** Simulates react-router's data-router bookkeeping for the history index. */
function setHistoryIdx(idx: number | null) {
  window.history.replaceState(idx === null ? null : { idx }, "");
}

describe("useLobbyReturn", () => {
  beforeEach(() => {
    sessionStorage.clear();
    setHistoryIdx(null);
    resetLobbyReturnGuardForTests();
  });

  afterEach(() => {
    mockNavigate.mockClear();
    sessionStorage.clear();
    setHistoryIdx(null);
    resetLobbyReturnGuardForTests();
  });

  it("pops back when the lobby entry sits directly beneath the current one", () => {
    setHistoryIdx(3);
    renderHook(() => useMarkLobbyRoot()); // lobby mounted at idx 3

    setHistoryIdx(4); // pushed one entry above the lobby (room / match / profile)
    const { result } = renderHook(() => useLobbyReturn());
    result.current();

    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith(-1);
  });

  it("replaces to /lobby when the entry beneath is not the lobby", () => {
    setHistoryIdx(1);
    renderHook(() => useMarkLobbyRoot()); // lobby recorded at idx 1

    setHistoryIdx(5); // several entries above — the one beneath is unknown
    const { result } = renderHook(() => useLobbyReturn());
    result.current();

    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("replaces to /lobby when no lobby index was ever recorded (deep link)", () => {
    setHistoryIdx(4);
    const { result } = renderHook(() => useLobbyReturn());
    result.current();

    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("replaces to /lobby when the current history index is missing", () => {
    sessionStorage.setItem("beljot:lobby-idx", "3");
    setHistoryIdx(null); // history.state carries no idx (fresh document)

    const { result } = renderHook(() => useLobbyReturn());
    result.current();

    expect(mockNavigate).toHaveBeenCalledWith("/lobby", { replace: true });
  });

  it("treats a non-numeric stored lobby index as not-poppable", () => {
    sessionStorage.setItem("beljot:lobby-idx", "garbage");
    setHistoryIdx(4);

    expect(canPopToLobby()).toBe(false);
  });

  it("reports canPopToLobby true only for the exact lobby-beneath layout", () => {
    setHistoryIdx(2);
    renderHook(() => useMarkLobbyRoot());

    setHistoryIdx(3);
    expect(canPopToLobby()).toBe(true);

    setHistoryIdx(2); // back on the lobby entry itself
    expect(canPopToLobby()).toBe(false);
  });

  it("does not record an index when history.state has none", () => {
    setHistoryIdx(null);
    renderHook(() => useMarkLobbyRoot());

    expect(sessionStorage.getItem("beljot:lobby-idx")).toBeNull();
  });

  it("treats an empty-string stored lobby index as not-poppable", () => {
    // Number("") is 0 — must not read as a valid lobby at index 0.
    sessionStorage.setItem("beljot:lobby-idx", "");
    setHistoryIdx(1);

    expect(canPopToLobby()).toBe(false);
  });

  it("ignores a second pop while one is already in flight (double-tap guard)", () => {
    setHistoryIdx(3);
    renderHook(() => useMarkLobbyRoot());
    setHistoryIdx(4);

    const { result } = renderHook(() => useLobbyReturn());
    result.current();
    result.current(); // double-tap: must not queue a second pop past the lobby

    expect(mockNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledWith(-1);
  });

  it("re-arms after the in-flight pop lands (popstate)", () => {
    setHistoryIdx(3);
    renderHook(() => useMarkLobbyRoot());
    setHistoryIdx(4);

    const { result } = renderHook(() => useLobbyReturn());
    result.current();
    window.dispatchEvent(new PopStateEvent("popstate"));
    setHistoryIdx(4); // user pushed above the lobby again later
    result.current();

    expect(mockNavigate).toHaveBeenCalledTimes(2);
  });
});
