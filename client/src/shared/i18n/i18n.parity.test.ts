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
