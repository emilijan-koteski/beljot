// i18n parity test. The existing `i18n.test.ts` already runs an equivalent
// assertion bundled with other i18n smoke checks; this file isolates the
// parity gate per the spec (AC-007) so a single dedicated failure points
// directly at translation drift without unrelated i18n behaviour mixed in.
//
// Approach: deep-flatten en.json (the canonical source) and every supported
// locale into dotted key paths (e.g. "lobby.roomLobby.startMatch", "team.us")
// and assert every locale's set is identical to en.json's. Sorted arrays
// make the failure diff readable in CI logs. Also asserts every leaf is a
// non-empty string so a stray `"key": ""` cannot pass parity (AC-005).

import { describe, expect, it } from "vitest";

import en from "./en.json";
import hr from "./hr.json";
import mk from "./mk.json";
import sr from "./sr.json";

// flattenKeys walks an arbitrary JSON object and returns the dotted path of
// every leaf. Arrays inside translation files would be flattened too, but
// the project's i18n files contain only nested string maps; the recursion
// handles either shape defensively.
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

// flattenEntries returns [path, leafValue] tuples — used to assert every
// translated leaf is a non-empty string, not just present in the key tree.
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
      // Belt-and-suspenders: explicit set equality so a failure shows the
      // sorted full key list in the CI log, not just the first diff.
      expect(localeKeys, `${name}.json key parity vs en.json`).toEqual(enKeys);
    }
  });

  it("no leaf string is empty in any locale (AC-005)", () => {
    const allFiles = [["en", en], ...locales] as const;

    for (const [name, locale] of allFiles) {
      // Use trim() so whitespace-only leaves (" ", "\t", "\n") also fail —
      // a bare " " is functionally an empty translation but slips through a
      // length === 0 check.
      const empties = flattenEntries(locale)
        .filter(([, v]) => typeof v !== "string" || v.trim().length === 0)
        .map(([path]) => path);

      expect(empties, `${name}.json has empty/non-string leaves at: ${empties.join(", ")}`).toEqual(
        [],
      );
    }
  });
});

// The Croatian deck's suits are ordinary common nouns (zelje / srce / bundeva /
// žir, лист / срце / ѕвонче / жир), unlike the French row's borrowed, capitalised
// card words (Pik / Herc / Karo / Tref). Most templates put the suit
// mid-sentence — "zvao bundeva za aduta", "го зеде ѕвонче за адут" — and none of
// these languages capitalise common nouns there, so a capital in the stored
// string is an orthography bug at every one of those call sites.
//
// Nothing else would catch it: the parity test above checks key sets and
// non-empty leaves, never wording. The one surface that needs a leading capital,
// TrumpIndicator's suit caption, takes it from CSS `capitalize` rather than from
// the stored value, so lowercase here is safe as well as correct.
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

// The declaration-phase copy replaced a per-seat "waiting for {name}" banner
// whose own test carried the only cross-locale assertion for these strings.
// Parity above proves the keys EXIST; this proves they were actually translated
// — an untranslated locale most often shows up as an English word left in place
// or a Latin string in the Cyrillic bundle.
describe("declaration phase copy", () => {
  const KEYS = ["noneTitle", "noneBody", "waitingOthers"] as const;

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
      // Strip the interpolation placeholder — {{answered}} is Latin by design.
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

// The support surfaces are the one place the app asks players for money, so
// their copy gets its own gate. Parity above proves the keys EXIST; this proves
// they were translated, that the Macedonian stayed Cyrillic, and that the
// clickable slot survived translation — a locale that drops <action> renders a
// sentence with nothing to click, and naming that slot <link> breaks silently
// (`link` is a real HTML void element, so Trans's parser closes it immediately
// and spills the label into the surrounding sentence).
describe("support copy", () => {
  const PROSE_KEYS = ["label", "note"] as const;
  const DIALOG_KEYS = ["eyebrow", "title", "body", "cta", "qrToggle"] as const;

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
      // Allowed Latin: the <action> markup slot and the brand name itself.
      // `qrAlt` is deliberately excluded from this list — it carries both the
      // "QR" abbreviation and the full "Buy Me a Coffee" brand name.
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
