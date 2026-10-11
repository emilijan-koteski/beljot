---
title: "Fix auth routing: eliminate stale refresh calls and add route guards"
type: "bugfix"
created: "2026-04-16"
status: "done"
baseline_commit: "ae13e9c"
context:
  - "_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Three auth routing defects: (1) `useAuthInit()` fires `refresh()` on every app mount — including the login page where no session exists, producing two wasted 401 calls in StrictMode; (2) authenticated users can freely navigate to `/login` and `/register`; (3) navigating to `/` (or any unknown path) redirects to `/login` unconditionally, which feels like a logout for authenticated users.

**Approach:** Skip the refresh call on guest pages (login/register), add a `GuestRoute` guard that redirects authenticated users to `/lobby`, and make the root `/` and catch-all routes auth-aware.

## Boundaries & Constraints

**Always:** `fetchClient.ts` 401-interceptor refresh cycle stays untouched — only the init-time refresh in `useAuthInit` changes. `ProtectedRoute` remains the single guard for authenticated routes. `useReconnectionRedirect` handles game-in-progress redirection once inside the protected area.

**Ask First:** If the UX should show a toast/message when redirecting an authenticated user away from `/login`.

**Never:** No changes to backend auth endpoints. No token persistence changes (stays in-memory Zustand). No new API calls.

## I/O & Edge-Case Matrix

| Scenario                             | Input / State                            | Expected Output / Behavior                                             | Error Handling                                 |
| ------------------------------------ | ---------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------- |
| Unauthenticated lands on `/login`    | No token, no refresh cookie              | Render login page, zero `/auth/refresh` calls                          | N/A                                            |
| Unauthenticated lands on `/register` | No token, no refresh cookie              | Render register page, zero refresh calls                               | N/A                                            |
| Returning user lands on `/lobby`     | No in-memory token, valid refresh cookie | `useAuthInit` calls refresh once, restores session, renders lobby      | Refresh fails → clear state, redirect to login |
| Authenticated navigates to `/login`  | Valid token in store                     | Redirect to `/lobby` immediately                                       | N/A                                            |
| Authenticated navigates to `/`       | Valid token in store                     | Redirect to `/lobby` (then `useReconnectionRedirect` may push to game) | N/A                                            |
| Unauthenticated navigates to `/`     | No token                                 | Redirect to `/login`                                                   | N/A                                            |
| Authenticated hits unknown path      | Valid token                              | Redirect to `/lobby`                                                   | N/A                                            |
| Unauthenticated hits unknown path    | No token                                 | Redirect to `/login`                                                   | N/A                                            |

</frozen-after-approval>

## Code Map

- `client/src/App.tsx` -- Route definitions, `useAuthInit` call site, catch-all route
- `client/src/shared/hooks/useAuth.ts` -- `useAuthInit()` hook that unconditionally calls `refresh()`
- `client/src/shared/components/ProtectedRoute.tsx` -- Existing guard for authenticated routes
- `client/src/shared/components/GuestRoute.tsx` -- **NEW** inverse guard for login/register
- `client/src/shared/hooks/useReconnectionRedirect.ts` -- Existing hook that redirects to active game

## Tasks & Acceptance

**Execution:**

- [x] `client/src/shared/hooks/useAuth.ts` -- Add location check: if pathname is `/login` or `/register`, skip `refresh()` and set `isLoading = false` immediately -- eliminates wasted refresh calls on guest pages
- [x] `client/src/shared/components/GuestRoute.tsx` -- Create component: if `token` exists, `<Navigate to="/lobby" replace />`; otherwise render `<Outlet />` -- prevents authenticated users from reaching auth pages
- [x] `client/src/App.tsx` -- Wrap `/login` and `/register` in `<GuestRoute />`; add explicit `<Route path="/" ...>` that redirects based on auth state; update catch-all to also be auth-aware -- fixes root route and unknown path handling

**Acceptance Criteria:**

- Given an unauthenticated user, when they open `/login`, then zero `/auth/refresh` network requests are made
- Given an authenticated user, when they navigate to `/login` or `/register`, then they are redirected to `/lobby`
- Given an authenticated user, when they navigate to `/`, then they are redirected to `/lobby` (not logged out)
- Given an unauthenticated user, when they navigate to `/`, then they are redirected to `/login`
- Given an authenticated user with an active game, when they navigate to `/`, then they reach `/lobby` and `useReconnectionRedirect` pushes them to the game page

## Spec Change Log

## Verification

**Commands:**

- `cd client && npx vitest run` -- expected: all existing tests pass
- `cd client && npx eslint src/` -- expected: no lint errors

**Manual checks:**

- Open browser DevTools Network tab, navigate to `/login` — confirm zero `/auth/refresh` requests
- Log in, then manually navigate to `/login` — confirm redirect to `/lobby`
- Log in, navigate to `/` — confirm redirect to `/lobby`, not logout

## Suggested Review Order

**Auth init — skip refresh on guest pages**

- `GUEST_PATHS` guard skips `refresh()` when pathname is `/login` or `/register`
  [`useAuth.ts:9`](../../../client/src/shared/hooks/useAuth.ts#L9)

- Early return with `setLoading(false)` so the app doesn't hang on a loading spinner
  [`useAuth.ts:29`](../../../client/src/shared/hooks/useAuth.ts#L29)

**Route guards — GuestRoute + auth-aware catch-all**

- New `GuestRoute` mirrors `ProtectedRoute` — redirects authenticated users to `/lobby`
  [`GuestRoute.tsx:5`](../../../client/src/shared/components/GuestRoute.tsx#L5)

- `AuthAwareRedirect` replaces the old unconditional `/login` catch-all
  [`App.tsx:18`](../../../client/src/App.tsx#L18)

- `/login` and `/register` wrapped in `GuestRoute`; `*` uses `AuthAwareRedirect`
  [`App.tsx:34`](../../../client/src/App.tsx#L34)

**Tests**

- Three branch tests for `GuestRoute` (no token, token present, loading)
  [`GuestRoute.test.tsx:24`](../../../client/src/shared/components/GuestRoute.test.tsx#L24)
