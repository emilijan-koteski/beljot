import "@/shared/i18n/i18n";

import i18n from "i18next";
import { beforeAll, describe, expect, it } from "vitest";

import { modeLabel, modeOptionLabel, variantLabel } from "./roomLabels";

const t = ((key: string, options?: Record<string, unknown>) => i18n.t(key, options)) as Parameters<
  typeof variantLabel
>[0];

const LOCALES = ["en", "mk", "hr", "sr"] as const;

describe("roomLabels — the single variant/mode label resolver", () => {
  beforeAll(async () => {
    await i18n.changeLanguage("en");
  });

  it.each(["bitola", "croatia"] as const)("resolves the %s variant to real copy", (v) => {
    const label = variantLabel(t, v);
    expect(label).not.toBe("");
    expect(label).not.toContain("lobby.card");
  });

  it.each(["1001", "701", "501"] as const)("resolves the %s mode in both forms", (m) => {
    expect(modeLabel(t, m)).toContain(m);
    expect(modeLabel(t, m)).not.toContain("matchMode");
    expect(modeOptionLabel(t, m)).toContain(m);
    expect(modeOptionLabel(t, m)).not.toContain("matchMode");
  });

  it.each([
    ["en", "701 pts", "701 points"],
    ["sr", "701 bod", "701 bod"],
    ["hr", "701 bod", "701 bod"],
    ["mk", "701 поен", "701 поен"],
  ] as const)("resolves the 701 mode to its own %s copy", async (loc, compact, spelled) => {
    await i18n.changeLanguage(loc);
    const labels = [modeLabel(t, "701"), modeOptionLabel(t, "701")];
    await i18n.changeLanguage("en");
    expect(labels).toEqual([compact, spelled]);
  });

  it("keeps the compact and spelled-out mode forms distinct", () => {
    expect(modeLabel(t, "1001")).not.toBe(modeOptionLabel(t, "1001"));
  });

  describe("unknown values fall back readably", () => {
    it("title-cases an unknown variant rather than echoing a key", () => {
      expect(variantLabel(t, "zagreb")).toBe("Zagreb");
    });

    it("renders an unknown numeric mode through i18n, not hardcoded English (D138)", async () => {
      const rendered: string[] = [];
      for (const loc of LOCALES) {
        await i18n.changeLanguage(loc);
        const out = modeLabel(t, "751");
        expect(out).toContain("751");
        expect(out).not.toContain("matchModeGeneric");
        rendered.push(out);
      }
      await i18n.changeLanguage("en");
      expect(new Set(rendered).size).toBeGreaterThan(1);
    });

    it("passes a non-numeric mode through, and empty values render an em dash", () => {
      expect(modeLabel(t, "blitz")).toBe("blitz");
      expect(modeLabel(t, "")).toBe("—");
      expect(variantLabel(t, "")).toBe("—");
    });
  });
});
