import "@testing-library/jest-dom/vitest";

import { transferableAbortController } from "node:util";

if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function scrollIntoView() {};
}

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

/** Mirrors a jsdom AbortSignal onto a native one, since Node's fetch Request accepts only its own signals. */
function toNativeSignal(signal: AbortSignal): AbortSignal {
  const controller = transferableAbortController();
  if (signal.aborted) {
    controller.abort(signal.reason);
  } else {
    signal.addEventListener("abort", () => controller.abort(signal.reason), { once: true });
  }
  return controller.signal;
}

/** Node's Request, taking the jsdom signals React Router passes on every navigation. */
class NativeSignalRequest extends Request {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    super(input, init?.signal ? { ...init, signal: toNativeSignal(init.signal) } : init);
  }
}

globalThis.Request = NativeSignalRequest;
