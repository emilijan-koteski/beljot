/** Returns the Vite-injected build version, or "dev" where `define` is absent (vitest). */
function injectedVersion(): string {
  return typeof __APP_VERSION__ !== "undefined" ? __APP_VERSION__ : "dev";
}

export const APP_VERSION: string = injectedVersion();

// Seam around window.location.reload() — jsdom's location is non-configurable,
// so tests stub this module instead.
export function reloadForNewVersion(): void {
  window.location.reload();
}
