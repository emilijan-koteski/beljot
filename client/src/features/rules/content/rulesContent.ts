import { en } from "./en";
import { hr } from "./hr";
import { mk } from "./mk";
import { type CardBase, DECLARATIONS_BASE, PLAIN_ROWS, TRUMP_ROWS } from "./shared";
import { sr } from "./sr";
import type { CardRow, Declaration, RulesContent, RulesLang, RulesLangData } from "./types";

function buildCards(
  rows: CardBase[],
  names: RulesLangData["cardNames"],
  notes: Partial<Record<CardRow["rank"], string>>,
): CardRow[] {
  return rows.map((r) => ({
    rank: r.rank,
    name: names[r.rank],
    pts: r.pts,
    strength: r.strength,
    note: notes[r.rank] ?? "",
  }));
}

function buildDeclarations(strings: RulesLangData["declarations"]): Declaration[] {
  return DECLARATIONS_BASE.map((d) => {
    const s = strings[d.id];
    if (!s) throw new Error(`Rules content: missing declaration strings for "${d.id}"`);
    return { ...d, ...s };
  });
}

function build(data: RulesLangData): RulesContent {
  return {
    cardsTrump: buildCards(TRUMP_ROWS, data.cardNames, data.trumpNotes),
    cardsPlain: buildCards(PLAIN_ROWS, data.cardNames, data.plainNotes),
    declarations: buildDeclarations(data.declarations),
    sections: data.sections,
    ui: data.ui,
  };
}

export const RULES_CONTENT: Record<RulesLang, RulesContent> = {
  en: build(en),
  mk: build(mk),
  hr: build(hr),
  sr: build(sr),
};

/**
 * Resolve rules content for the active i18n language. Accepts plain ("mk") or
 * region-tagged ("sr-Latn") codes and falls back to English for anything else.
 */
export function getRulesContent(lang: string | undefined): RulesContent {
  if (!lang) return RULES_CONTENT.en;
  const base = lang.split("-")[0] as RulesLang;
  return RULES_CONTENT[base] ?? RULES_CONTENT.en;
}
