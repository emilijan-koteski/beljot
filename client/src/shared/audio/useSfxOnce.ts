import { useEffect } from "react";

import { playSfx, type SfxName } from "./audioEngine";

const payloadKeys = new WeakMap<object, string>();
let nextPayloadKey = 0;

/**
 * Returns a dedupe key unique to this payload object, stable across remounts.
 * A new payload never reuses an older payload's key, so it is never silenced.
 */
export function payloadSoundKey(payload: object): string {
  let key = payloadKeys.get(payload);
  if (key === undefined) {
    nextPayloadKey += 1;
    key = `payload-${nextPayloadKey}`;
    payloadKeys.set(payload, key);
  }
  return key;
}

/**
 * Play `name` once, when the calling surface mounts (or its key changes).
 * A null/undefined key is silent: callers pass null for a surface rebuilt
 * from a resync rather than shown for the moment it describes.
 */
export function useSfxOnce(name: SfxName, dedupeKey: string | null | undefined): void {
  useEffect(() => {
    if (dedupeKey === null || dedupeKey === undefined) return;
    playSfx(name, { dedupeKey });
  }, [name, dedupeKey]);
}
