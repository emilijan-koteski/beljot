import { describe, expect, it } from "vitest";

import en from "./en.json";
import hr from "./hr.json";
import mk from "./mk.json";
import sr from "./sr.json";

/** Returns the dotted path of every leaf in a JSON object. */
function flattenKeys(obj: unknown, prefix = ""): string[] {
  if (obj === null || typeof obj !== "object") {
    return prefix === "" ? [] : [prefix];
  }
  const out: string[] = [];
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const path = prefix === "" ? k : `${prefix}.${k}`;
    if (v !== null && typeof v === "object") {
      out.push(...flattenKeys(v, path));
    } else {
      out.push(path);
    }
  }
  return out;
}

/** Returns a [path, value] pair for every leaf in a JSON object. */
function flattenEntries(obj: unknown, prefix = ""): Array<[string, unknown]> {
  if (obj === null || typeof obj !== "object") {
    return prefix === "" ? [] : [[prefix, obj]];
  }
  const out: Array<[string, unknown]> = [];
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    const path = prefix === "" ? k : `${prefix}.${k}`;
    if (v !== null && typeof v === "object") {
      out.push(...flattenEntries(v, path));
    } else {
      out.push([path, v]);
    }
  }
  return out;
}

const locales = [
  ["sr", sr],
  ["mk", mk],
  ["hr", hr],
] as const;

describe("i18n parity", () => {
  it("every key in en.json exists in every locale (and vice versa)", () => {
    const enKeys = flattenKeys(en).sort();

    for (const [name, locale] of locales) {
      const localeKeys = flattenKeys(locale).sort();

      const missingInLocale = enKeys.filter((k) => !localeKeys.includes(k));
      const extraInLocale = localeKeys.filter((k) => !enKeys.includes(k));

      expect(
        missingInLocale,
        `Keys present in en.json but missing in ${name}.json: ${missingInLocale.join(", ")}`,
      ).toEqual([]);
      expect(
        extraInLocale,
        `Keys present in ${name}.json but missing in en.json: ${extraInLocale.join(", ")}`,
      ).toEqual([]);
      expect(localeKeys, `${name}.json key parity vs en.json`).toEqual(enKeys);
    }
  });

  it("no leaf string is empty in any locale (AC-005)", () => {
    const allFiles = [["en", en], ...locales] as const;

    for (const [name, locale] of allFiles) {
      const empties = flattenEntries(locale)
        .filter(([, v]) => typeof v !== "string" || v.trim().length === 0)
        .map(([path]) => path);

      expect(empties, `${name}.json has empty/non-string leaves at: ${empties.join(", ")}`).toEqual(
        [],
      );
    }
  });
});

describe("card-deck suit name casing", () => {
  const LATIN_AND_CYRILLIC = { hr, sr, mk } as const;

  it.each(Object.entries(LATIN_AND_CYRILLIC))(
    "%s stores the Croatian deck's suit names lower case",
    (_locale, bundle) => {
      const suits: Record<string, string> = bundle.match.card.suit.croatian;
      for (const [suit, name] of Object.entries(suits)) {
        expect(name, `${suit} must not start capitalised`).toBe(
          name[0]!.toLowerCase() + name.slice(1),
        );
      }
    },
  );

  it("keeps English capitalised, since English does capitalise suit names", () => {
    const suits: Record<string, string> = en.match.card.suit.croatian;
    for (const [suit, name] of Object.entries(suits)) {
      expect(name[0], `${suit} should be capitalised in en`).toBe(name[0]!.toUpperCase());
    }
  });
});

describe("declaration phase copy", () => {
  const KEYS = ["noneTitle", "waitingOthers"] as const;

  function decl(locale: Record<string, unknown>): Record<string, string> {
    const match = locale.match as Record<string, unknown>;
    return match.declaration as Record<string, string>;
  }

  it.each([
    ["hr", hr],
    ["mk", mk],
    ["sr", sr],
  ])("%s translates every declaration-phase string away from the English", (name, locale) => {
    const localised = decl(locale as unknown as Record<string, unknown>);
    const english = decl(en as unknown as Record<string, unknown>);
    for (const key of KEYS) {
      expect(localised[key], `${name}.${key}`).toBeTruthy();
      expect(localised[key], `${name}.${key} is still the English string`).not.toBe(english[key]);
    }
  });

  it("keeps the Macedonian declaration-phase copy all-Cyrillic", () => {
    const localised = decl(mk as unknown as Record<string, unknown>);
    for (const key of KEYS) {
      const prose = (localised[key] ?? "").replace(/\{\{.*?\}\}/g, "");
      expect(prose, `mk.${key}`).not.toMatch(/[A-Za-z]/);
    }
  });

  it("keeps the answered/total readout in every locale", () => {
    for (const [name, locale] of [
      ["en", en],
      ["hr", hr],
      ["mk", mk],
      ["sr", sr],
    ] as const) {
      const value = decl(locale as unknown as Record<string, unknown>).waitingOthers;
      expect(value, `${name}.waitingOthers`).toContain("{{answered}}");
      expect(value, `${name}.waitingOthers`).toContain("/4");
    }
  });
});

describe("support copy", () => {
  const PROSE_KEYS = ["label", "note"] as const;
  const DIALOG_KEYS = ["title", "body", "cta", "qrToggle"] as const;

  type Support = {
    label: string;
    note: string;
    dialog: Record<string, string>;
  };

  function support(locale: unknown): Support {
    return (locale as { support: Support }).support;
  }

  it.each([
    ["hr", hr],
    ["mk", mk],
    ["sr", sr],
  ])("%s translates every support string away from the English", (name, locale) => {
    const localised = support(locale);
    const english = support(en);
    for (const key of PROSE_KEYS) {
      expect(localised[key], `${name}.support.${key}`).toBeTruthy();
      expect(localised[key], `${name}.support.${key} is still the English string`).not.toBe(
        english[key],
      );
    }
    for (const key of DIALOG_KEYS) {
      expect(
        localised.dialog[key],
        `${name}.support.dialog.${key} is still the English string`,
      ).not.toBe(english.dialog[key]);
    }
  });

  it("keeps the Macedonian support copy Cyrillic apart from the brand name", () => {
    const localised = support(mk);
    const entries: Array<[string, string]> = [
      ...PROSE_KEYS.map((k): [string, string] => [k, localised[k]]),
      ...DIALOG_KEYS.map((k): [string, string] => [`dialog.${k}`, localised.dialog[k] ?? ""]),
    ];
    for (const [key, value] of entries) {
      const prose = value.replace(/<\/?action>/g, "").replace(/Beljot(\.online)?/g, "");
      expect(prose, `mk.support.${key}`).not.toMatch(/[A-Za-z]/);
    }
  });

  it("keeps the clickable slot intact in the inline note, in every locale", () => {
    for (const [name, locale] of [
      ["en", en],
      ["hr", hr],
      ["mk", mk],
      ["sr", sr],
    ] as const) {
      for (const key of ["note"] as const) {
        const value = support(locale)[key];
        expect(value, `${name}.support.${key} lost its <action> slot`).toMatch(
          /<action>.+<\/action>/,
        );
        expect(value, `${name}.support.${key} must not use the void <link> tag`).not.toContain(
          "<link>",
        );
      }
    }
  });
});
