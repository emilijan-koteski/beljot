/// <reference types="vite/client" />

/** Build identity injected by Vite `define`; undefined under vitest, so read it via shared/lib/appVersion.ts. */
declare const __APP_VERSION__: string;

/** Vite env vars; the Google button is hidden when VITE_GOOGLE_CLIENT_ID is unset or empty. */
interface ImportMetaEnv {
  readonly VITE_GOOGLE_CLIENT_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
