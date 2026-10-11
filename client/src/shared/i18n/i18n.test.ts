import { afterEach, beforeEach, describe, expect, it } from "vitest";

import en from "./en.json";
import hr from "./hr.json";
import { i18n, LANGUAGE_STORAGE_KEY, normalizeLanguage } from "./i18n";
import mk from "./mk.json";
import sr from "./sr.json";

function flattenKeys(obj: unknown, prefix = ""): string[] {
  if (obj === null || typeof obj !== "object") return [prefix];
  const out: string[] = [];
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v !== null && typeof v === "object") {
      out.push(...flattenKeys(v, path));
    } else {
      out.push(path);
    }
  }
  return out;
}

describe("i18n", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
    window.localStorage.clear();
  });

  it("returns English translation by default", () => {
    expect(i18n.t("common.appName")).toBe("Beljot");
  });

  it("returns Serbian when language is sr", async () => {
    await i18n.changeLanguage("sr");
    expect(i18n.t("common.appName")).toBe("Beljot");
    expect(i18n.t("nav.play")).toBe("Igraj");
    expect(i18n.t("chat.title")).toBe("Lobi chat");
  });

  it("returns Macedonian when language is mk", async () => {
    await i18n.changeLanguage("mk");
    expect(i18n.t("common.appName")).toBe("Beljot");
    expect(i18n.t("nav.play")).toBe("Играј");
  });

  it("returns Croatian when language is hr", async () => {
    await i18n.changeLanguage("hr");
    expect(i18n.t("common.appName")).toBe("Beljot");
    expect(i18n.t("nav.play")).toBe("Igraj");
    expect(i18n.t("chat.title")).toBe("Chat lobija");
  });

  describe("localStorage persistence", () => {
    beforeEach(() => {
      window.localStorage.clear();
    });

    it("writes the supported language code to localStorage on changeLanguage", async () => {
      await i18n.changeLanguage("hr");
      expect(window.localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("hr");
    });

    it("overwrites the localStorage value when the language changes again", async () => {
      await i18n.changeLanguage("sr");
      expect(window.localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("sr");

      await i18n.changeLanguage("mk");
      expect(window.localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("mk");
    });

    it("uses the documented storage key beljot.lang", () => {
      expect(LANGUAGE_STORAGE_KEY).toBe("beljot.lang");
    });

    it("normalizes region-tagged codes before writing to localStorage", async () => {
      await i18n.changeLanguage("en-US");
      expect(window.localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("en");
    });

    it("does not write unsupported codes to localStorage", async () => {
      window.localStorage.setItem(LANGUAGE_STORAGE_KEY, "hr");
      await i18n.changeLanguage("fr");
      expect(window.localStorage.getItem(LANGUAGE_STORAGE_KEY)).toBe("hr");
    });
  });

  describe("normalizeLanguage", () => {
    it("accepts the four supported short codes verbatim", () => {
      expect(normalizeLanguage("en")).toBe("en");
      expect(normalizeLanguage("hr")).toBe("hr");
      expect(normalizeLanguage("sr")).toBe("sr");
      expect(normalizeLanguage("mk")).toBe("mk");
    });

    it("strips region subtags down to the short code", () => {
      expect(normalizeLanguage("en-US")).toBe("en");
      expect(normalizeLanguage("hr-HR")).toBe("hr");
    });

    it("is case-insensitive on the language portion", () => {
      expect(normalizeLanguage("EN")).toBe("en");
      expect(normalizeLanguage("Mk")).toBe("mk");
    });

    it("rejects junk and unsupported codes (used as the boot-time allowlist guard)", () => {
      expect(normalizeLanguage("xx")).toBeNull();
      expect(normalizeLanguage("fr")).toBeNull();
      expect(normalizeLanguage("")).toBeNull();
      expect(normalizeLanguage(null)).toBeNull();
      expect(normalizeLanguage(undefined)).toBeNull();
    });
  });

  it("en.json, sr.json, mk.json, hr.json have identical key sets", () => {
    const enKeys = flattenKeys(en).sort();
    for (const [name, locale] of [
      ["sr", sr],
      ["mk", mk],
      ["hr", hr],
    ] as const) {
      const localeKeys = flattenKeys(locale).sort();
      const missing = enKeys.filter((k) => !localeKeys.includes(k));
      const extra = localeKeys.filter((k) => !enKeys.includes(k));
      expect(missing, `keys missing in ${name}.json: ${missing.join(", ")}`).toEqual([]);
      expect(extra, `keys missing in en.json (extra in ${name}.json): ${extra.join(", ")}`).toEqual(
        [],
      );
    }
  });
});
