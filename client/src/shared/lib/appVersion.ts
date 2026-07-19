// __APP_VERSION__ is injected by Vite `define` (vite.config.ts): the commit
// SHA in CI images, "dev" locally. The `typeof` guard matters — vitest runs
// off vitest.config.ts, which has no `define`, so the identifier doesn't
// exist there at all.
export const APP_VERSION: string = typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "dev";

// Seam around window.location.reload() — jsdom's location is non-configurable,
// so tests stub this module instead.
export function reloadForNewVersion(): void {
  window.location.reload();
}
