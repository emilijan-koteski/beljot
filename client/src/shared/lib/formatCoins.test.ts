import { describe, expect, it } from "vitest";

import { formatCoins } from "./formatCoins";

describe("formatCoins", () => {
  it("groups thousands the same way as toLocaleString", () => {
    expect(formatCoins(6000)).toBe((6000).toLocaleString());
    expect(formatCoins(12345)).toBe((12345).toLocaleString());
  });

  it("leaves sub-thousand values ungrouped", () => {
    expect(formatCoins(0)).toBe("0");
    expect(formatCoins(500)).toBe("500");
  });

  it("formats negative amounts (e.g. a settlement loss)", () => {
    expect(formatCoins(-500)).toBe((-500).toLocaleString());
  });
});
