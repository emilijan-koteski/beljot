---
baseline_commit: dd59662ca56bb4b80c1f0bb10d74fb2345ea34a4
---

# Story 11.3: Public Player Profiles

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to view other players' public profiles,
so that I can see their reliability, progression, and competitive history.

## Acceptance Criteria

**Epic source:** `_bmad-output/planning-artifacts/epics.md#Story-11.3-Public-Player-Profiles` (FR47). The design decisions that shape these ACs (public-endpoint shape, XP-privacy override, career-points net-new work, Add-Friend deferral, season omit) are recorded in Dev Notes → **Design Decisions**.

1. **AC1 — Public profile endpoint.** An authenticated viewer requesting `GET /api/v1/users/:id/profile` for **another** player's id receives that player's **public** profile (HTTP 200, no longer `403`): `id`, `username`, `level`, `totalXp`, honor score + tier label + raw completed/abandoned counts + recent-trend indicator + `isNewPlayer`, win/loss/abandoned record, and `createdAt` (member-since). Private fields are **absent** from the payload: `email`, `walletBalance`, `loginStreakDays`, `languagePreference`, `usernameChangedAt`, `passwordHash`. All subject data is computed for the **path id (`:id`)**, never the authenticated viewer.
2. **AC2 — Career sidebar is public.** `GET /api/v1/users/:id/career` and `GET /api/v1/users/:id/matches` return the subject player's career aggregates and match history for a non-self viewer (200, keyed on `:id`). Career payload additionally includes **`careerPoints`** (lifetime total game points the player scored across completed matches — net-new aggregate). Only participant usernames + ids are exposed (never PII).
3. **AC3 — New Player honor treatment.** When the requested player has **< 5 completed matches** (`isNewPlayer == true`, threshold `HONOR_NEW_PLAYER_MIN_MATCHES = 5`), the honor section shows the "New Player" label (`data-testid="profile-honor-new"`, "N / 5") in place of a tier, and the raw completed/abandoned counts are still shown. Honor score/tier remain populated in the payload (the client decides suppression).
4. **AC4 — Read-only layout parity.** The public profile at route `/players/:id` uses the **same visual layout** as the player's own profile but with **no edit capabilities**: no username edit pencil, no linked-accounts (SSO) management, no preferences editing, and no viewer-data leakage (the viewer's own wallet/streak/honor must never render or hydrate on someone else's page). A "View Match History" surface (the subject's match history) is present (match history is always public in Phase 3).
5. **AC5 — Graceful absence of season data (Epic 13 not built).** The profile renders normally with **no seasonal-rank section and no prior-season archive** in the DOM. There is no season data anywhere today, so this AC is satisfied by simply not adding a season section; the rest of the profile renders unaffected.
6. **AC6 — "Add Friend" is deferred to Story 11.2.** The friend model, friendship-state, and the "Add Friend" action button are **out of scope for this story** and owned by Story 11.2 (no friend backend exists on `master`). Story 11.2 will add the button onto this public profile. This story must leave a clean, documented insertion point but must **not** build a friend backend or a dead/placeholder request button. See Dev Notes → **Design Decisions D4**.
7. **AC7 — Not-found + i18n + quality gates.** An unknown/soft-deleted `:id` returns `404` (`USER_NOT_FOUND`) and the page renders a localized not-found state. All new user-facing strings are in all four locales (`en`/`sr`/`mk`/`hr`), `mk` all-Cyrillic, no em dash in `mk`/`sr`/`hr`, parity test green. `make lint` + `make test` pass.

## Tasks / Subtasks

- [x] **Task 0: Branch setup**
  - [x] Cut `feat/11-3-public-player-profiles` from `master` (Epic 9 honor is merged, PR #24 / `dc14308`). This story is **independent of Story 11.1** (search) — a public profile is reachable directly by URL; 11.1 later points its result-click at this story's `/players/:id` route.

- [x] **Task 1: Backend — public profile projection on `GET /users/:id/profile`** (AC: #1, #3)
  - [x] Add a `PublicProfileResponse` struct in [server/internal/user/handler.go](server/internal/user/handler.go) with **only** the public-safe fields: `ID`, `Username`, `CreatedAt`, `Level`, `TotalXP`, `XPIntoLevel`, `XPForNextLevel`, the seven honor fields (`HonorScore`, `HonorTier`, `HonorCompletedTotal`, `HonorAbandonedTotal`, `IsNewPlayer`, `HonorTrendDelta`, `HonorTrendDirection`), and `TotalGamesPlayed`/`Wins`/`Losses`/`Abandoned`. Do **not** include `WalletBalance`, `LoginStreakDays`, `LanguagePreference`, `UsernameChangedAt`, `Email`. Update the `ProfileResponse` doc comment (lines ~29-32) that says XP fields must stay off a public DTO — see Design Decision D2 (this story deliberately overrides that with `level`+`totalXp` per the epic AC). **DONE:** struct added; both the top `ProfileResponse` comment and the XP-privacy comment updated for D2.
  - [x] Refactor `GetProfile`: parse `paramID`; **remove the `if paramID != authUserID { return apperr.ErrForbidden }` gate**; load the subject with `FindByID(subjectID)` → `ErrUserNotFound` (404) if nil; compute stats/level/honor-snapshot/honor-trend **keyed on `subjectID`**; then branch: **if `paramID == authUserID`** return the full `ProfileResponse` (self shape UNCHANGED), **else** return `PublicProfileResponse`. **DONE.**
  - [x] **CRITICAL:** every repo call and honor-trend query in the handler body uses `subjectID` (from `paramID`), not `authUserID`. Pinned by `TestGetProfile_ForeignID_ReturnsPublicProjection` (subject honor 83 vs viewer 86; `lastStatsUserID == subject`).
  - [x] **Tests** [server/internal/user/handler_test.go](server/internal/user/handler_test.go):
    - [x] `TestGetProfile_Forbidden` → `TestGetProfile_ForeignID_ReturnsPublicProjection` (200 + subject-keying proof); `TestGetProfile_Forbidden_WraparoundID` → `TestGetProfile_HugeUnknownID_NotFound` (404); foreign-id sub-case removed from `TestGetProfile_AuthFailures_DoNotCallStats`.
    - [x] `TestGetProfile_PublicProjection_NeverLeaksPrivateFields`: body `NotContains` email/passwordHash/walletBalance/loginStreakDays/languagePreference/usernameChangedAt.
    - [x] Non-self returns the **subject's** honor/stats (score 83 vs viewer 86; `lastStatsUserID`).
    - [x] `< 5 completed` subject → `isNewPlayer:true` with score/tier populated (`TestGetProfile_PublicProjection_NewPlayerCarriesScore`).
    - [x] Unknown `:id` → 404 `USER_NOT_FOUND` (`TestGetProfile_PublicProjection_UnknownSubject404`).
    - [x] Self id → full `ProfileResponse` shape unchanged (existing `TestGetProfile_Includes*` regression guards still green).

- [x] **Task 2: Backend — de-self-gate career + matches; add career points** (AC: #1, #2, #4)
  - [x] `GetCareer`: removed the self-only 403; added a `FindByID(subjectID)` 404 guard; swapped **all** repo calls to `subjectID` (`GetCareerAggregatesForUser`, `GetCareerPointsForUser`, `GetTopPartnersForUser`, `GetTopRivalsForUser`). Auth still required (viewer id read, discarded).
  - [x] Added `GetCareerPointsForUser(userID uint) (int64, error)` on the `MatchRepository` interface ([server/internal/match/repository.go](server/internal/match/repository.go)) + GORM impl (sum of the subject's own team score over **completed** matches; team from seat via `viewerTeamCase`; no migration). Added `CareerPoints int64` to `CareerResponse`, wired in `GetCareer`.
  - [x] `ListMatches`: removed the self-only 403; added a `FindByID(subjectID)` 404 guard; swapped `GetMatchesForUser(subjectID, …)` and `buildMatchListItem(m, subjectID, …)` so outcome + `viewerSeat` are the **subject's**.
  - [x] Updated **all** `MatchRepository` mocks: `user/handler_test.go`, `match/manager_test.go`, `match/matchend_test.go` (grepped the tree; honor_service uses a narrow local interface, unaffected). Build confirmed green.
  - [x] **Tests:** `TestGetCareer_ForeignSubject_Public` (200 + `careerPoints` keyed on subject via `lastCareerPointsID`); DB-backed `TestGormMatchRepository_GetCareerPointsForUser` (a→1000, b→800, none→0, abandoned excluded — RAN not skipped on dev DB :5433); `TestListMatches_ForeignSubject_ViewerRelativeToSubject` (viewerSeat/outcome from subject); `TestListMatches_UnknownSubject_NotFound` (404); GetCareer forbidden sub-case → unknown-subject 404.

- [x] **Task 3: Frontend — public profile page + route** (AC: #1, #3, #4, #5, #7)
  - [x] Added `PublicProfileResponse` TS type + `getPublicProfile(id)` in [client/src/shared/api/profile.ts](client/src/shared/api/profile.ts) (same URL, narrower type omitting wallet/streak/lang/usernameChangedAt). Added `careerPoints: number` to `CareerResponse` in [client/src/shared/api/career.ts](client/src/shared/api/career.ts).
  - [x] Added `usePublicProfileQuery(id)` in [client/src/shared/hooks/queries/usePublicProfile.ts](client/src/shared/hooks/queries/usePublicProfile.ts). **Chose a DISTINCT `queryKeys.publicProfile` namespace** (not `profile`) so the self full-shape and public narrow-shape entries can never collide for the same id — documented in queryKeys.ts.
  - [x] Created [client/src/features/profile/PublicPlayerProfilePage.tsx](client/src/features/profile/PublicPlayerProfilePage.tsx): `useParams` → `Number(id)` guarded (NaN/≤0 → not-found); `usePublicProfileQuery` + `useCareerQuery` + `MatchHistory`. Composes IdentityHero (read-only), HonorHeroBand (via `honor`), StatsGrid, MatchHistory, PartnerSpotlight/Rivalries/Milestones. **Excludes** the honor hydration effect, `<LinkedAccounts>`, edit pencil (D5 — separate page, not dual-mode). Renders loading / `public-profile-not-found` (404) / `public-profile-error`. No season section.
  - [x] Added `hidePrivatePills?: boolean` to [IdentityHero.tsx](client/src/features/profile/components/IdentityHero.tsx) (suppresses wallet + login-streak pills; keeps level/XP per D2). Public page passes `hidePrivatePills` + `userId={undefined}` (hides edit pencil) and never reads `useAuthStore` for the subject. Self page unchanged (default false).
  - [x] Fixed the `MatchHistory` "YOU" seat chip: added `subjectIsSelf?: boolean` (default true). On a public profile (`false`) the subject's seat shows the subject's username with no "You" badge (`SeatChip you={subjectIsSelf}`). Self behavior unchanged.
  - [x] Added the route in [client/src/App.tsx](client/src/App.tsx): `/players/:id` inside the `ProtectedRoute → AppLayout` group (TopBar + auth gate).
  - [x] **Tests** ([PublicPlayerProfilePage.test.tsx](client/src/features/profile/PublicPlayerProfilePage.test.tsx), 10): subject profile for a foreign id (honor 30 = subject, not viewer 90); no wallet/streak pill, no edit pencil, no LinkedAccounts; **honor hydration does NOT fire** (viewer store honor stays 90 — inverse of ProfilePage); New Player → `profile-honor-new`; unknown id → not-found; non-numeric id → not-found without an API call; no season DOM; match-history labels the subject (no "YOU"); careerPoints rendered. Plus 2 IdentityHero `hidePrivatePills` unit tests.

- [x] **Task 4: "Add Friend" + "View Match History" scoping** (AC: #4, #6)
  - [x] Did **not** implement Add Friend / any friend backend (D4). Left a clear Story-11.2 insertion comment in `PublicPlayerProfilePage.tsx` (under the IdentityHero) and recorded the deferral in [deferred-work.md](_bmad-output/implementation-artifacts/deferred-work.md) (new "Deferred from: 11-3" section).
  - [x] "View Match History" is the inline subject match-history section (mirrors the self profile; no separate route).

- [x] **Task 5: i18n** (AC: #7)
  - [x] Added to all four locales [en](client/src/shared/i18n/en.json)/[sr](client/src/shared/i18n/sr.json)/[mk](client/src/shared/i18n/mk.json)/[hr](client/src/shared/i18n/hr.json): new top-level `publicProfile` block (`notFound.title/body/cta`, `error`) + `profile.milestones.careerPointsLabel/Hint`. Reused existing `profile.*` keys for shared labels. `mk` all-Cyrillic, no em dash in mk/sr/hr; `i18n.parity.test.ts` green (1:1 parity verified, 0 missing/extra).

- [x] **Task 6: Full validation gates** (AC: #7)
  - [x] All gates green (run via the project's `mise` toolchain — Go 1.26.4, golangci-lint **v1.64.8** matching CI, node 22). **Backend:** `go build`, `go vet ./...`, `go test ./...` (all 18 packages ok, DB-backed handler/repo tests RAN not skipped against dev DB `:5433` — migrated to v18 via golang-migrate), golangci-lint v1.64.8 clean, gofmt clean. **Client:** `tsc -p tsconfig.build.json` clean, `vitest run` 102 files / **1118** tests pass, `eslint .` clean, `prettier --check` clean. (NB: `make lint`/`make test` shell out to `npx`/`golangci-lint` which are not on the bare PATH and the repo's `.golangci.yml` is v1-format vs the mise `latest` v2 binary — equivalent commands run directly with the CI-matching tools instead.)

## Dev Notes

### Design Decisions (READ FIRST)

- **D1 — Same endpoint, dual shape, subject = `:id`.** `GET /users/:id/profile` returns the full self `ProfileResponse` when `:id == authUserID` and a narrower `PublicProfileResponse` otherwise. This matches the epic AC (which names that exact URL) and keeps the self response shape byte-for-byte unchanged. **The non-negotiable correctness rule: swap the subject id from `authUserID` to `paramID` throughout the handler body**, not just delete the 403 — otherwise the viewer's own stats leak under a foreign URL. Same applies to `GetCareer` and `ListMatches`.
- **D2 — XP is exposed publicly, overriding the conservative `ProfileResponse` comment.** The `ProfileResponse` doc comment (from Stories 9.5/9.7) says to keep `TotalXP`/`Level`/XP-breakdown off a public DTO. The **epic AC for 11.3 explicitly lists `level` and `total_xp` as public.** The requirement wins: `PublicProfileResponse` includes `level`, `totalXp`, `xpIntoLevel`, `xpForNextLevel`. `level` is already public elsewhere (room seat tiles), and XP is non-sensitive progression data. Update the stale comment so the next reader isn't misled.
- **D3 — "Total career game points scored" is net-new backend work.** No existing DTO/field/repo method carries a lifetime points total (win/loss come from `GetStatsForUser`; the only points data is per-match team scores + per-hand `BestHand.Points`). Add `GetCareerPointsForUser` + `CareerResponse.careerPoints`. Define precisely (sum of the subject's team score over completed matches) and pin it in a test.
- **D4 — "Add Friend" is deferred to Story 11.2.** There is zero friend backend on `master` (no model, endpoint, or UI). Building a friend request flow here would either be dead/placeholder or drag 11.2's whole scope in. Consistent with the reorder rationale (don't build against non-existent dependencies), this story delivers the public read surface and leaves a documented insertion point; **11.2 adds the button**. The layout-parity AC (AC4) is still met — Add Friend is an additive element, not part of the read layout.
- **D5 — New page, not a dual-mode `ProfilePage`.** Build a separate `PublicPlayerProfilePage` that reuses the already-extracted presentational sub-components. This structurally excludes the self-only side effects rather than gating them: the honor auth-store hydration effect, `LinkedAccounts`, and edit pencils are simply **not mounted**. `ProfilePage` (self) stays untouched. If duplication of the grid layout is significant, optionally extract a shared presentational `ProfileView`, but do not risk leaking self-only effects into the public path.
- **D6 — Season section: nothing to do.** No season/rank data exists (Epic 13 unbuilt); the self profile already renders nothing for it. The public profile inherits that. AC5 is satisfied by not adding a season section; add a test asserting no season DOM so the graceful-absence contract is pinned for when Epic 13 lands.

### Three concrete leak fixes (do not skip)

1. **Honor auth-store hydration** (`ProfilePage.tsx:51-73`) writes the viewed profile's honor into `authStore` — on a public page this would overwrite the *viewer's own* TopBar honor chip. Excluded by using a new page (D5).
2. **IdentityHero wallet/streak pills** are fed from the **viewer's** `authStore` in `ProfilePage` (`user?.walletBalance`, `user?.loginStreakDays`). On a public page they'd show the viewer's private data on someone else's profile. Fix via the new `hidePrivatePills` prop + never passing viewer data.
3. **MatchHistory "YOU" seat chip** (`MatchHistory.tsx:449`, derived from `viewerSeat`) mislabels the subject as "you" for a different viewer. Fix with a subject-aware prop.

### Backend implementation notes

- **Files:** all additions in the existing `user` + `match` packages + `main.go` needs **no route changes** (the three routes already exist; only their self-gate/subject-id change). `PublicProfileResponse` + handler edits in [server/internal/user/handler.go](server/internal/user/handler.go); `GetCareerPointsForUser` in [server/internal/match/repository.go](server/internal/match/repository.go) + its GORM impl; `CareerResponse.careerPoints` wired in `GetCareer`.
- **Auth still required** — `/users/:id/*` stays under the authenticated `api` group (`main.go:134`). "Public" means "not self-restricted," not "unauthenticated."
- **Honor is recomputed from stored weights on every read** (`NewHonorSnapshot`), so it is authoritative for any viewer; just key it on `paramID`. The honor trend is best-effort (a failed windows query degrades to a flat trend — keep that behavior).
- **404 vs 403:** unknown/soft-deleted subject → `apperr.ErrUserNotFound` (404). `FindByID` returns `nil,nil` on not-found (GORM `ErrRecordNotFound` swallowed) — guard for `u == nil`.
- **Envelope/errors:** success `c.JSON(200, map[string]interface{}{"data": <dto>})`; errors via `return apperr.ErrXxx` (central `appErrorHandler` renders `{error:{code,message}}`). No new error code needed (reuse `ErrUserNotFound`, `ErrBadRequest`).

### Frontend implementation notes

- **Stack:** axios wrapper [client/src/shared/api/axiosClient.ts](client/src/shared/api/axiosClient.ts) (base `/api/v1`, `{data}` unwrap, Bearer header, 401 refresh); TanStack Query v5; named exports only; `@/` → `client/src/`; `react-router` `useParams`/`useNavigate` (import from `react-router`, matching `RoomPage`).
- **The api fns/hooks are already id-parametric** — `getProfile(id)`/`getCareer(id)`/`getUserMatches(id, …)` and `useProfileQuery`/`useCareerQuery`/`useUserMatchesInfiniteQuery` all take a plain numeric id. The public page just passes the route `:id`. `getPublicProfile(id)` exists mainly to attach the narrower `PublicProfileResponse` type to the same URL.
- **Reusable presentational components** (all props-driven, confirmed no self-coupling except `IdentityHero`'s optional `userId` edit-switch + the wallet/streak pills fix): `IdentityHero`, `HonorHeroBand` (handles `isNewPlayer` "N / 5" internally), `XpBar`, `StatsGrid`, `MatchHistory`, `PartnerSpotlight`, `Rivalries`, `Milestones`, `WinRateRing`.
- **Query-cache note:** `queryKeys.profile.detail(id)` is keyed by id, so viewing player 2's public profile caches under `["profile", 2]` — distinct from the viewer's own `["profile", selfId]`. Safe. (Only consider a separate key if you worry a future self-visit to `["profile", 2]` could be confused with a public payload — not possible here since a viewer never fetches their own id via the public page.)

### i18n notes

- Locale files `client/src/shared/i18n/{en,sr,mk,hr}.json`; add to all four (`i18n.parity.test.ts` enforces). Reuse the existing `profile.*` block for shared labels; add only genuinely new keys (public not-found, "Career points" label, public page title/eyebrow). `mk` all-Cyrillic (proper nouns like "Beljot" stay Latin); no em dash (`—`) in `mk`/`sr`/`hr` (use `…`/`–`).

### Testing standards summary

- **Go:** `testing` + `testify`; mock-repo + `httptest` `ServeHTTP` handler tests (`mockUserRepo`/`mockMatchRepo` + real JWT via `auth.GenerateAccessToken`, `testErrorHandler`), and DB-backed repo tests via `getTestDB` (per-test tx rollback, DSN dev DB `:5433`, skips if no DB — use for the `GetCareerPointsForUser` aggregate). The existing forbidden/PII/new-player tests are the templates — **rewrite the forbidden ones** (non-self is now 200) and add the public-projection PII-absent assertions.
- **Client:** Vitest + Testing Library; `vi.mock("@/shared/api/profile"|"career"|"matches")`; render with `QueryWrapper` + a router at `/players/:id` (`MemoryRouter` with `initialEntries` or `BrowserRouter`); `makeUser` fixtures; assert by `data-testid` in `waitFor`; present-tense `it(...)`. Crucial new test: honor hydration `setUser` is **NOT** called for a public profile (inverse of `ProfilePage.test.tsx:246-312`).

### Known Traps (project-context + Epic 9 learnings)

- **Interface mock blast radius:** adding `GetCareerPointsForUser` to `MatchRepository` breaks every mock until updated — grep the whole tree, not just the user package.
- **Subject-id swap (D1)** is the single highest-risk correctness bug — tests must seed viewer ≠ subject and assert the subject's data is returned.
- **Never JS-truthiness on Go numerics/bools** (project rule) — honor score 0 is real (`=== 0`, never `|| 80`); the existing `honorScoreOrPrior`/guards must carry over to the public page.
- **Return `[]`/populated structs, not `null`** — match/career lists initialize non-nil.
- **One story = one branch = one PR** — if the career-points aggregate surfaces an unrelated scoring bug, file it in `deferred-work.md`, don't fix it here.
- **`ProfileResponse` frontend type already omits wallet/streak** — the public subset aligns naturally; don't reintroduce them.

### Project Structure Notes

- New files: `client/src/features/profile/PublicPlayerProfilePage.tsx` (+ test), `client/src/shared/hooks/queries/usePublicProfile.ts` (+ optional test). Additions to `server/internal/user/handler.go`, `server/internal/match/repository.go` (+ gorm impl + mocks), `client/src/shared/api/profile.ts` + `career.ts`, `client/src/features/profile/components/IdentityHero.tsx` + `MatchHistory.tsx` (new props), `client/src/App.tsx`, four i18n JSONs. Conforms to feature-folder + `shared/` conventions; frontend api files still map 1:1 to backend domains (profile/career/matches unchanged homes).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-11.3-Public-Player-Profiles] — user story + ACs (FR47), private-field exclusion, New Player rule, layout parity, season graceful-omit.
- [Source: _bmad-output/planning-artifacts/epics.md#Epic-11] — Epic objectives, FRs, Phase 3.
- [Source: _bmad-output/planning-artifacts/prd.md#FR47] — public profiles requirement.
- [Source: _bmad-output/project-context.md] — TS/Go rules, feature-folder structure, i18n rules, testing rules, API response formats, "never JS-truthiness on Go zero values".
- [Source: server/internal/user/handler.go:213-393] — `GetProfile`/`GetCareer` (self-only gates to relax; subject-id swap; assembly of stats/honor/trend), `ProfileResponse`/`CareerResponse` structs + the XP-privacy comment overridden by D2.
- [Source: server/internal/user/handler.go:552-641] — `ListMatches` + `parseMatchesQuery` + `buildMatchListItem` (viewer-relative; must key on `paramID`).
- [Source: server/internal/match/repository.go:72-128] — `GetStatsForUser` (win/loss origin), career aggregate methods, honor-trend windows; site for the new `GetCareerPointsForUser`.
- [Source: server/cmd/api/main.go:134-139] — the three `/users/:id/*` read routes (no route change needed).
- [Source: server/internal/apperr/errors.go] — `ErrUserNotFound`, `ErrForbidden`, `ErrBadRequest`.
- [Source: client/src/features/profile/ProfilePage.tsx] — self composition; honor-hydration side-effect (must be excluded); viewer wallet/streak leak; edit surfaces to drop.
- [Source: client/src/features/profile/components/IdentityHero.tsx + HonorHeroBand.tsx + MatchHistory.tsx] — reusable props; the wallet/streak pills + "YOU" seat-chip fixes.
- [Source: client/src/shared/api/profile.ts + career.ts + matches.ts] — id-parametric api fns + types (`ProfileResponse` already omits wallet/streak).
- [Source: client/src/shared/hooks/queries/useProfile.ts + useCareer.ts + useMatches.ts] — id-scoped query hooks + keys.
- [Source: client/src/App.tsx:86-94] — `AppLayout` protected route group for `/players/:id`; `react-router` `useParams` idiom (RoomPage).
- [Source: client/src/shared/lib/honor.ts] — `HONOR_NEW_PLAYER_MIN_MATCHES = 5`; tier tokens.
- [Source: client/src/shared/i18n/*.json + i18n.parity.test.ts] — locales + parity/quality enforcement.
- [Source: client/src/test-utils.tsx] — `makeUser`, `QueryWrapper`, `TestProviders`.

## Dev Agent Record

### Agent Model Used

Opus 4.8 (`claude-opus-4-8[1m]`) — BMAD dev-story workflow.

### Debug Log References

- Toolchain is off the bare PATH in this environment; ran everything through the repo's `mise` env (`mise env` → Go 1.26.4, node 22, golang-migrate). Dev Postgres started via `docker compose up -d postgres` (host `:5433`) and migrated to v18 with `golang-migrate` so the DB-backed repo/handler tests RAN rather than skipped.
- golangci-lint: the mise `latest` binary is v2.12.2, which rejects the repo's v1-format `server/.golangci.yml`. Installed the CI-matching **v1.64.8** via `go install …@v1.64.8` and ran it clean.

### Completion Notes List

- **Endpoint shape (D1).** `GET /users/:id/profile` is now public: self → full `ProfileResponse` (byte-for-byte unchanged), any other viewer → the narrower `PublicProfileResponse`. The non-negotiable correctness move — swapping the subject from `authUserID` to `subjectID` (= path id) across `GetProfile`, `GetCareer`, `ListMatches` — is pinned by tests that seed viewer ≠ subject and assert the SUBJECT's data (honor 83 vs viewer 86; `lastStatsUserID`/`lastCareerPointsID` == subject; matches viewerSeat/outcome from the subject). Self→public branch compares `paramID` as `uint64` (wraparound-safe, D86).
- **Career points (D3).** New `GetCareerPointsForUser` (interface + GORM impl + 3 mocks) sums the subject's own team score over COMPLETED matches (team via the shared `viewerTeamCase`; abandoned/in-progress excluded). DB-backed test proves a=1000, b=800, none=0 against real SQL.
- **XP made public (D2).** `PublicProfileResponse` carries `level`/`totalXp`/`xpIntoLevel`/`xpForNextLevel`; the stale "keep XP off any public DTO" comment on `ProfileResponse` was corrected.
- **Three leak fixes.** New page (not dual-mode, D5) structurally excludes the honor auth-store hydration effect and `<LinkedAccounts>`; `hidePrivatePills` suppresses the wallet + login-streak pills (never reads the viewer's store for the subject); `subjectIsSelf={false}` stops the "YOU" seat-chip mislabel. Each is pinned by a test, including the inverse-of-ProfilePage assertion that the viewer's store honor is untouched.
- **404 + i18n.** Unknown/soft-deleted subject → 404 `USER_NOT_FOUND` (handler `FindByID` nil-guard) → localized `public-profile-not-found`. New `publicProfile` block + `careerPoints` label in all four locales; parity green, mk all-Cyrillic, no em dash in mk/sr/hr.
- **Add Friend deferred to 11.2 (D4, AC6).** Read surface only; documented insertion point in the page + `deferred-work.md` entry. No friend backend built.
- **Not run:** manual E2E (Playwright) — every gate above is automated; 11.3 touches responsive surfaces (the hero, the sidebar) that automated tests do not judge. Recommend a manual E2E pass in review, as 9.6 found real bugs post-review.

### File List

**Backend (Go)**

- `server/internal/user/handler.go` — `PublicProfileResponse` struct; de-self-gated `GetProfile` (dual-shape, subject-id swap); de-self-gated `GetCareer` (+ `careerPoints`, subject 404 guard) & `ListMatches` (subject-id swap, 404 guard); `CareerResponse.careerPoints`; updated doc comments (D2).
- `server/internal/match/repository.go` — `GetCareerPointsForUser` on the `MatchRepository` interface.
- `server/internal/match/gorm_repo.go` — GORM impl of `GetCareerPointsForUser`.
- `server/internal/user/handler_test.go` — rewrote forbidden/wraparound/auth-failure tests to the public-projection behavior; added public PII-absent, New-Player, unknown-subject-404, foreign-subject career/matches, subject-keying tests; mock `GetCareerPointsForUser`.
- `server/internal/match/gorm_repo_test.go` — DB-backed `TestGormMatchRepository_GetCareerPointsForUser`.
- `server/internal/match/manager_test.go`, `server/internal/match/matchend_test.go` — mock `GetCareerPointsForUser` (interface blast radius).

**Frontend (TypeScript/React)**

- `client/src/shared/api/profile.ts` — `PublicProfileResponse` type + `getPublicProfile(id)`.
- `client/src/shared/api/career.ts` — `careerPoints: number` on `CareerResponse`.
- `client/src/shared/api/queryKeys.ts` — distinct `publicProfile` key namespace.
- `client/src/shared/hooks/queries/usePublicProfile.ts` — new `usePublicProfileQuery`.
- `client/src/features/profile/PublicPlayerProfilePage.tsx` — new public profile page.
- `client/src/features/profile/PublicPlayerProfilePage.test.tsx` — new (10 tests).
- `client/src/features/profile/components/IdentityHero.tsx` — `hidePrivatePills` prop.
- `client/src/features/profile/components/IdentityHero.test.tsx` — 2 `hidePrivatePills` tests.
- `client/src/features/profile/MatchHistory.tsx` — `subjectIsSelf` prop (YOU-chip fix).
- `client/src/features/profile/components/Milestones.tsx` — `careerPoints` prop + row.
- `client/src/features/profile/ProfilePage.tsx` — passes `careerPoints` to Milestones.
- `client/src/features/profile/ProfilePage.test.tsx` — `careerFixture` gains `careerPoints`.
- `client/src/App.tsx` — `/players/:id` route.
- `client/src/shared/i18n/{en,sr,mk,hr}.json` — `publicProfile` block + `careerPoints` labels.

**Docs**

- `_bmad-output/implementation-artifacts/deferred-work.md` — Add-Friend deferral (11.2) note.

## Change Log

| Date       | Version | Description                                                                                             | Author  |
| ---------- | ------- | ------------------------------------------------------------------------------------------------------- | ------- |
| 2026-08-14 | 0.1     | Implemented Story 11.3 (public player profiles): public dual-shape profile/career/matches endpoints, career points aggregate, `PublicPlayerProfilePage` + `/players/:id` route, three leak fixes, i18n ×4. All gates green. Status → review. | Amelia (dev-story) |

## Review Findings

_Adversarial code review (2026-08-14) — Blind Hunter + Edge Case Hunter + Acceptance Auditor, all Opus. **1 decision-needed, 2 patch, 1 defer, 4 dismissed as noise.** The Acceptance Auditor found no AC/D violations; the highest-risk items — the subject-id swap (D1), the private-field whitelist, and the `GetCareerPointsForUser` SQL — were each verified correct against the actual code, not the checkboxes._

### Decision needed

- [x] [Review][Decision] No rate-limiting or caching on the newly-public read endpoints — **RESOLVED 2026-08-14 → DEFERRED.** Reason: auth-gated + indexed lookups; rate-limiting is a platform-wide gap best solved by a dedicated infra story, not scoped into 11.3. _(Detail: lifting the self-gate on `GET /users/:id/{profile,career,matches}` lets any authenticated viewer walk the entire user table, forcing ~5 uncached SQL round-trips per profile hit — `FindByID` + `GetStatsForUser` + honor recompute + honor-trend scan; career adds 4 aggregates incl. the new `GetCareerPointsForUser`. No HTTP rate-limiter or cache exists anywhere in the server today.)_ [server/internal/user/handler.go:254,387,645]

### Patch

- [x] [Review][Patch] **FIXED 2026-08-14 (test-verified).** MatchHistory empty-state addresses the viewer on a foreign profile [client/src/features/profile/MatchHistory.tsx:594] — the `counts.all === 0` branch renders `profile.matchHistory.empty` ("No matches yet — Quick Play to get started") + a `/lobby` CTA unconditionally; `subjectIsSelf` is threaded to the seat chips but not this branch, so a never-played subject's page tells the *viewer* to go play. Fix: branch copy + CTA on `subjectIsSelf`; add a subject-addressed public empty string to all four locales.
- [x] [Review][Patch] **FIXED 2026-08-14 (test-verified).** Frontend id guard uses `Number()` — non-canonical ids alias real players; over-range → generic error not not-found [client/src/features/profile/PublicPlayerProfilePage.tsx:36] — `Number("1e2")=100`, `"0x10"=16`, `" 3 "=3`, `"+5"=5`, `"5.0"=5` all pass `Number.isInteger && > 0`, so `/players/1e2` loads player 100; an over-range id (`1e21`) becomes `/users/1e+21/profile` → server 400 → generic "try again" instead of the not-found surface. Fix: validate the raw param with `/^[1-9][0-9]*$/` and treat a non-safe-integer / a 400 as not-found.

### Deferred

- [x] [Review][Defer] HonorHeroBand "New Player" counter can render "N / 5" with N > 5 [client/src/features/profile/components/HonorHeroBand.tsx:93] — deferred, pre-existing (Story 9.7). The numerator is `completedTotal + abandonedTotal` against the fixed denominator `HONOR_NEW_PLAYER_MIN_MATCHES` (5), so a player with abandonments can display e.g. "7 / 5". Reused verbatim per spec (D-note: "HonorHeroBand handles isNewPlayer 'N / 5' internally"); not introduced by 11.3.

### Manual E2E (Playwright, 2026-08-14) — PASS, closes the dev's "manual E2E not run" caveat

Ran `make dev` (Vite 5173 + Go 8080 + dev Postgres 5433, schema v18) and drove a headless Chromium (playwright-core → system `/usr/bin/chromium`) as a real logged-in viewer. Seeded a viewer(91), a veteran(92, 6 completed matches / honor 91 / level 4), a new-player-with-history(93), and a clean empty user(94) via API + SQL. **23/23 assertions passed; no bugs found; no corrections needed.**

- **Backend contract (live):** public `GET /users/92/profile` returns the narrow projection with **no** email/wallet/loginStreak/languagePreference/usernameChangedAt; self `GET /users/91/profile` still carries them; honor/stats/careerPoints keyed to the **subject** (91 vs 92 seen distinct, viewer-relative stats inverted correctly); `careerPoints=5691`; unknown id → **404 on all three** of profile/career/matches (AC7).
- **Public profile (browser):** veteran page renders honor **91 TRUSTED**, career points **5,691**, 6 matches all labelled `veteranE2E` with **no "You" badge**, **no** wallet/streak pills or edit pencil on the hero, **no** LinkedAccounts, **no** season section — while the **TopBar simultaneously shows the viewer's own** Lvl 0 / 5,000 coins / `viewerE2E` (no viewer-data corruption; `nav-user` identity intact across every navigation).
- **PATCH #2 verified live:** the empty user's match history reads **"This player hasn't played any matches yet."** with **no** "Go to lobby" CTA (subject-addressed, not the viewer's onboarding).
- **PATCH #3 verified live:** `/players/1e2` and `/players/0x10` render the not-found surface and make **no** `/users/*` API call (no aliasing to player 100/16); unknown `/players/999999` → not-found.
- **New Player + self-view:** empty user shows the "0 / 5" New-Player honor state; self via `/players/91` renders read-only (pills suppressed, no edit pencil) without crashing.
- Only console output was the 3 intended 404s from the unknown-id fetch (+favicon). Seed match/room clutter removed afterward; the 4 reusable test accounts left in place (per the 9.8 E2E precedent); dev server stopped.

### Dismissed (recorded, no action)

- **No UI links to `/players/:id`** — by design; Story 11.1 (player search) wires the result-click to this route (spec Task 0). Page is currently reachable only by direct URL.
- **Self-id via `/players/{ownId}` returns the full private payload** into the `["publicProfile", ownId]` cache — it is the viewer's *own* data (not a cross-user leak), never rendered (`hidePrivatePills`, `PublicProfileResponse` type, no `useAuthStore`). Latent-only.
- **`careerPoints` excludes abandoned-wins** while the win counter credits them — intentional and documented in `GetCareerPointsForUser` ("abandoned rows never reach a real score").
- **`userId!` non-null assertion** in `usePublicProfile` query key/fn — guarded by `enabled: userId !== undefined`; matches the established `useCareer`/`useProfile` pattern.
