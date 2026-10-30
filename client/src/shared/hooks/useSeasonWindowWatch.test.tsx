import "@/shared/i18n/i18n";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render } from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSeasonWindowWatch } from "@/shared/hooks/useSeasonWindowWatch";
import { i18n } from "@/shared/i18n/i18n";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

const NOW = Date.parse("2026-08-27T12:00:00Z");

const baseSeason: CurrentSeasonResponse = {
  seasonName: "2026 Q3",
  endsAt: "2026-10-01T00:00:00Z",
  sp: 680,
  rankTier: "gold",
  rankDivision: 2,
  spIntoDivision: 13,
  spForNextDivision: 67,
  gamesPlayed: 31,
  gamesWon: 15,
  gamesCompleted: 29,
};

/** Renders nothing; mounts the hook so its effects can be tested in isolation. */
function Probe({ season }: { season: CurrentSeasonResponse | undefined }) {
  useSeasonWindowWatch(season);
  return null;
}

function renderWatch(
  season: CurrentSeasonResponse | undefined,
  qc: QueryClient = new QueryClient(),
) {
  return render(
    <QueryClientProvider client={qc}>
      <Probe season={season} />
    </QueryClientProvider>,
  );
}

describe("useSeasonWindowWatch", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    vi.mocked(toast.success).mockClear();
  });

  afterEach(async () => {
    cleanup();
    vi.useRealTimers();
    await i18n.changeLanguage("en");
  });

  it("does nothing at all while the season query is in flight", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");
    renderWatch(undefined, qc);
    expect(spy).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("invalidates season.current, every leaderboard key and the seasons list when the window has ended", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);

    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "current"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "leaderboard"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["season", "list"] });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  it("does not invalidate while the window is still open", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch(baseSeason, qc);

    expect(spy).not.toHaveBeenCalled();
  });

  it("invalidates once per boundary, not on every 30s tick", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    act(() => {
      vi.advanceTimersByTime(90_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  it("fires the invalidation when the countdown crosses zero on a later tick", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch({ ...baseSeason, endsAt: "2026-08-27T12:00:15Z" }, qc);
    expect(spy).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);
  });

  it("re-arms for the next quarter after the refetched season arrives", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    const view = renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    view.rerender(
      <QueryClientProvider client={qc}>
        <Probe season={{ ...baseSeason, seasonName: "2026 Q4", endsAt: "2026-08-27T12:00:15Z" }} />
      </QueryClientProvider>,
    );
    expect(spy).toHaveBeenCalledTimes(3);

    act(() => {
      vi.advanceTimersByTime(30_000);
    });
    expect(spy).toHaveBeenCalledTimes(6);
  });

  it("retries the invalidation when the refetch returns the same window", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries");

    renderWatch({ ...baseSeason, endsAt: "2026-08-27T11:00:00Z" }, qc);
    expect(spy).toHaveBeenCalledTimes(3);

    act(() => {
      vi.advanceTimersByTime(4 * 60_000);
    });
    expect(spy).toHaveBeenCalledTimes(3);

    act(() => {
      vi.advanceTimersByTime(2 * 60_000);
    });
    expect(spy).toHaveBeenCalledTimes(6);
  });

  it("fires one transition toast when the observed season changes, and none on first load", () => {
    const qc = new QueryClient();
    const view = renderWatch(baseSeason, qc);
    expect(toast.success).not.toHaveBeenCalled();

    const next: CurrentSeasonResponse = {
      ...baseSeason,
      seasonName: "2026 Q4",
      endsAt: "2027-01-01T00:00:00Z",
      sp: 0,
      rankTier: "iron",
      rankDivision: 1,
      spIntoDivision: 0,
      spForNextDivision: 50,
    };
    view.rerender(
      <QueryClientProvider client={qc}>
        <Probe season={next} />
      </QueryClientProvider>,
    );
    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(vi.mocked(toast.success).mock.calls[0]![0]).toBe(
      i18n.t("season.banner.newSeason", { season: "2026 Q4" }),
    );

    view.rerender(
      <QueryClientProvider client={qc}>
        <Probe season={{ ...next, sp: 120 }} />
      </QueryClientProvider>,
    );
    expect(toast.success).toHaveBeenCalledTimes(1);
  });
});
