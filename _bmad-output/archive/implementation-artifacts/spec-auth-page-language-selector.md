---
title: 'Language selector on /login and /register with persistence into profile'
type: 'feature'
created: '2026-05-23'
status: 'done'
baseline_commit: '10b0d5f07d9599226046531fd9d73f84ebebcf91'
context:
  - '{project-root}/_bmad-output/project-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `/login` and `/register` render only in English. All four locales (`en`, `hr`, `sr`, `mk`) ship with full bundles but the selector lives only in the authenticated lobby nav; visitors cannot read the auth flow in their language, and any pick is not carried into their profile.

**Approach:** Add a pre-auth selector on both auth pages. Persist the choice in `localStorage` so reloads keep the language with no English flash. On **register**, include the active locale in the `POST /auth/register` body so the new user's profile defaults to it. On **login**, if the active locale differs from the user's stored `languagePreference`, reuse `PATCH /users/:id/preferences` to reconcile — same optimistic-with-rollback pattern as `LanguageSelector.tsx`.

## Boundaries & Constraints

**Always:**
- Same four codes and same order as `LanguageSelector.tsx` (`en, hr, sr, mk`).
- Seed initial `lng` synchronously in `i18n.ts` from `localStorage["beljot.lang"]` (allowlist-guarded) — first React paint must already be correct.
- One write site: `i18n.on("languageChanged", …)` in `i18n.ts` writes localStorage. Never `setItem` from a component (covers both selectors uniformly).
- Server validates register-body `languagePreference` against the existing `supportedLanguages`; absent or unsupported → silently default to `"en"` (do not 4xx a register over a bad locale).
- Reuse `PATCH /users/:id/preferences` for post-login reconciliation. On PATCH failure, revert `authStore.user.languagePreference` but keep UI language as picked.

**Ask First:** none — the lobby selector benefits from the shared listener as a side effect of the centralised write site; no separate decision needed.

**Never:**
- No new languages. No `i18next-browser-languagedetector` or detection plugins.
- No `languagePreference` on the login request body. No new endpoints.
- Do not block register submission on language validity.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected | Error |
|---|---|---|---|
| Pick HR, reload `/login` | empty localStorage → click HR | `localStorage["beljot.lang"]="hr"`; reload boots in HR, no EN flash | N/A |
| Pick MK then register | active `mk` → submit register | body has `"languagePreference":"mk"`; new user `language_preference='mk'`; lobby in MK | N/A |
| Pick SR then login (stored pref `en`) | active `sr`, response `"en"` | one `PATCH /users/:id/preferences {languagePreference:"sr"}`; authStore updated; lobby in SR | PATCH fails → revert authStore to `"en"`, keep UI in SR, silent |
| Login, language matches stored | active locale == stored pref | No PATCH; existing flow switches `i18n` to stored pref | N/A |
| Register body has unsupported code | `"languagePreference":"fr"` | User created with `"en"`; response `"en"` — no 4xx | N/A |
| Junk localStorage on boot | `"beljot.lang"="xx"` | `i18n` boots in `"en"`; value stays until next supported pick | N/A |

</frozen-after-approval>

## Code Map

- `client/src/shared/components/AuthLanguageSelector.tsx` -- NEW; pre-auth dropdown; no auth-store or API calls; same 4 entries + order as `LanguageSelector`.
- `client/src/shared/i18n/i18n.ts` -- seed initial `lng` from localStorage (allowlist); install `languageChanged` listener that writes localStorage.
- `client/src/features/auth/LoginPage.tsx` -- mount selector top-right; post-login PATCH on mismatch with revert on failure.
- `client/src/features/auth/RegisterPage.tsx` -- mount selector top-right; include `languagePreference: i18n.language` in register payload.
- `client/src/shared/api/auth.ts` -- add `languagePreference: string` to `RegisterRequest`.
- `client/src/shared/i18n/{en,hr,sr,mk}.json` -- add `auth.languageSelector.label` (parity gate).
- `server/internal/user/handler.go` -- export `IsSupportedLanguage(code string) bool` over the existing set.
- `server/internal/auth/handler.go` -- accept `languagePreference` in register; supported → use, else → `"en"`.

## Tasks & Acceptance

**Execution:**
- [x] `client/src/shared/i18n/i18n.ts` -- seed `lng` from `localStorage["beljot.lang"]` with allowlist `en|hr|sr|mk` (else `en`); add `i18n.on("languageChanged", …)` writing localStorage for supported codes.
- [x] `client/src/shared/components/AuthLanguageSelector.tsx` -- NEW; same `languages` array + order as `LanguageSelector`; on pick → `await i18n.changeLanguage(code)`; absolute-positioned; `aria-label` from `auth.languageSelector.label`; `data-testid="auth-language-option-${code}"` on items.
- [x] `client/src/features/auth/LoginPage.tsx` -- mount selector; after `loginMutation.mutateAsync`, if `i18n.language !== res.languagePreference` call `updatePreferences(res.id, { languagePreference: i18n.language })` and update authStore; on PATCH error revert authStore (keep i18n); then navigate.
- [x] `client/src/features/auth/RegisterPage.tsx` -- mount selector; pass `languagePreference: i18n.language` in register payload.
- [x] `client/src/shared/api/auth.ts` -- add `languagePreference: string` to `RegisterRequest`.
- [x] `client/src/shared/i18n/{en,hr,sr,mk}.json` -- add non-empty `auth.languageSelector.label` in each locale.
- [x] `server/internal/user/handler.go` -- export `IsSupportedLanguage(code) bool`.
- [x] `server/internal/auth/handler.go` -- extend `RegisterRequest` with `LanguagePreference string \`json:"languagePreference"\``; supported → use, else `"en"`; persist on the new user row.
- [x] Tests cover every I/O Matrix row: new `AuthLanguageSelector.test.tsx`; extended `i18n.test.ts` (seed + listener); extended `LoginPage.test.tsx` (PATCH fires / no-PATCH / failure-rollback); extended `RegisterPage.test.tsx` (payload field); extended `server/internal/auth/handler_test.go` (`mk` / absent / `fr` → `"en"`). Parity test verifies the new key in all 4 locales.

**Acceptance Criteria (beyond the I/O Matrix):**
- Given the selector renders on `/login` or `/register`, when no user is authenticated, then it makes zero calls to `updatePreferences` or `useAuthStore.setUser` (the auth-store/API path belongs to the lobby `LanguageSelector` only).
- Given the dropdown is open, when entries render, then exactly four entries appear in the order `English, Hrvatski, Srpski, Македонски`.

## Spec Change Log

- 2026-05-23 — Implementation complete. Three-reviewer adversarial pass (blind hunter / edge-case hunter / acceptance auditor) applied patches: (1) region-subtag normalization via new `normalizeLanguage` helper in `i18n.ts`, used in `LoginPage.handleSubmit` and `RegisterPage.handleSubmit`, and inside the `languageChanged` listener so `"en-US"` never reaches the server or localStorage; (2) reordered `SUPPORTED_LANGUAGES` in `i18n.ts` to `[en, hr, sr, mk]` to match the spec's stated dropdown order; (3) SSR `typeof window` guard around `window.localStorage` access; (4) `afterEach` localStorage cleanup added to `i18n.test.ts` so state doesn't leak between Vitest files; (5) new tests cover `normalizeLanguage` (4 supported codes verbatim, region-subtag strip, case-insensitivity, junk/empty/null/undefined rejection) and listener-edge cases (region-tagged code normalized before write; unsupported code does not clobber existing supported value). Deferred: PATCH-blocks-navigation UX, 5-site supported-language source-of-truth growth, AbortController for the post-login reconcile. Final: 718/718 frontend tests, all Go tests, ESLint + Prettier + golangci-lint clean.

## Verification

- `make lint` — clean.
- `make test` — all suites green including new/extended tests and the four-locale parity check.
- Manual: `make dev`, private window → `/register`, pick HR, reload (still HR, no flash), register → lobby in HR. Log out → `/login`, pick MK, login as an `en`-pref user → DevTools shows one `PATCH /users/:id/preferences {"languagePreference":"mk"}`; lobby in MK.

## Suggested Review Order

**Persistence & boot**

- New normalize+seed+listener entry point — read this first to grasp the localStorage contract.
  [`i18n.ts:23`](../../../client/src/shared/i18n/i18n.ts#L23)

- Region-subtag guard exposed for reuse at every payload/compare site.
  [`i18n.ts:30`](../../../client/src/shared/i18n/i18n.ts#L30)

**Auth-page UI**

- Pre-auth dropdown: no auth-store / no API, mirrors the lobby selector's languages array.
  [`AuthLanguageSelector.tsx:13`](../../client/src/shared/components/AuthLanguageSelector.tsx#L13)

- Selector mount on `/login` (top-right absolute), then post-login reconcile.
  [`LoginPage.tsx:67`](../../../client/src/features/auth/LoginPage.tsx#L67)

- Selector mount on `/register` and language carried into the register payload.
  [`RegisterPage.tsx:91`](../../../client/src/features/auth/RegisterPage.tsx#L91)

**Server contract**

- Register handler now reads `languagePreference` from the body, allowlist via shared helper, silent fallback to `"en"`.
  [`auth/handler.go:105`](../../../server/internal/auth/handler.go#L105)

- Single-source allowlist for both auth and preferences.
  [`user/handler.go:39`](../../../server/internal/user/handler.go#L39)

**API contract & i18n keys**

- `RegisterRequest` extended on the client side.
  [`auth.ts:5`](../../../client/src/shared/api/auth.ts#L5)

- Parity-gate i18n key added in all four locales.
  [`en.json:561`](../../../client/src/shared/i18n/en.json#L561)

**Tests (last)**

- `normalizeLanguage` + listener + parity coverage.
  [`i18n.test.ts:31`](../../../client/src/shared/i18n/i18n.test.ts#L31)

- AuthLanguageSelector unit tests: 4-option order, no API/auth-store calls, localStorage write.
  [`AuthLanguageSelector.test.tsx:31`](../../client/src/shared/components/AuthLanguageSelector.test.tsx#L31)

- Login reconcile happy/no-op/failure paths.
  [`LoginPage.test.tsx:201`](../../../client/src/features/auth/LoginPage.test.tsx#L201)

- Register payload includes the active language.
  [`RegisterPage.test.tsx:177`](../../../client/src/features/auth/RegisterPage.test.tsx#L177)

- Server table-driven cases for absent / empty / unsupported / each supported code.
  [`auth/handler_test.go:175`](../../../server/internal/auth/handler_test.go#L175)
