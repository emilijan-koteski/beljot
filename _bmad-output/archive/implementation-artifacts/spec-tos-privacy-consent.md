---
title: "Required ToS & Privacy consent on register, with WIP placeholder pages"
type: "feature"
created: "2026-04-29"
status: "done"
route: "one-shot"
---

# Required ToS & Privacy consent on register, with WIP placeholder pages

## Intent

**Problem:** The app collects email + username + password on register without any Terms of Service or Privacy Policy disclosure or consent — and the legal copy itself does not exist yet (the project is still WIP / used for testing).

**Approach:** Pure-frontend addition. Add `/terms` and `/privacy` placeholder pages that explicitly say the app is a work-in-progress used for testing and that the real legal copy is coming soon. Add a required consent checkbox on the Register form whose label links to those pages (opens in a new tab). Block registration submit until the checkbox is checked, with both a disabled-button gate and an inline validation error for defense-in-depth.

## Suggested Review Order

1. [client/src/App.tsx](../../../client/src/App.tsx) — confirm `/terms` and `/privacy` are public routes (outside both `GuestRoute` and `ProtectedRoute`) so authed and unauthed users can both reach them.
2. [client/src/features/legal/TermsPage.tsx](../../../client/src/features/legal/TermsPage.tsx) and [PrivacyPage.tsx](../../../client/src/features/legal/PrivacyPage.tsx) — verify the WIP badge + notice + "coming soon" copy is wired through `legal.*` i18n keys.
3. [client/src/features/auth/RegisterPage.tsx](../../../client/src/features/auth/RegisterPage.tsx) — checkbox state, inline links open in a new tab with `rel="noopener noreferrer"`, submit disabled until checked, and `handleSubmit` validates `acceptedTerms` independently.
4. [client/src/shared/i18n/en.json](../../../client/src/shared/i18n/en.json) and [sr.json](../../../client/src/shared/i18n/sr.json) — `legal.*`, `auth.register.consent.*`, and `auth.register.errors.consentRequired` are present and consistent in both locales.
5. [client/src/features/auth/RegisterPage.test.tsx](../../../client/src/features/auth/RegisterPage.test.tsx) — existing happy-path tests now check the consent box; new tests cover the consent gate, error clearing, and link attributes.
6. [client/src/features/legal/TermsPage.test.tsx](../../../client/src/features/legal/TermsPage.test.tsx) and [PrivacyPage.test.tsx](../../../client/src/features/legal/PrivacyPage.test.tsx) — render checks for title, WIP notice, and back-to-root link.
