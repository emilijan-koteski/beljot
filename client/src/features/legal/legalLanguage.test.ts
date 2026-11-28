import { describe, expect, it } from "vitest";

import { legalLanguageFor, needsLanguageNote } from "./legalLanguage";

describe("legalLanguageFor", () => {
  it("returns Macedonian for the mk UI language", () => {
    expect(legalLanguageFor("mk")).toBe("mk");
    expect(legalLanguageFor("mk-MK")).toBe("mk");
  });

  it.each(["en", "sr", "hr", "de", "", null, undefined])("returns English for %s", (lang) => {
    expect(legalLanguageFor(lang)).toBe("en");
  });
});

describe("needsLanguageNote", () => {
  it.each(["sr", "hr", "hr-HR"])("is true for %s", (lang) => {
    expect(needsLanguageNote(lang)).toBe(true);
  });

  it.each(["en", "mk", "de", null])("is false for %s", (lang) => {
    expect(needsLanguageNote(lang)).toBe(false);
  });
});
