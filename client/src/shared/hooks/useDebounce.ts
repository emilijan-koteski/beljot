import { useEffect, useState } from "react";

/**
 * Returns a debounced copy of `value` that only updates after `delayMs` have
 * elapsed without a change. Used to throttle live-search input so a network
 * request fires once the player stops typing rather than on every keystroke
 * (Story 11.1). Generic so it works for any value type.
 *
 * The timer is reset on each `value` change and cleared on unmount, so a
 * pending update never lands after the component is gone.
 */
export function useDebounce<T>(value: T, delayMs = 250): T {
  const [debounced, setDebounced] = useState<T>(value);

  useEffect(() => {
    const handle = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(handle);
  }, [value, delayMs]);

  return debounced;
}
