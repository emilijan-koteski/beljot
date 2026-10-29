import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { BrowserRouter } from "react-router";
import { expect } from "vitest";

import type { RoomInvite } from "@/shared/stores/roomStore";
import type { Room, RoomDetail, RoomPlayer, User } from "@/shared/types/apiTypes";

/**
 * Builds a complete authenticated `User` for tests, defaulted to a plain established player.
 * Pass only the fields the test cares about.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    username: "testuser",
    email: "test@example.com",
    languagePreference: "en",
    cardDeckPreference: "french",
    soundEnabled: true,
    musicEnabled: true,
    soundVolume: 70,
    musicVolume: 70,
    allowNonFriendInvites: true,
    walletBalance: 5000,
    loginStreakDays: 0,
    totalXp: 0,
    level: 0,
    honorScore: 80,
    honorTier: "fair",
    isNewPlayer: false,
    avatarUrl: null,
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

/** Builds a complete, ungated `Room` for tests with every field defaulted. */
// eslint-disable-next-line react-refresh/only-export-components
export function makeRoom(overrides: Partial<Room> = {}): Room {
  return {
    id: 1,
    name: "Test Room",
    code: "XYZ123",
    ownerId: 10,
    ownerUsername: "owner",
    ownerAvatarUrl: null,
    variant: "bitola",
    matchMode: "1001",
    timerStyle: "relaxed",
    timerDurationSeconds: null,
    coinBuyIn: 0,
    isPrivate: false,
    minHonor: 0,
    allowNewPlayers: true,
    status: "waiting",
    playerCount: 0,
    isQuickPlay: false,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

// eslint-disable-next-line react-refresh/only-export-components
export function makeRoomPlayer(overrides: Partial<RoomPlayer> = {}): RoomPlayer {
  return {
    id: 1,
    roomId: 1,
    userId: 1,
    username: "player",
    avatarUrl: null,
    seat: null,
    team: null,
    isBot: false,
    createdAt: "",
    ...overrides,
  };
}

/**
 * Builds the `{room, players, returnedUserIds}` envelope that GET /rooms/:id returns.
 * `returnedUserIds` defaults to every seated player's id, as for a room never in a match.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function makeRoomDetail(overrides: Partial<RoomDetail> = {}): RoomDetail {
  const players = overrides.players ?? [];
  return {
    room: makeRoom(),
    players,
    returnedUserIds: players.map((p) => p.userId),
    ...overrides,
  };
}

// eslint-disable-next-line react-refresh/only-export-components
export function makeRoomInvite(overrides: Partial<RoomInvite> = {}): RoomInvite {
  return {
    inviteId: 1,
    roomId: 1,
    roomName: "Test Room",
    inviterUserId: 20,
    inviterUsername: "inviter",
    coinBuyIn: 0,
    isPrivate: false,
    isHostInvite: false,
    minHonor: 0,
    expiresAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

// eslint-disable-next-line react-refresh/only-export-components
export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
}

/** Wraps children with QueryClientProvider only (use when tests provide their own BrowserRouter) */
export function QueryWrapper({ children }: { children: ReactNode }) {
  const queryClient = createTestQueryClient();
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

/** Wraps children with both QueryClientProvider and BrowserRouter */
export function TestProviders({ children }: { children: ReactNode }) {
  const queryClient = createTestQueryClient();
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  );
}

/**
 * Asserts the action-taking dialog control is immediately followed by the status-quo control in the DOM.
 * Compares positions strictly, since FOLLOWING is also set when one control contains the other.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function expectActionBeforeStatusQuo(actionTestId: string, statusQuoTestId: string) {
  const action = screen.getByTestId(actionTestId);
  const statusQuo = screen.getByTestId(statusQuoTestId);
  expect(
    action.compareDocumentPosition(statusQuo),
    `expected ${actionTestId} to be followed by a sibling ${statusQuoTestId}`,
  ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
}

/**
 * Makes `window.matchMedia` answer min- and max-width queries as a window `width` pixels wide,
 * so a test renders a page's phone, tablet or desktop layout. Returns a function that restores it.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function mockViewportWidth(width: number): () => void {
  const original = window.matchMedia;
  window.matchMedia = (query: string) => {
    const min = /min-width:\s*(\d+)px/.exec(query);
    const max = /max-width:\s*(\d+)px/.exec(query);
    const matches = (!min || width >= Number(min[1])) && (!max || width <= Number(max[1]));
    return {
      matches,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    } as MediaQueryList;
  };
  return () => {
    window.matchMedia = original;
  };
}
