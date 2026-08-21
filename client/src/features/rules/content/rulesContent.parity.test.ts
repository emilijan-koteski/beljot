// Cross-locale drift gate for the Rules content module. The four locales live
// outside the parity-tested i18n JSON, so this test guarantees they never drift
// in structure and that no translated string is empty.

import { describe, expect, it } from "vitest";

import { type Variant, VARIANTS } from "@/shared/types/matchTypes";

import { getRulesContent, RULES_CONTENT } from "./rulesContent";
import type { RuleBlock, RulesContent, RulesLang, StepItem, VariantScope } from "./types";
import { visibleFor } from "./visibility";

const LANGS: RulesLang[] = ["en", "mk", "hr", "sr"];
const reference = RULES_CONTENT.en;

// The block kinds whose renderers actually paint a difference marker, in BOTH
// the light `RuleBlock` switch and the dark `DarkBlock` one. `cards` and `melds`
// take no marker in either (they render a grid, with no text to hang it on), and
// `tiers` renders in the light theme only — `DarkBlock` has no `tiers` case at
// all, a pre-existing gap filed as deferred work. `VariantScope` sits on every
// member of the union, so nothing in the type system stops a scoped `melds`
// block; this list is what makes it a test failure instead of a note that
// silently disappears in one theme.
const MARKER_CAPABLE: RuleBlock["kind"][] = ["p", "rule", "steps", "note"];

// The variant scope and the presence of a difference marker are structure, not
// translation: a block scoped in one locale only, or one that lost its marker
// in translation, renders a different page per language. Folding both into the
// shape string is what makes that a test failure instead of silent drift.
function scopeShape(s: VariantScope): string {
  return `${s.variant ?? "all"}/${s.otherVariantNote ? "marked" : "plain"}`;
}

function blockShape(b: RuleBlock): string {
  // Step items carry their own scope, so the per-item scopes are part of the
  // block's shape: a locale that scoped the same six steps differently would
  // render a different page under the same tab.
  if (b.kind === "steps")
    return `steps:${b.items.length}:${scopeShape(b)}:[${b.items.map(scopeShape).join("|")}]`;
  // Tier tokens are structure, not translation — the shape pins their order.
  if (b.kind === "tiers") return `tiers:${b.items.map((i) => i.tier).join(",")}:${scopeShape(b)}`;
  return `${b.kind}:${scopeShape(b)}`;
}

/** Every scoped block in a section, grouped by the variant it is scoped to. */
function scopedVariants(blocks: RuleBlock[]): Set<Variant> {
  return new Set(blocks.flatMap((b) => (b.variant ? [b.variant] : [])));
}

/** Every scoped step item across a section's `steps` blocks. */
function stepItems(blocks: RuleBlock[]): { blockIndex: number; index: number; item: StepItem }[] {
  return blocks.flatMap((b, blockIndex) =>
    b.kind === "steps" ? b.items.map((item, index) => ({ blockIndex, index, item })) : [],
  );
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

  it("renders a non-empty block list for both variants in every section", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        for (const variant of VARIANTS) {
          expect(
            visibleFor(section.blocks, variant).length,
            `${lang} section "${section.id}" renders nothing under ${variant}`,
          ).toBeGreaterThan(0);
        }
      }
    }
  });

  it("gives every variant-scoped block a difference marker", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        section.blocks.forEach((b, i) => {
          if (!b.variant) return;
          expect(
            b.otherVariantNote,
            `${lang} section "${section.id}" block ${i} is scoped to ${b.variant} with no otherVariantNote`,
          ).toBeTruthy();
        });
      }
    }
  });

  it("scopes a block to a real variant only", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        for (const b of section.blocks) {
          if (b.variant) expect(VARIANTS as readonly string[]).toContain(b.variant);
        }
      }
    }
  });

  it("puts a difference note only on kinds whose renderers show one", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        section.blocks.forEach((b, i) => {
          if (!b.otherVariantNote) return;
          expect(
            MARKER_CAPABLE,
            `${lang} section "${section.id}" block ${i} is a "${b.kind}" carrying an otherVariantNote, which its renderer would drop`,
          ).toContain(b.kind);
        });
      }
    }
  });

  it("never carries a difference note on an unscoped block", () => {
    // An unscoped block renders under BOTH tabs, so a note saying "the other
    // variant does X" would be wrong on one of them.
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        section.blocks.forEach((b, i) => {
          if (!b.otherVariantNote) return;
          expect(
            b.variant,
            `${lang} section "${section.id}" block ${i} has an otherVariantNote but no variant scope`,
          ).toBeTruthy();
        });
      }
    }
  });

  it("covers both variants wherever a section scopes anything", () => {
    // A section that scopes a block for one variant and gives the other no
    // counterpart is a content hole: that tab is simply missing the rule.
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        const scoped = scopedVariants(section.blocks);
        if (scoped.size === 0) continue;
        for (const variant of VARIANTS) {
          expect(
            [...scoped],
            `${lang} section "${section.id}" scopes blocks but has none for ${variant}`,
          ).toContain(variant);
        }
      }
    }
  });

  // ── Item-level scoping ───────────────────────────────────────────────────
  //
  // `basics` holds ONE steps block whose individual items are scoped, so the
  // block-level gates above see nothing there. These are the equivalents one
  // level down.

  it("gives every variant-scoped step item a difference marker", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        for (const { blockIndex, index, item } of stepItems(section.blocks)) {
          if (!item.variant) continue;
          expect(
            item.otherVariantNote,
            `${lang} "${section.id}" block ${blockIndex} step ${index} ("${item.t}") is scoped to ${item.variant} with no otherVariantNote`,
          ).toBeTruthy();
        }
      }
    }
  });

  it("scopes a step item to a real variant only", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        for (const { item } of stepItems(section.blocks)) {
          if (item.variant) expect(VARIANTS as readonly string[]).toContain(item.variant);
        }
      }
    }
  });

  it("never carries a difference note on an unscoped step item", () => {
    // A shared item renders under BOTH tabs, so "the other variant does X" is
    // wrong on one of them.
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        for (const { blockIndex, index, item } of stepItems(section.blocks)) {
          if (!item.otherVariantNote) continue;
          expect(
            item.variant,
            `${lang} "${section.id}" block ${blockIndex} step ${index} ("${item.t}") has an otherVariantNote but no variant scope`,
          ).toBeTruthy();
        }
      }
    }
  });

  it("covers both variants wherever a steps block scopes any item", () => {
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        section.blocks.forEach((block, blockIndex) => {
          if (block.kind !== "steps") return;
          const scoped = new Set(block.items.flatMap((it) => (it.variant ? [it.variant] : [])));
          if (scoped.size === 0) return;
          for (const variant of VARIANTS) {
            expect(
              [...scoped],
              `${lang} "${section.id}" block ${blockIndex} scopes step items but has none for ${variant}`,
            ).toContain(variant);
          }
        });
      }
    }
  });

  it("shows both variants the same number of steps in every steps block", () => {
    // "Almost identical" is the product requirement, not a nice-to-have: one tab
    // silently short a step is the failure this catches. It also pins the
    // gapless 01..06 the renderers derive from the filtered list.
    for (const lang of LANGS) {
      for (const section of RULES_CONTENT[lang].sections) {
        section.blocks.forEach((block, blockIndex) => {
          if (block.kind !== "steps") return;
          const counts = VARIANTS.map((v) => visibleFor(block.items, v).length);
          expect(
            new Set(counts).size,
            `${lang} "${section.id}" block ${blockIndex} renders ${counts.join(" vs ")} steps across ${VARIANTS.join("/")}`,
          ).toBe(1);
        });
      }
    }
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
