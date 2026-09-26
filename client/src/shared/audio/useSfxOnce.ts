import { useEffect } from "react";

import { playSfx, type SfxName } from "./audioEngine";

const payloadKeys = new WeakMap<object, string>();
let nextPayloadKey = 0;

/**
 * A dedupe key that names one event payload. The same object — a reveal the
 * store still holds when the page remounts — always gets the same key, so its
 * sound is not heard twice; every new payload gets a key no earlier one ever
 * had, so a later match or hand can never be silenced by an old key that
 * happens to look the same.
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
