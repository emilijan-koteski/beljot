import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import { applyDayjsLocale } from "@/shared/lib/relativeTime";

import en from "./en.json";
import hr from "./hr.json";
import mk from "./mk.json";
import sr from "./sr.json";

const SUPPORTED_LANGUAGES = ["en", "hr", "sr", "mk"] as const;
type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

export const LANGUAGE_STORAGE_KEY = "beljot.lang";

function isSupported(code: string | null | undefined): code is SupportedLanguage {
  return !!code && (SUPPORTED_LANGUAGES as readonly string[]).includes(code);
}

/**
 * Normalizes any runtime language value to a supported short code, or null.
 * Region subtags such as "en-US" are stripped.
 */
export function normalizeLanguage(lng: string | null | undefined): SupportedLanguage | null {
  if (!lng) return null;
  const short = lng.toLowerCase().split("-")[0];
  return isSupported(short) ? short : null;
}

function readStoredLanguage(): SupportedLanguage | null {
  if (typeof window === "undefined") return null;
  try {
    const stored = window.localStorage.getItem(LANGUAGE_STORAGE_KEY);
    return isSupported(stored) ? stored : null;
  } catch {
    return null;
  }
}

function readBrowserLanguages(): readonly string[] {
  if (typeof navigator === "undefined") return [];
  return navigator.languages?.length ? navigator.languages : [navigator.language];
}

/** Picks the stored choice, else the first supported browser language, else English. */
export function pickInitialLanguage(
  stored: SupportedLanguage | null,
  browserLanguages: readonly string[],
): SupportedLanguage {
  if (stored) return stored;
  for (const lng of browserLanguages) {
    const supported = normalizeLanguage(lng);
    if (supported) return supported;
  }
  return "en";
}

const initialLng = pickInitialLanguage(readStoredLanguage(), readBrowserLanguages());

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    sr: { translation: sr },
    mk: { translation: mk },
    hr: { translation: hr },
  },
  lng: initialLng,
  fallbackLng: "en",
  interpolation: {
    escapeValue: false,
  },
});

/** Sets the page's lang attribute so screen readers and search engines read the right language. */
function applyDocumentLanguage(lng: SupportedLanguage) {
  if (typeof document === "undefined") return;
  document.documentElement.lang = lng;
}

applyDayjsLocale(initialLng);
applyDocumentLanguage(initialLng);

/** Applies a language switch to dayjs and the document, and persists it to localStorage. */
function syncLanguage(lng: string) {
  const normalized = normalizeLanguage(lng);
  if (!normalized) return;
  applyDayjsLocale(normalized);
  applyDocumentLanguage(normalized);
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(LANGUAGE_STORAGE_KEY, normalized);
  } catch {
    return;
  }
}

i18n.on("languageChanged", syncLanguage);

export { i18n };
