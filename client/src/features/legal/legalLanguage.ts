import type { LegalDocumentLanguage } from "@/shared/api/legal";
import { normalizeLanguage } from "@/shared/i18n/i18n";

export type LegalLanguage = LegalDocumentLanguage;

export const LEGAL_LANGUAGES: readonly LegalLanguage[] = ["en", "mk"];

const LANGUAGES_WITH_NOTE = new Set(["sr", "hr"]);

/** Picks the legal text for a UI language: Macedonian for mk, English for everything else. */
export function legalLanguageFor(uiLanguage: string | null | undefined): LegalLanguage {
  return normalizeLanguage(uiLanguage) === "mk" ? "mk" : "en";
}

/** True for UI languages that have no legal text of their own and get a note saying which versions exist. */
export function needsLanguageNote(uiLanguage: string | null | undefined): boolean {
  const lang = normalizeLanguage(uiLanguage);
  return lang !== null && LANGUAGES_WITH_NOTE.has(lang);
}
