import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useAuthStore } from "@/shared/stores/authStore";
import type { WsMessage } from "@/shared/types/wsEvents";

import type { WsConnectionState } from "./useWebSocket";
import { useWebSocket } from "./useWebSocket";

class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = MockWebSocket.CONNECTING;
  onopen: ((ev: Event) => void) | null = null;
  onclose: ((ev: CloseEvent) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;

  sent: string[] = [];

  url: string;

  constructor(url: string) {
    this.url = url;
    mockInstances.push(this);
  }

  send(data: string) {
    this.sent.push(data);
  }

  close(_code?: number, _reason?: string) {
    this.readyState = MockWebSocket.CLOSED;
    if (this.onclose) {
      this.onclose(new CloseEvent("close", { code: 1000 }));
    }
  }

  simulateOpen() {
    this.readyState = MockWebSocket.OPEN;
    if (this.onopen) {
      this.onopen(new Event("open"));
    }
  }

  simulateMessage(data: unknown) {
    if (this.onmessage) {
      this.onmessage(new MessageEvent("message", { data: JSON.stringify(data) }));
    }
  }

  simulateClose() {
    this.readyState = MockWebSocket.CLOSED;
    if (this.onclose) {
      this.onclose(new CloseEvent("close", { code: 1006 }));
    }
  }
}

let mockInstances: MockWebSocket[] = [];

beforeEach(() => {
  mockInstances = [];
  vi.useFakeTimers();
  vi.stubGlobal("WebSocket", MockWebSocket);
  useAuthStore.setState({ token: "test-jwt-token", user: null, isLoading: false });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
  useAuthStore.setState({ token: null, user: null, isLoading: false });
});

describe("useWebSocket", () => {
  it("transitions to connecting state on mount", () => {
    const onMessage = vi.fn();
    const { result } = renderHook(() => useWebSocket({ onMessage }));

    act(() => vi.advanceTimersByTime(0));

    expect(result.current.state).toBe("connecting" as WsConnectionState);
    expect(mockInstances).toHaveLength(1);
  });

  it("sends auth message on connection open", () => {
    const onMessage = vi.fn();
    renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    const ws = mockInstances[0]!;
    act(() => ws.simulateOpen());

    expect(ws.sent).toHaveLength(1);
    const sent = JSON.parse(ws.sent[0]!);
    expect(sent.type).toBe("action:authenticate");
    expect(sent.payload.token).toBe("test-jwt-token");
  });

  it("transitions to connected on auth success", () => {
    const onMessage = vi.fn();
    const { result } = renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    const ws = mockInstances[0]!;
    act(() => ws.simulateOpen());
    act(() =>
      ws.simulateMessage({
        type: "system:authenticated",
        payload: { userId: 42 },
      }),
    );

    expect(result.current.state).toBe("connected" as WsConnectionState);
  });

  it("forwards non-auth messages to onMessage callback", () => {
    const onMessage = vi.fn();
    renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    const ws = mockInstances[0]!;
    act(() => ws.simulateOpen());
    act(() =>
      ws.simulateMessage({
        type: "system:authenticated",
        payload: { userId: 42 },
      }),
    );

    act(() =>
      ws.simulateMessage({
        type: "event:card_played",
        payload: { cardId: "KS" },
      }),
    );

    const calls = onMessage.mock.calls.map((c) => (c[0] as WsMessage).type);
    expect(calls).toContain("event:card_played");
  });

  it("transitions to disconnected on connection close", () => {
    const onMessage = vi.fn();
    const { result } = renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    const ws = mockInstances[0]!;
    act(() => ws.simulateOpen());
    act(() =>
      ws.simulateMessage({
        type: "system:authenticated",
        payload: { userId: 42 },
      }),
    );
    expect(result.current.state).toBe("connected" as WsConnectionState);

    act(() => ws.simulateClose());
    expect(result.current.state).toBe("disconnected" as WsConnectionState);
  });

  it("attempts reconnection after disconnect", () => {
    const onMessage = vi.fn();
    renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    const ws = mockInstances[0]!;
    act(() => ws.simulateOpen());
    act(() =>
      ws.simulateMessage({
        type: "system:authenticated",
        payload: { userId: 42 },
      }),
    );

    act(() => ws.simulateClose());
    expect(mockInstances).toHaveLength(1);

    act(() => vi.advanceTimersByTime(1100));
    expect(mockInstances).toHaveLength(2);
  });

  it("sends messages when connected", () => {
    const onMessage = vi.fn();
    const { result } = renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    const ws = mockInstances[0]!;
    act(() => ws.simulateOpen());
    act(() =>
      ws.simulateMessage({
        type: "system:authenticated",
        payload: { userId: 42 },
      }),
    );

    act(() => result.current.sendMessage("action:play_card", { cardId: "KS" }));

    expect(ws.sent).toHaveLength(2);
    const sent = JSON.parse(ws.sent[1]!);
    expect(sent.type).toBe("action:play_card");
    expect(sent.payload.cardId).toBe("KS");
  });

  it("does not connect when no auth token", () => {
    useAuthStore.setState({ token: null });
    const onMessage = vi.fn();
    const { result } = renderHook(() => useWebSocket({ onMessage }));
    act(() => vi.advanceTimersByTime(0));

    expect(result.current.state).toBe("disconnected" as WsConnectionState);
    expect(mockInstances).toHaveLength(0);
  });
});
