import { describe, expect, it } from "vitest";

import {
  BMC_MARK_SRC,
  BMC_STICKER_SRC,
  BMC_URL,
  supportMarkSrc,
} from "@/shared/components/support/supportLinks";

describe("supportLinks", () => {
  it("links the canonical page URL, with no www redirect hop", () => {
    expect(BMC_URL).toBe("https://buymeacoffee.com/emilijan");
  });
});

describe("supportMarkSrc", () => {
  const STICKER = "/support/coffee.gif";

  it("uses the animated sticker when one is configured and motion is allowed", () => {
    expect(supportMarkSrc(false, STICKER)).toBe(STICKER);
  });

  it("falls back to the static mark under prefers-reduced-motion", () => {
    expect(supportMarkSrc(true, STICKER)).toBe(BMC_MARK_SRC);
  });

  it("falls back to the static mark when no sticker is configured", () => {
    expect(supportMarkSrc(false, null)).toBe(BMC_MARK_SRC);
    expect(supportMarkSrc(true, null)).toBe(BMC_MARK_SRC);
  });

  it("animates by default and stills under reduced motion, as shipped", () => {
    expect(BMC_STICKER_SRC).not.toBeNull();
    expect(supportMarkSrc(false)).toBe(BMC_STICKER_SRC);
    expect(supportMarkSrc(true)).toBe(BMC_MARK_SRC);
  });
});
