import { act, render } from "@testing-library/react";
import { MemoryRouter, type NavigateFunction, useNavigate } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useVersionCheck } from "./useVersionCheck";

const CHECK_INTERVAL_MS = 5 * 60 * 1000;

// APP_VERSION is swapped per-test through the getter; reloadForNewVersion is
// the seam the hook uses instead of window.location.reload (non-configurable
// in jsdom).
const mocks = vi.hoisted(() => ({
  appVersion: "old-sha",
  reload: vi.fn(),
}));

vi.mock("@/shared/lib/appVersion", () => ({
  get APP_VERSION() {
    return mocks.appVersion;
  },
  reloadForNewVersion: mocks.reload,
}));

const fetchMock = vi.fn();

function mockServedVersion(version: string) {
  fetchMock.mockResolvedValue({
    ok: true,
    json: () => Promise.resolve({ version }),
  });
}

let navigateFn: NavigateFunction;

function Harness() {
  useVersionCheck();
  navigateFn = useNavigate();
  return null;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Harness />
    </MemoryRouter>,
  );
}

describe("useVersionCheck", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", fetchMock);
    mocks.appVersion = "old-sha";
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("reloads when the server reports a newer version", async () => {
    mockServedVersion("new-sha");
    renderAt("/lobby");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));

    expect(fetchMock).toHaveBeenCalledWith("/version.json", { cache: "no-store" });
    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("does nothing when versions match", async () => {
    mockServedVersion("old-sha");
    renderAt("/lobby");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));

    expect(fetchMock).toHaveBeenCalled();
    expect(mocks.reload).not.toHaveBeenCalled();
  });

  it("checks immediately when the tab becomes visible again", async () => {
    mockServedVersion("new-sha");
    renderAt("/lobby");

    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("defers the reload during an active match until leaving it", async () => {
    mockServedVersion("new-sha");
    renderAt("/match/room-1");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));
    expect(mocks.reload).not.toHaveBeenCalled();

    act(() => navigateFn("/lobby"));
    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("does not defer on /matchmaking routes despite the shared prefix", async () => {
    mockServedVersion("new-sha");
    renderAt("/matchmaking/room-1");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));

    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("reloads at most once per version (no reload loop on a sticky cache)", async () => {
    mockServedVersion("new-sha");
    renderAt("/lobby");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));
    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));

    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("stays inert in dev builds", async () => {
    mocks.appVersion = "dev";
    renderAt("/lobby");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));

    expect(fetchMock).not.toHaveBeenCalled();
    expect(mocks.reload).not.toHaveBeenCalled();
  });

  it("survives a failed fetch and retries on the next tick", async () => {
    fetchMock.mockRejectedValueOnce(new Error("offline"));
    renderAt("/lobby");

    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));
    expect(mocks.reload).not.toHaveBeenCalled();

    mockServedVersion("new-sha");
    await act(() => vi.advanceTimersByTimeAsync(CHECK_INTERVAL_MS));
    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });
});
