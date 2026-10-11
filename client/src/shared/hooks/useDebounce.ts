import { useEffect, useState } from "react";

/**
 * Returns a copy of `value` that only updates after `delayMs` have elapsed without a change.
 * The timer resets on each change and is cleared on unmount.
 */
export function useDebounce<T>(value: T, delayMs = 250): T {
  const [debounced, setDebounced] = useState<T>(value);

  useEffect(() => {
    const handle = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(handle);
  }, [value, delayMs]);

  return debounced;
}
