import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  recordServerNow,
  remainingMsUntil,
  resetClockSyncForTest,
  serverClockOffsetMs,
  serverNowMs,
} from "./clockSync";

describe("clockSync", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    resetClockSyncForTest();
  });

  afterEach(() => {
    resetClockSyncForTest();
    vi.useRealTimers();
  });

  it("returns zero offset before any sample arrives", () => {
    expect(serverClockOffsetMs()).toBe(0);
    expect(serverNowMs()).toBe(100_000);
  });

  it("estimates the offset from a serverNow stamp", () => {
    recordServerNow(new Date(102_000).toISOString());
    expect(serverClockOffsetMs()).toBe(2_000);
    expect(serverNowMs()).toBe(102_000);
  });

  it("keeps the max sample — the one with the least latency error", () => {
    recordServerNow(new Date(101_700).toISOString());
    recordServerNow(new Date(101_950).toISOString());
    recordServerNow(new Date(101_850).toISOString());
    expect(serverClockOffsetMs()).toBe(1_950);
  });

  it("forgets samples older than the rolling window", () => {
    recordServerNow(new Date(105_000).toISOString());
    for (let i = 0; i < 10; i++) {
      recordServerNow(new Date(101_000 + i).toISOString());
    }
    expect(serverClockOffsetMs()).toBe(1_009);
  });

  it("expires stale samples by age, not only by count", () => {
    recordServerNow(new Date(105_000).toISOString());
    vi.setSystemTime(280_000);
    recordServerNow(new Date(281_000).toISOString());
    expect(serverClockOffsetMs()).toBe(1_000);
  });

  it("ignores unparseable stamps", () => {
    recordServerNow(new Date(101_000).toISOString());
    recordServerNow("not-a-date");
    expect(serverClockOffsetMs()).toBe(1_000);
  });

  it("computes remaining time against the corrected clock", () => {
    recordServerNow(new Date(102_000).toISOString());
    expect(remainingMsUntil(new Date(105_000).toISOString())).toBe(3_000);
    expect(remainingMsUntil(new Date(90_000).toISOString())).toBe(0);
    expect(remainingMsUntil("garbage")).toBe(0);
  });
});
