// Cross-locale drift gate for the Rules content module. The four locales live
// outside the parity-tested i18n JSON, so this test guarantees they never drift
// in structure and that no translated string is empty.

import { describe, expect, it } from "vitest";

import { getRulesContent, RULES_CONTENT } from "./rulesContent";
import type { RuleBlock, RulesContent, RulesLang } from "./types";

const LANGS: RulesLang[] = ["en", "mk", "hr", "sr"];
const reference = RULES_CONTENT.en;

function blockShape(b: RuleBlock): string {
  if (b.kind === "steps") return `steps:${b.items.length}`;
  // Tier tokens are structure, not translation — the shape pins their order.
  if (b.kind === "tiers") return `tiers:${b.items.map((i) => i.tier).join(",")}`;
  return b.kind;
}

// Walk every leaf string in a content object and collect empties.
function emptyStringPaths(value: unknown, prefix = ""): string[] {
  if (typeof value === "string") {
    return value.trim().length === 0 ? [prefix] : [];
  }
  if (Array.isArray(value)) {
    return value.flatMap((v, i) => emptyStringPaths(v, `${prefix}[${i}]`));
  }
  if (value && typeof value === "object") {
    return Object.entries(value).flatMap(([k, v]) =>
      // `note` is an optional card annotation — empty for most ranks by design.
      k === "note" ? [] : emptyStringPaths(v, prefix ? `${prefix}.${k}` : k),
    );
  }
  return [];
}

describe("rules content parity", () => {
  it("exposes all four locales", () => {
    for (const lang of LANGS) {
      expect(RULES_CONTENT[lang], `missing locale ${lang}`).toBeDefined();
    }
  });

  it("has identical section ids and order across locales", () => {
    const refIds = reference.sections.map((s) => s.id);
    for (const lang of LANGS) {
      expect(
        RULES_CONTENT[lang].sections.map((s) => s.id),
        `${lang} section ids`,
      ).toEqual(refIds);
    }
  });

  it("has identical block shapes per section across locales", () => {
    reference.sections.forEach((section, i) => {
      const refShapes = section.blocks.map(blockShape);
      for (const lang of LANGS) {
        const blocks = RULES_CONTENT[lang].sections[i]?.blocks.map(blockShape) ?? [];
        expect(blocks, `${lang} block shapes for section "${section.id}"`).toEqual(refShapes);
      }
    });
  });

  it("has identical declaration ids and shared numbers across locales", () => {
    const refDecls = reference.declarations.map((d) => ({
      id: d.id,
      pts: d.pts,
      tier: d.tier,
      kind: d.kind,
    }));
    for (const lang of LANGS) {
      const decls = RULES_CONTENT[lang].declarations.map((d) => ({
        id: d.id,
        pts: d.pts,
        tier: d.tier,
        kind: d.kind,
      }));
      expect(decls, `${lang} declarations`).toEqual(refDecls);
    }
  });

  it("has identical card ladders (rank/pts/strength) across locales", () => {
    const numeric = (c: RulesContent) => ({
      trump: c.cardsTrump.map((r) => ({ rank: r.rank, pts: r.pts, strength: r.strength })),
      plain: c.cardsPlain.map((r) => ({ rank: r.rank, pts: r.pts, strength: r.strength })),
    });
    const ref = numeric(reference);
    for (const lang of LANGS) {
      expect(numeric(RULES_CONTENT[lang]), `${lang} card ladders`).toEqual(ref);
    }
  });

  it("has four facts in every locale", () => {
    for (const lang of LANGS) {
      expect(RULES_CONTENT[lang].ui.facts, `${lang} facts`).toHaveLength(4);
    }
  });

  it("has no empty strings in any locale", () => {
    for (const lang of LANGS) {
      const empties = emptyStringPaths(RULES_CONTENT[lang]);
      expect(empties, `${lang} has empty strings at: ${empties.join(", ")}`).toEqual([]);
    }
  });

  it("falls back to English for unknown languages", () => {
    expect(getRulesContent("xx")).toBe(RULES_CONTENT.en);
    expect(getRulesContent(undefined)).toBe(RULES_CONTENT.en);
    expect(getRulesContent("sr-Latn")).toBe(RULES_CONTENT.sr);
  });
});
