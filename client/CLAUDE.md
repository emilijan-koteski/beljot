# Client

React 19, TypeScript (strict), Vite 6, Tailwind CSS v4, shadcn/ui, Zustand 5, TanStack Query 5, React Router 7, react-i18next, Zod 4, Vitest 3 with jsdom.

## Commands

```bash
npm run dev                      # Vite dev server (make dev at the root also starts the API)
npx vitest run <path>            # run tests for specific files
npx tsc --noEmit && npx eslint . && npx prettier --check .   # what make lint-client runs
```

## Structure

- `src/features/<feature>/`: pages and their components. New features get their own folder.
- `src/shared/api/`: one client module per backend domain. All HTTP goes through `axiosClient.ts`.
- `src/shared/hooks/queries/` and `hooks/mutations/`: TanStack Query hooks. Query keys come from `shared/api/queryKeys.ts`.
- `src/shared/stores/`: Zustand stores (`authStore`, `matchStore`, `roomStore`, `chatStore`, `levelUpStore`, `rankChangeStore`).
- `src/shared/hooks/useWsDispatch.ts`: the single typed dispatcher for incoming WebSocket frames.
- `src/shared/types/`: `wsEvents.ts` mirrors the server contract, and `wsEvents.schemas.ts` holds its Zod schemas.
- `src/shared/components/ui/`: shadcn components, owned copies.

## Conventions

- Named exports only, never `export default`. The file name matches the exported component.
- Components never call axios or fetch directly. They use `shared/api/` functions through query or mutation hooks.
- `axiosClient.ts` owns the 401, refresh and retry cycle. Call `refreshAccessToken()` and never hit `/auth/refresh` yourself.
- The access token lives in memory in `authStore`, never in `localStorage`. The refresh token is an httpOnly cookie.
- Game components are presentational. They render server state and hold no game logic.
- Store updates are immutable: replace objects, never mutate them.
- Use union literal types, not TypeScript `enum`.
- Tailwind v4 is configured CSS-first in `src/index.css` with `@theme`. There is no `tailwind.config.js`.
- Add shadcn components with `npx shadcn@latest add <component>`. Style them at the call site, and edit `components/ui/` only for project-wide design changes.
- Overlay stacking uses the `Z` layers in `shared/lib/zLayers.ts`. Animation timings live in `shared/lib/motion.ts`.
- Imports are sorted by `simple-import-sort`. Run `npx eslint --fix` instead of ordering them by hand.

## Page layout

- Pages sit in `PageShell` (`shared/components/layout/`): a 1320px cap with 16, 24 and 28px gutters, an optional 300px desktop sidebar and an optional phone bottom bar.
- Switch structure per width with `useLayoutBand()` (phone below 768px, tablet below 1024px, desktop above). Use Tailwind's `md:` and `lg:` for styling alone. Tests pick a width with `mockViewportWidth` from `src/test-utils.tsx`.
- Build from the shared parts before writing new ones: `ActionTile`, `Panel`, `DetailRows`, `controlVariants` (32px round or labelled controls), `FilterBar`, `SortMenu` and `RoomRulesPopover`/`MatchRulesPopover`. Take every rule and room-setting icon from `RULE_ICONS` in `shared/components/roomRules/roomRuleLines.ts`.
- Room seats count bots: use `roomOccupancy` and `isRoomFull` from `shared/lib/roomOccupancy.ts`, never `room.playerCount` alone.

## Testing

- Tests sit next to their source as `*.test.ts(x)`, using Vitest and Testing Library.
- Select elements by role or `data-testid`, never by CSS class.
- Test names are present tense: `it("renders the error when login fails")`.
- Shared render helpers live in `src/test-utils.tsx`, and global setup in `src/test-setup.ts`.
