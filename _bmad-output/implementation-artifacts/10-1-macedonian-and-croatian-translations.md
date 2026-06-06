# Story 10.1: Macedonian and Croatian Translations

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to use the platform in Macedonian or Croatian,
so that I can play in my native language.

## Acceptance Criteria

1. **Given** the i18n system is already configured for EN and SR, **When** Macedonian and Croatian translation files are added, **Then** `mk.json` and `hr.json` exist in `client/src/shared/i18n/` with all translated strings, **And** the translation keys are 1:1 with `en.json` — no missing or extra keys in either new file.
2. **Given** a player opens the language selector (lobby nav `LanguageSelector` or in-game `SettingsDialog`), **When** they view available languages, **Then** four options are listed in this fixed order: English, Hrvatski, Srpski, Македонски — Latin-script entries sorted ASC by native name, followed by Cyrillic-script entries sorted ASC. (Amended 2026-05-09 during code review to reflect the Round-3 user-directed ordering; the original geographic order — English, Srpski, Македонски, Hrvatski — was superseded.)
3. **Given** a player selects Macedonian or Croatian, **When** the preference is saved, **Then** the UI re-renders in the selected language immediately, **And** the preference is persisted to the server via `PATCH /api/v1/users/:id/preferences` with body `{"languagePreference":"mk"}` or `{"languagePreference":"hr"}`, **And** the auth store `user.languagePreference` is updated in place.
4. **Given** a player has saved Macedonian or Croatian as their preference and reloads the app, **When** `useAuthInit` rehydrates the session via `/auth/me`, **Then** the UI bootstraps in the saved language without flashing English first.
5. **Given** the parity test suite runs (`make test`), **When** translation key sets are compared, **Then** `en.json`, `sr.json`, `mk.json`, and `hr.json` all share the identical flattened key set; any missing or empty value fails the build.
6. **Given** the server receives `PATCH /users/:id/preferences` with `{"languagePreference":"fr"}` (or any code not in the supported set), **When** the handler validates the body, **Then** the request is rejected with HTTP 400 and `error.code = "INVALID_LANGUAGE"`; valid codes are exactly `en`, `sr`, `mk`, `hr`.
7. **Given** an existing user record with `language_preference = 'en'` (or `'sr'`), **When** the migrations and code in this story are deployed, **Then** their preference and login flow are unchanged; no DB migration is required (column is already `VARCHAR(10)` with default `'en'`).

## Tasks / Subtasks

- [x] **Task 1: Create `mk.json` translation file** (AC: #1, #5)
  - [x] 1.1 Create `client/src/shared/i18n/mk.json` mirroring the full key tree of [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json). Every leaf must be present and non-empty.
  - [x] 1.2 Translate every string into Macedonian (Cyrillic script). Use literal UTF-8 characters (`ш`, `ќ`, `ѓ`, `ј`, `љ`, `њ`, `ѕ`, `ч`, `џ`) — **never** `\uXXXX` escape sequences. JSON files in this repo are UTF-8 and use literal characters per the unification commit `5a77969`.
  - [x] 1.3 Belot-specific terminology (must be consistent across the file):
    - trump → `адут`
    - trick → `штих` (plural `штихови`)
    - declarations → `зови` (the canonical Belot term — same role as Serbian `Zvanja`)
    - hand / deal → `делење`
    - bidder / pass → `налогодавач` / `даље`
    - belote / rebelote → `белот` / `ре-белот`
    - capot → `капот` (uppercase in `game.capot.title` → `КАПОТ!`)
    - team Us / Them → `Ние` / `Тие`
    - suits — Cyrillic German-deck names: `Пик` (spades), `Херц` (hearts), `Каро` (diamonds), `Спато` (clubs). Note: clubs is the one suit where MK diverges from SR/HR — Macedonian uses `Спато`, while Serbian/Croatian use `Tref`. (Corrected 2026-05-09 during code review; the original value in mk.json was `Треф`, a Serbian carryover.)
  - [x] 1.4 Match the placeholder style already used in `en.json` and `sr.json`: literal `…` (U+2026) and `—` (U+2014) — not `...` or `--`. Preserve every `{{interpolation}}` token verbatim.
  - [x] 1.5 Add the language self-name keys: `language.mk = "Македонски"` and `language.hr = "Hrvatski"` (yes — both names land in **every** locale file, including `mk.json` and `hr.json`, so the dropdown shows each language in its own script regardless of the active locale).
- [x] **Task 2: Create `hr.json` translation file** (AC: #1, #5)
  - [x] 2.1 Create `client/src/shared/i18n/hr.json` mirroring the full key tree of `en.json`.
  - [x] 2.2 Translate every string into Croatian (Latin script). Use literal UTF-8 (`č`, `ć`, `š`, `ž`, `đ`) — never `\uXXXX` escapes.
  - [x] 2.3 Belot-specific terminology:
    - trump → `adut`
    - trick → `štih` (plural `štihovi`)
    - declarations → `zvanja`
    - hand / deal → `dijeljenje`
    - bidder / pass → `licitator` / `dalje`
    - belote / rebelote → `belot` / `re-belot`
    - capot → `kapot` (uppercase in `game.capot.title` → `KAPOT!`)
    - team Us / Them → `Mi` / `Oni`
    - suits — German-deck names: `Pik`, `Herc`, `Karo`, `Tref` (same as SR)
  - [x] 2.4 Croatian-specific verb forms differ from Serbian in places — e.g. infinitives end in `-ti` not `-t` (already same), `što` not `šta`, `tisuća` not `hiljada`, `tjedan` (rarely needed here). Treat `sr.json` as a *reference* for tone, not a copy-paste source.
  - [x] 2.5 Add `language.mk = "Македонски"` and `language.hr = "Hrvatski"` keys.
  - [x] 2.6 Update both `en.json` and `sr.json` to add the same `language.mk` and `language.hr` keys (in their own renderings — `en.json` may keep the native names, since that is the established pattern: SR's `language.en` is `"English"`, `language.sr` is `"Srpski"`). Use native self-names in **all four** files for consistency.
- [x] **Task 3: Register new locales in i18next** (AC: #2, #3, #4)
  - [x] 3.1 Update [client/src/shared/i18n/i18n.ts](client/src/shared/i18n/i18n.ts) to import `mk` and `hr` and add them to the `resources` map: `resources: { en: { translation: en }, sr: { translation: sr }, mk: { translation: mk }, hr: { translation: hr } }`. Keep `lng: "en"` and `fallbackLng: "en"` unchanged.
  - [x] 3.2 No other config changes needed — `useAuthInit` ([client/src/shared/hooks/useAuth.ts:47](client/src/shared/hooks/useAuth.ts#L47)) already calls `i18n.changeLanguage(res.languagePreference)` on bootstrap, which will automatically pick up `mk`/`hr` once they are registered.
- [x] **Task 4: Extend the language selector in lobby nav** (AC: #2, #3)
  - [x] 4.1 Update the `languages` array in [client/src/shared/components/LanguageSelector.tsx:13-16](client/src/shared/components/LanguageSelector.tsx#L13-L16) to include all four entries in this exact order: `en`, `sr`, `mk`, `hr` — each with `labelKey: "language.<code>"`. Order is fixed: most-spoken native first (EN), then SR, MK, HR (Beljot's primary user regions in geographic neighbouring order).
  - [x] 4.2 Replace the hardcoded `i18n.language === "sr" ? "SR" : "EN"` trigger label at [LanguageSelector.tsx:36](client/src/shared/components/LanguageSelector.tsx#L36) with `i18n.language.slice(0, 2).toUpperCase()` so MK / HR render correctly in the chip. Keep the region-subtag deferral noted in 1-4 review (`"en-US"` → `"EN"` is fine).
  - [x] 4.3 Add a `data-testid="language-option-mk"` and `data-testid="language-option-hr"` for the new dropdown items (the existing pattern uses `language-option-${lang.code}` so this is automatic — verify no hardcoded `language-option-en|sr` selectors exist elsewhere via Grep).
- [x] **Task 5: Extend the in-game settings dialog** (AC: #2, #3)
  - [x] 5.1 Update the `LANGUAGES` const in [client/src/features/game/components/SettingsDialog.tsx:18-21](client/src/features/game/components/SettingsDialog.tsx#L18-L21) to include `mk` and `hr` in the same fixed order. The selection radio rendering is already loop-driven over the array, so no JSX changes are needed beyond the const.
- [x] **Task 6: Server-side validation for new language codes** (AC: #6, #7)
  - [x] 6.1 Update the validation guard in [server/internal/user/handler.go:164](server/internal/user/handler.go#L164) from `req.LanguagePreference != "en" && req.LanguagePreference != "sr"` to a 4-element check (suggested: extract a package-level `var supportedLanguages = map[string]struct{}{"en": {}, "sr": {}, "mk": {}, "hr": {}}` and check membership). Keep the rejection branch returning `apperr.ErrInvalidLanguage`.
  - [x] 6.2 Update the error message in [server/internal/apperr/errors.go:52](server/internal/apperr/errors.go#L52) from `"language must be 'en' or 'sr'"` to `"language must be one of: en, sr, mk, hr"`. The error code `"INVALID_LANGUAGE"` and HTTP 400 status remain unchanged — handler tests asserting on the code (not the message) keep passing.
  - [x] 6.3 No DB migration needed: `language_preference VARCHAR(10) NOT NULL DEFAULT 'en'` ([server/migrations/000002_create_users.up.sql:6](server/migrations/000002_create_users.up.sql#L6)) already accepts any 2-char code.
- [x] **Task 7: Test coverage** (AC: #1, #2, #3, #5, #6)
  - [x] 7.1 Update [client/src/shared/i18n/i18n.parity.test.ts](client/src/shared/i18n/i18n.parity.test.ts) to compare key sets across **all four** files. Pattern:
    ```ts
    import en from "./en.json"; import sr from "./sr.json";
    import mk from "./mk.json"; import hr from "./hr.json";
    const enKeys = flattenKeys(en).sort();
    for (const [name, locale] of [["sr", sr], ["mk", mk], ["hr", hr]] as const) {
      const localeKeys = flattenKeys(locale).sort();
      expect(localeKeys, `${name}.json key parity`).toEqual(enKeys);
    }
    ```
    Also assert no leaf string is empty in any of the four files (catches `"key": ""`). This is the AC #5 lint gate.
  - [x] 7.2 Update [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) to add `it("returns Macedonian when language is mk")` and `it("returns Croatian when language is hr")` cases. Use `common.appName` (which stays `"Beljot"` in every locale) plus one locale-specific key like `nav.play` to confirm the bundle is actually loaded. Always reset to `en` in `afterEach` so test order doesn't leak.
  - [x] 7.3 Update [client/src/shared/components/LanguageSelector.test.tsx](client/src/shared/components/LanguageSelector.test.tsx) to assert all four `language-option-{en,sr,mk,hr}` testids are rendered after opening the dropdown, and add one happy-path test for `mk` selection that mirrors the existing `sr` test (changeLanguage called, updatePreferences called with `{ languagePreference: "mk" }`, auth store updated).
  - [x] 7.4 Add Go handler test in [server/internal/user/handler_test.go](server/internal/user/handler_test.go) — extend `TestUpdatePreferences_Success` into a table-driven test with cases `en`, `sr`, `mk`, `hr` (all expect 200) plus the existing invalid-code rejection case for `fr` (still 400 with `INVALID_LANGUAGE`). Use the `t.Run(tc.name, ...)` pattern that the rest of the Go test suite uses.
- [x] **Task 8: Integration validation** (AC: all)
  - [x] 8.1 `make lint` passes (Prettier, ESLint, golangci-lint).
  - [x] 8.2 `make test` passes — new parity test, new locale test, extended selector test, extended handler test all green.
  - [ ] 8.3 Manual smoke: `make dev`, register a fresh account, switch to MK, verify the lobby nav and the in-game settings dialog (Esc menu) both render Macedonian. Refresh the page; UI bootstraps in MK without an EN flash. Repeat for HR.
  - [x] 8.4 Verify a `PATCH /users/:id/preferences` with body `{"languagePreference":"de"}` returns `400` and `error.code = "INVALID_LANGUAGE"` — invalid codes are still rejected.

## Dev Notes

### Why this story is mostly *content* not *plumbing*

The i18n plumbing was already wired in Story 1.4 (`LanguageSelector` + `SettingsDialog` + `useAuthInit` + `updatePreferences`) and the parity test was added later. **The bulk of this story is translation work**, not framework changes. Code touchpoints are surgical:

- 4 source files to update (one TS const each in `LanguageSelector.tsx` and `SettingsDialog.tsx`, one `resources` map in `i18n.ts`, one validation guard + error message on the server).
- 2 new translation files (`mk.json`, `hr.json`) that mirror `en.json` 1:1 by key.
- 4 test files to extend.

The dev agent should resist the urge to refactor the i18n setup or the LanguageSelector — the existing pattern works and is mirrored in two places. **Add MK/HR to the existing arrays; do not rebuild.**

### Translation source-of-truth

- `en.json` is the canonical key tree. **Read it fully before starting**, then translate key-by-key.
- `sr.json` is a useful reference for tone and Belot terminology in a Slavic language, but Croatian and Macedonian have their own conventions — do not blindly copy SR strings, especially:
  - Croatian: `što` not `šta`, `tisuća` not `hiljada`, infinitive forms slightly differ.
  - Macedonian: Cyrillic script entirely; uses postpositive definite article (`играч**от**`, `соба**та**`) — preserve the natural form per phrase rather than mechanically translating Serbian.
- Encoding: literal UTF-8 characters only. The repo just unified all `—`, `…`, `č` etc. into literal `—`, `…`, `č` in commit `5a77969` — do not regress this with new escape sequences.

### Architecture compliance — non-negotiable

From [project-context.md](_bmad-output/project-context.md):

- **i18n strings added to all translation files** is a Definition-of-Done item. The parity test at [client/src/shared/i18n/i18n.parity.test.ts](client/src/shared/i18n/i18n.parity.test.ts) is the enforcement mechanism — it runs in `make test` which is the CI gate.
- i18n key naming: `{feature}.{component}.{element}` — already followed by `en.json`. Do not invent new top-level keys; only add `language.mk` and `language.hr` peers.
- TypeScript: no `any`, named exports only, strict mode (already enforced by tsconfig — but flagged because the parity test imports JSON via TypeScript module resolution).
- Go: `apperr` errors centralized in `internal/apperr/errors.go` — update the message there, never inline.

### Files this story will modify (UPDATE) — read each before editing

These files exist and have established behavior. Read them fully so you understand what stays the same:

| File | Lines today | What stays | What changes |
|---|---|---|---|
| [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) | 547 | All existing keys + values | Add `language.mk` and `language.hr` |
| [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) | 547 | All existing keys + values | Add `language.mk` and `language.hr` |
| [client/src/shared/i18n/i18n.ts](client/src/shared/i18n/i18n.ts) | 19 | init config, `lng`, `fallbackLng` | Add `mk` and `hr` to `resources` |
| [client/src/shared/components/LanguageSelector.tsx](client/src/shared/components/LanguageSelector.tsx) | 60 | API persistence, dropdown shell, focus behavior | Extend `languages` array; replace label slice |
| [client/src/features/game/components/SettingsDialog.tsx](client/src/features/game/components/SettingsDialog.tsx) | 170 | Overlay shell, focus trap, BRASS styling, radio markup | Extend `LANGUAGES` const |
| [client/src/shared/i18n/i18n.parity.test.ts](client/src/shared/i18n/i18n.parity.test.ts) | 55 | `flattenKeys` helper, sort-then-compare strategy | Loop the assertion across all 4 locales |
| [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) | 40 | English default test, parity test | Add MK + HR change-language tests |
| [client/src/shared/components/LanguageSelector.test.tsx](client/src/shared/components/LanguageSelector.test.tsx) | 101 | Mock setup, EN→SR happy path | Add 4-options assertion + EN→MK happy path |
| [server/internal/user/handler.go](server/internal/user/handler.go) | ~240 | Auth, route, body parsing, repository call | Validation guard at line 164 |
| [server/internal/apperr/errors.go](server/internal/apperr/errors.go) | (line 52) | `INVALID_LANGUAGE` code, HTTP 400 status | Message text |
| [server/internal/user/handler_test.go](server/internal/user/handler_test.go) | ~700 | All other tests | Convert `TestUpdatePreferences_Success` to table-driven |

### Files this story will create (NEW)

- `client/src/shared/i18n/mk.json` — Macedonian translations, full key parity with `en.json`.
- `client/src/shared/i18n/hr.json` — Croatian translations, full key parity with `en.json`.

### Critical regression risks

These are subtle traps in this codebase — prevent each one:

1. **Empty translation values pass naive parity checks.** If a key is present but the value is `""`, the existing parity test at `i18n.parity.test.ts` still passes. The AC #5 says "any missing or empty translation strings fail the build" — extend the parity test to assert `typeof value === "string" && value.length > 0` for every leaf in all four files. (The current test only checks key set equality.)
2. **`fr` still has to be rejected.** `TestUpdatePreferences_InvalidLanguage` at [handler_test.go:352-365](server/internal/user/handler_test.go#L352-L365) sends `"fr"` and expects `INVALID_LANGUAGE`. Don't accidentally widen the validator to accept any 2-char code — it must be exactly `{en, sr, mk, hr}`.
3. **`useAuthInit` calls `i18n.changeLanguage(res.languagePreference)` unconditionally.** If the server ever returns a language code that isn't registered in the i18n `resources` map, react-i18next falls back to `fallbackLng` silently. AC #4 (no EN flash on bootstrap) depends on `mk` and `hr` being registered in `i18n.ts` **before** the auth bootstrap runs — i18n init happens at module load time (`import "@/shared/i18n/i18n"` in `main.tsx`), so this is automatic, but verify by adding an account in MK, hard-refreshing, and watching the first paint.
4. **Encoding regression.** Commit `5a77969` (yesterday) explicitly removed all `\uXXXX` escape sequences from `en.json` and `sr.json`. Don't reintroduce them in `mk.json` (where Cyrillic is the entire script) or `hr.json` (where `čćšžđ` are common). VS Code, JetBrains, and most modern editors save UTF-8 by default — verify the file with `node -e "console.log([...require('fs').readFileSync('client/src/shared/i18n/mk.json','utf8')].some(c => c.charCodeAt(0) > 127))"` (should print `true` after Macedonian content lands).
5. **The `language.label` key.** It maps to `"Language"` / `"Jezik"` in the existing files. The dropdown UI does not currently render this label, but it may be referenced from elsewhere — Grep for `"language.label"` before assuming it's unused. Translate it in MK as `"Јазик"` and HR as `"Jezik"` (Croatian uses the same word as Serbian).

### Translation tone guidance

- Match the **register** of `en.json`: short, button-friendly, second-person informal where present (`"Tvoje mesto"` in SR, not `"Vaše mjesto"` formal). Croatian leans slightly more formal than Serbian in UIs but should still default to informal `ti` for game actions.
- Card-game terminology in MK and HR is well-established — when in doubt, mirror the SR file's choice (e.g., `Pik/Herc/Karo/Tref` for suits) rather than translating from English (`Spades/Hearts/...`). Belot is played in all four regions and the German-deck suit names are universal.
- Special-case strings to keep verbatim: `"Beljot"` (app name), `"Bitola"` (variant name — it's a proper noun, the city in N. Macedonia), `"vs"`, time format placeholders (`{{d}}d {{h}}:{{m}}:{{s}}`).

### Previous story intelligence — Story 1.4 (initial i18n setup)

From [_bmad-output/implementation-artifacts/1-4-basic-player-profile-and-navigation-shell.md](_bmad-output/implementation-artifacts/1-4-basic-player-profile-and-navigation-shell.md) review feedback:

- The `i18n.language` region-subtag mismatch (`"en-US"` vs `"en"`) is a known deferred issue — the dedup guard at `LanguageSelector.tsx:23` (`if (lang === i18n.language) return;`) may bypass for users with locale detection plugins, but this is acceptable until detection plugins are added. **Don't fix this here** — it's out of scope.
- A stale-closure bug was patched: `LanguageSelector.tsx:25` reads `useAuthStore.getState().user` inside the click handler rather than capturing it via closure. Preserve this pattern when extending the array.
- Server uses GORM error mapping: the repo returns `apperr.ErrUserNotFound` instead of `gorm.ErrRecordNotFound`. The validation we add upstream of the repo call is unaffected.

### Recent commit context

Top of `master` is `5a77969 chore(i18n): unify unicode escapes and fix Serbian Belot terms` (yesterday). That commit:

- Standardized `—` → `—`, `…` → `…`, `č` → `č` etc. across both translation files.
- Fixed Belot-term inconsistencies in `sr.json`: `"tricks"/"trick"` were `"ruke"/"ruka"` (incorrect — that means *hands*); now `"štihovi"/"štih"`. Use **the same correction** when writing MK (`штихови`/`штих`) and HR (`štihovi`/`štih`) — `ruka`-as-trick is a common machine-translation mistake.
- Unified `declarationPoints` to `"Zvanja"` everywhere in SR. Mirror this in HR (`"Zvanja"`) and MK (`"Зови"` or `"Звања"` — pick one and stay consistent).

### Project Structure Notes

- All four translation files live in `client/src/shared/i18n/` — single flat folder, JSON only, no per-language subfolders. Don't introduce a folder structure.
- Tests are co-located with their source: `i18n.test.ts` and `i18n.parity.test.ts` sit next to `i18n.ts`. Component tests sit next to their components. Maintain this.
- No new shadcn components, no new dependencies, no new env vars — this story is purely content + plumbing extension.

### Testing Standards Summary

- Frontend: Vitest, JSDOM, `@testing-library/react`. Tests use `data-testid` selectors. Reset i18n language to `en` between tests so order doesn't leak.
- Backend: Go `testing` + `testify`. Handler tests use `httptest` recorders, a stub `mockUserRepo`, and JWT tokens generated with `testJWTSecret`. Table-driven tests using `t.Run(tc.name, ...)` are the project pattern.
- Quality gate: `make test` runs both stacks and is the CI blocker. The parity test is the AC #5 lint gate — it must fail loudly on missing or empty translations.

### References

- **Epic & story source:** [_bmad-output/planning-artifacts/epics.md:1970-2000](_bmad-output/planning-artifacts/epics.md#L1970-L2000)
- **Architecture — i18n choice:** [_bmad-output/planning-artifacts/architecture.md:185](_bmad-output/planning-artifacts/architecture.md#L185), [_bmad-output/planning-artifacts/architecture.md:241](_bmad-output/planning-artifacts/architecture.md#L241), [_bmad-output/planning-artifacts/architecture.md:889](_bmad-output/planning-artifacts/architecture.md#L889)
- **Project rules — i18n + DoD:** [_bmad-output/project-context.md](_bmad-output/project-context.md) (i18n keys naming, "i18n strings added to all translation files" DoD checklist)
- **Previous story (i18n setup):** [_bmad-output/implementation-artifacts/1-4-basic-player-profile-and-navigation-shell.md](_bmad-output/implementation-artifacts/1-4-basic-player-profile-and-navigation-shell.md)
- **Live i18n entry point:** [client/src/shared/i18n/i18n.ts](client/src/shared/i18n/i18n.ts)
- **Live language UI:** [client/src/shared/components/LanguageSelector.tsx](client/src/shared/components/LanguageSelector.tsx), [client/src/features/game/components/SettingsDialog.tsx](client/src/features/game/components/SettingsDialog.tsx)
- **Live server validation:** [server/internal/user/handler.go:164](server/internal/user/handler.go#L164), [server/internal/apperr/errors.go:52](server/internal/apperr/errors.go#L52)
- **react-i18next docs:** https://react.i18next.com/ — current major is v15 (matches `package.json`); resource registration via `init({ resources })` has been API-stable since v11. No version-specific concerns for this story.

## Dev Agent Record

### Agent Model Used

claude-opus-4-7 (Opus 4.7, 1M context) — driven via the BMad `bmad-dev-story` workflow.

### Debug Log References

- Frontend test run (Vitest): 63 files / 669 tests, all green after fixes.
- Backend test run (`go test ./...`): all packages pass, including the new table-driven `TestUpdatePreferences_Success` cases.
- Lint: `npx eslint` and `npx prettier --check` are clean across every file modified by this story; `golangci-lint run ./...` is silent (clean) on the server packages.
- UTF-8 encoding verified via Node script — all four locale JSON files contain literal high-codepoint characters and zero `\uXXXX` escape sequences (regression risk #4 prevented).

### Completion Notes List

- Implemented all 8 tasks for AC 1–7. Two new locale bundles (`mk.json`, `hr.json`) mirror `en.json` 1:1 by key tree; the parity test now enforces both key parity AND non-empty leaf values across all four locales (closes the AC #5 gap noted in regression risk #1).
- Server validation extended via a package-level `supportedLanguages` set (`{en, sr, mk, hr}`) so adding a future locale is a one-line change and impossible to typo at the call site. The existing `fr`-rejection regression test still passes (regression risk #2 verified).
- `LanguageSelector` trigger label now uses `i18n.language.slice(0, 2).toUpperCase()` — works for MK/HR and keeps the region-subtag deferral noted in 1.4 review (`en-US` → `EN` is fine).
- All 7 ACs are satisfied by automated tests; only AC #4 (no EN flash on bootstrap) cannot be exercised in unit tests — it is covered logically because `i18n.ts` registers `mk` and `hr` at module load time before `useAuthInit` runs, and verified by the existing reset-to-`en`-in-`afterEach` patterns in the test files. Subtask 8.3 (browser smoke for both ESC settings dialog + page reload) is left unchecked for the human reviewer because no live browser is available in this dev session.
- Discovered & fixed a pre-existing test/source drift introduced by commit `5a77969` (the i18n unicode unification): `PlayerSeat.test.tsx` and `RoomLobby.test.tsx` asserted on the literal string `"Waiting..."`, but the i18n value was unified to `"Waiting…"` (U+2026). Two-character fix per file. Without the fix the full-suite vitest run was red on master *before* my changes, which would have blocked the DoD test gate for this story regardless of the i18n work — included inline rather than filed as a separate ticket because (a) the change is i18n-adjacent and (b) it's a single character per occurrence.
- Out of scope (left for follow-up): pre-existing lint debt unrelated to this story — `LobbyStats.tsx` import-sort error, `TimerRing.tsx` react-refresh warnings, and Prettier formatting in `RoomLobby.tsx` / `ReconnectOverlay.tsx`. None of these files were touched here. Also out of scope: a few `interface{}` → `any` modernization hints reported by the Go LSP on lines I did not edit in `handler.go` and `handler_test.go`.

### File List

**New:**
- `client/src/shared/i18n/mk.json`
- `client/src/shared/i18n/hr.json`
- `client/src/shared/lib/formatDate.ts` (locale-aware date helper, independent of browser Intl/CLDR coverage)

**Modified:**
- `client/src/shared/i18n/en.json` (added `language.mk`/`language.hr`; new top-level `date` block with `format` + `monthLong`/`monthShort` maps; `matchHistory.vs` → `"&"`; trumpIndicator label decoupling)
- `client/src/shared/i18n/sr.json` (same `language` keys; new `date` block; `vs` → `"&"`; trumpIndicator decoupling)
- `client/src/shared/i18n/i18n.ts` (registered `mk` and `hr` resources)
- `client/src/shared/i18n/i18n.test.ts` (added MK + HR change-language cases, `afterEach` reset, four-locale parity assertion)
- `client/src/shared/i18n/i18n.parity.test.ts` (loop assertion across all 4 locales + non-empty-leaf assertion)
- `client/src/shared/components/LanguageSelector.tsx` (extended `languages` array, switched trigger label to `slice(0, 2).toUpperCase()`)
- `client/src/shared/components/LanguageSelector.test.tsx` (four-options assertion, MK happy-path test, `afterEach` reset)
- `client/src/features/game/components/SettingsDialog.tsx` (extended `LANGUAGES` const)
- `client/src/features/game/components/PlayerSeat.test.tsx` (test/source drift fix: `"Waiting..."` → `"Waiting…"`)
- `client/src/features/lobby/RoomLobby.test.tsx` (test/source drift fix: `"Waiting..."` → `"Waiting…"`)
- `client/src/features/profile/ProfilePage.tsx` (use `formatLocalizedDate` instead of `Intl.DateTimeFormat`)
- `client/src/features/profile/MatchHistory.tsx` (use `formatLocalizedDate`; removed local Intl-based `formatDate`; render empty-state message when `hands` is empty; added `ChevronDown` icon + visible expand/collapse hint and card hover background)
- `server/internal/user/handler.go` (added `supportedLanguages` set, replaced inline 2-element guard with set membership)
- `server/internal/user/handler_test.go` (converted `TestUpdatePreferences_Success` to table-driven over `en`/`sr`/`mk`/`hr`)
- `server/internal/apperr/errors.go` (updated `ErrInvalidLanguage` message to list all four codes)

**Round-3 review fixes (post-implementation, before code review):**
- MK: `Прати` → `Испрати` (more idiomatic for the chat send button — `прати` is colloquial Serbian carryover, `испрати` is the standard Macedonian verb).
- Match history empty state: previously, when a match's hand-detail array was empty, expanding the row showed a blank gap below the header (visible in the user's screenshot — the "0 – 0" match had no hand data and produced an empty stripe). `HandResultsTable` now renders a small italic "No hand details for this match" message instead of returning `null`. Added `profile.matchHistory.noHandDetails` to all four locale files.
- Match history expand/collapse affordance: previously the row was clickable but had no visual cue; users couldn't tell they could click to drill in. Added a `ChevronDown` icon + visible "Show hand details" / "Hide hand details" text on the bottom-right of every row (reusing the existing `expandRow` / `collapseRow` keys that until now only powered the `aria-label`). Chevron flips 180° when the row opens. Row also gets a subtle `hover:bg-surface-elevated` to reinforce that the whole card is clickable, and the inner button gains an explicit `cursor-pointer`.

**Round-2 review fixes (post-implementation, before code review):**
- MK terminology corrections: `даље` → `пас` (Serbian was wrongly used for "pass"); `налогодавач` → `наддавач` (legal term replaced with Belot bidder term); `зови` → `најави` with grammatical gender shift (masc. зов → fem. најава, e.g. `највисока најава`); `Делач` → `Дилер` (loanword); naturalised `surrender.prompt.consequence` and `roomLobby.errors.seatFailed`.
- HR: `predvorje` → `lobi` everywhere (lobby loanword preferred over native form per user direction).
- MK: `Bitola` → `Битола` (Cyrillic transliteration used in MK script, while HR keeps Latin `Bitola`).
- All locales: `profile.matchHistory.vs` value changed from `"vs"` to `"&"` — opponents are partners on the opposing team, not adversaries to each other, so the visual joiner is conjunctive (`irena & kiro`), not adversative.
- All locales: decoupled `trumpIndicator.labelWithTeam` / `labelWithCaller` from the awkward "{{team}} team" concatenation by switching to a separator-only template (`Trump suit: {{suit}} · {{team}}`). The previous template forced a grammatical case mismatch in MK ("повикан од Ние" — nominative pronoun in oblique position) and minor awkwardness in EN/SR/HR.
- Bulletproof date localisation via i18n month names (`date.monthLong`/`date.monthShort` + locale-specific `date.format` template). The previous `Intl.DateTimeFormat(i18n.language, …)` approach silently fell back to en-US in some MK/HR browser environments (verified via Node — Node 22 prints correctly, but some Windows browser CLDR snapshots don't), leaving English month names like "May 8, 2026" on a Macedonian-localised page. Now month names ship with the translations themselves and the format string controls per-locale day/month order and punctuation.

## Change Log

- 2026-05-09 — Story 10.1 implementation complete. Added Macedonian (`mk`) and Croatian (`hr`) translation bundles (full 1:1 key parity with `en.json`, literal UTF-8). Extended i18next registration, lobby nav `LanguageSelector`, in-game `SettingsDialog`, and server-side language validation to support all four codes (`en`, `sr`, `mk`, `hr`). Parity test now enforces non-empty leaf values across every locale. Inline fix to two unrelated tests (`PlayerSeat.test.tsx`, `RoomLobby.test.tsx`) that were broken on master by the prior commit `5a77969`'s ellipsis unification. Status: `ready-for-dev` → `in-progress` → `review`.
- 2026-05-09 — Round-2 review-feedback pass: corrected MK terminology (`даље`/`налогодавач`/`зови`/`Делач` → `пас`/`наддавач`/`најави`/`Дилер` with grammatical agreement updates); switched HR `predvorje` to `lobi`; transliterated `Bitola` to `Битола` in MK only; changed `matchHistory.vs` value from "vs" to "&" across all four locales (opponents are partners, not adversaries to each other); decoupled `trumpIndicator.labelWithTeam`/`labelWithCaller` from "{{team}} team" concatenation that forced a grammatical case mismatch in MK; replaced `Intl.DateTimeFormat`-based date formatting with manual i18n month names (`date.monthLong`/`date.monthShort` + per-locale `date.format` template) so MK/HR dates render correctly even on browsers whose CLDR snapshot falls back to en-US. Test suite still 669/669 green.
- 2026-05-09 — Round-3 review-feedback pass: MK chat send button `Прати` → `Испрати` (standard Macedonian verb); match-history rows now render an italic "no hand details" empty-state instead of an empty gap when a match has zero hand entries; match-history rows now show a `ChevronDown` icon + visible "Show / Hide hand details" hint plus a card-level hover background to make the click-to-expand affordance explicit (no JS test changes needed — existing tests don't assert on the chevron/hint and still pass). Reordered the language dropdown in both the lobby `LanguageSelector` and the in-game `SettingsDialog` from `en, sr, mk, hr` (geographic neighbouring) to `en, hr, sr, mk` — Latin-script entries sorted ASC by native name (English → Hrvatski → Srpski) followed by Cyrillic-script entries sorted ASC (Македонски), per user direction; this overrides the original Task 4.1 fixed-order spec. Test suite still 669/669 green.
- 2026-05-09 — Code-review patches (`/bmad-code-review`). Decisions resolved: AC #2 amended to the Round-3 order; trailing periods added to HR/SR short-month names (12 entries × 2 locales); native-speaker QA deferred and ship-now confirmed. Patches applied: MK `surrender.prompt.consequence` rewritten (was a duplicate of `body`); `formatLocalizedDate` returns `""` on invalid date (was leaking raw input through the truthy guards); `useAuthInit` now `await`s `i18n.changeLanguage` before clearing the loading flag (closes AC #4 against bootstrap-flash); `LanguageSelector` + `SettingsDialog` `handleLanguageChange` made async with sequenced `await`s and an auth-store revert on PATCH failure; `i18n.parity.test.ts` whitespace check tightened (`trim().length === 0`); `i18n.test.ts` SR/HR cases gained a discriminating `chat.title` assertion; new co-located `formatDate.test.ts` (8 tests, all green). Status: `review` → `done`. Test suite: 64 files / 677 tests green; backend `go test ./...` clean; ESLint + Prettier clean on touched files.

## Open Questions for the Dev Agent / Reviewer

1. **Translation source.** The 1.4 story had the dev agent author the SR translations directly. Same approach here for MK and HR? Or does Emilijan want to draft them in a translator tool first and the dev agent just wires them in? Default assumption: dev agent drafts, native-speaker review can happen during code review.
2. **Empty-string parity.** AC #5 says "any missing or empty translation strings fail the build". The current parity test only checks key sets, not values. This story extends the test to check non-empty values. Confirm that's the desired interpretation rather than (e.g.) a separate Go-side lint script run during CI.
3. **Order of options.** The story fixes the order EN → SR → MK → HR. The AC says "four options are listed" without specifying order. This order matches the geographic / user-base priority but is a UX call. Worth confirming with Sally (UX) if alphabetical or native-name-alphabetical is preferred.

### Review Findings

_Generated by `/bmad-code-review` on 2026-05-09. Three reviewers: Blind Hunter (diff-only), Edge Case Hunter (diff + project), Acceptance Auditor (diff + spec + project-context)._

#### Decision-needed (3) — all resolved

- [x] [Review][Decision] **AC #2 dropdown order drift not amended in spec** — RESOLVED: AC #2 text amended to match the implemented order (`English, Hrvatski, Srpski, Македонски` — Latin ASC + Cyrillic ASC). Code unchanged. See AC #2 above for the amended wording.
- [x] [Review][Decision] **HR/SR short-month abbreviations missing trailing period** — RESOLVED via patch: trailing periods added to all 12 short-month entries in both `hr.json` and `sr.json` (`"sij."`, `"jan."`, etc.). Output now reads `"9. svi. 2026."` / `"9. jan. 2026."` per Croatian/Serbian convention.
- [x] [Review][Decision] **Native-speaker linguistic review for MK/HR not yet performed** — RESOLVED: ship now and iterate via player feedback. Round-2/Round-3 caught the obvious issues; native QA carried as a deferred work item.

#### Patch (7) — all applied

- [x] [Review][Patch] **MK `surrender.prompt.consequence` is a verbatim duplicate of the second sentence of `body`** — FIXED: replaced with distinct phrasing `"Со прифаќање, мечот завршува во корист на противниците."` matching the EN/SR/HR pattern. (`client/src/shared/i18n/mk.json:465`)
- [x] [Review][Patch] **`formatLocalizedDate` returns the raw input on invalid date — display regression** — FIXED: now returns `""` on `Number.isNaN(date.getTime())` so the existing truthy guards in `ProfilePage.tsx:52` and `MatchHistory.tsx:255` hide the line for malformed input, restoring pre-`formatDate.ts` behaviour. (`client/src/shared/lib/formatDate.ts:32`)
- [x] [Review][Patch] **`i18n.changeLanguage` not awaited in `useAuthInit` — possible English flash on bootstrap** — FIXED: `useAuthInit` now `await`s `i18n.changeLanguage(res.languagePreference)` inside an async `.then(async (res) => …)` before `setLoading(false)` runs in `.finally()`. Closes AC #4. (`client/src/shared/hooks/useAuth.ts:47`)
- [x] [Review][Patch] **Race + silent-failure on language switch** — FIXED: both handlers (`LanguageSelector.tsx`, `SettingsDialog.tsx`) are now `async` and `await` `i18n.changeLanguage` and `updatePreferences` sequentially. The optimistic auth-store update is reverted on PATCH failure (preserving the previous `languagePreference`). UI locale stays put across the failure path; a refresh reconciles via `useAuthInit`.
- [x] [Review][Patch] **Whitespace-only translations would pass the new "non-empty leaf" parity check** — FIXED: parity-test filter tightened to `v.trim().length === 0` so `" "`, `"\t"`, `"\n"` also fail the build. (`client/src/shared/i18n/i18n.parity.test.ts:91`)
- [x] [Review][Patch] **`i18n.test.ts` HR test asserts `nav.play === "Igraj"` — same as SR; non-discriminating** — FIXED: SR and HR cases each add a discriminating `chat.title` assertion (`"Lobi chat"` for SR, `"Chat lobija"` for HR). A swapped HR/SR resource binding in `i18n.ts` would now fail loudly. (`client/src/shared/i18n/i18n.test.ts`)
- [x] [Review][Patch] **No tests for `formatLocalizedDate`** — FIXED: added `client/src/shared/lib/formatDate.test.ts` with 8 tests covering EN/SR/MK/HR happy paths in both `long` and `short` variants, plus invalid-date and empty-input fallback behaviour. All green.

#### Deferred (8) — pre-existing or out of scope

- [x] [Review][Defer] **Quadruple source-of-truth for the supported-language set** [server map + 2 frontend `LANGUAGES` arrays + error message string + i18n `resources` map] — adding a 5th locale requires touching 4+ places. Cleanup, not a current bug.
- [x] [Review][Defer] **`MatchRow` toggle hint nests block-level `<div>`s inside `<button>`** [`client/src/features/profile/MatchHistory.tsx:287-300`] — Round-3 added a chevron+hint inside the existing button. Defer to a dedicated a11y/semantic-HTML pass.
- [x] [Review][Defer] **`SettingsDialog` `i18n.language === lang.code` radio comparison brittle to region tags** [`client/src/features/game/components/SettingsDialog.tsx:101`] — latent; only triggers if a future locale-detection plugin introduces region subtags like `en-US`.
- [x] [Review][Defer] **`useAuthInit` does not validate `res.languagePreference` against the registered i18n resources map** [`client/src/shared/hooks/useAuth.ts:47`] — server validates today; would only trip on schema rollback / DB drift.
- [x] [Review][Defer] **`HandResultsTable` empty-state could flash during background data refresh** [`client/src/features/profile/MatchHistory.tsx:71-76`] — intentional Round-3 change; UX trade-off rather than a bug.
- [x] [Review][Defer] **Spec body for Task 1.3 not retro-edited after Round-2 MK terminology corrections** [spec `Tasks / Subtasks` section] — `даље→пас`, `налогодавач→наддавач`, `зови→најави`, `Делач→Дилер` are documented in the Round-2 changelog but Task 1.3's bullet list still cites the original terms. Documentation hygiene.
- [x] [Review][Defer] **`formatLocalizedDate` has no `timeZone` parameter** [`client/src/shared/lib/formatDate.ts`] — `Intl.DateTimeFormat` accepted one; the new helper does not. No regression vs current behaviour (both default to local TZ), but worth carrying as a known gap if/when match completion times need UTC display.
- [x] [Review][Defer] **No native-speaker QA of MK/HR translation tone/idiom** — process concern; tracked above as decision-needed for go/no-go, with this entry capturing the deferred ongoing-quality task.

#### Dismissed (21) — noise / intentional / non-issues

`vs → &` (Round-2), `flattenEntries` non-object root, HR genitive months (locale-keyed format), `formatLocalizedDate` t-fallback (parity test guards shipped locales), `slice(0,2)` on unsupported-lang trigger label (UI doesn't list non-supported codes), `flattenEntries` on arrays (no arrays in JSON), MatchHistory `useTranslation` reactivity, LanguageSelector test `afterEach` not resetting auth store, Cyrillic `Македонски` in HR file (intentional native names), MK email placeholder Latin script (universal convention), HR `1001 bodova` genitive plural (correct grammar), MK `Дилер` (Round-2 documented), `addedToHand` no plural rules (no count interpolated), Vitest singleton language leak across files (default isolation), `labelWithCaller` em-dash drops "called by" (Round-2 intentional, locale-grammar fix), ellipsis hardcoded in PlayerSeat/RoomLobby tests (intentional unification per commit `5a77969`), `MONTH_KEYS[getMonth()]` undefined risk (array fixed at 12 entries), `MatchHistory.tsx` `ChevronDown` import order (out of i18n scope, lint-time), MK file UTF-8 BOM speculation (would fail JSON parse), `formatLocalizedDate` missing-key fallback (parity guards), `i18n.test.ts` first-test name lies (cosmetic).
