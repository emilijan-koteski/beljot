import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { BrowserRouter } from "react-router";
import { expect } from "vitest";

import type { RoomInvite } from "@/shared/stores/roomStore";
import type { Room, RoomDetail, RoomPlayer, User } from "@/shared/types/apiTypes";

/**
 * A complete authenticated `User` for tests, with every field defaulted to a
 * plain established-player value. Pass only what the test actually cares about.
 *
 * This exists because `User` grows: Story 9.5 added totalXp/level, 9.6 added
 * room fields, 9.7 added the honor trio, and each time a dozen inline fixtures
 * broke at once. Route new fixtures through here so the NEXT additive field is
 * a one-line change (the profileFixture() precedent from Story 7-2).
 */
// eslint-disable-next-line react-refresh/only-export-components
export function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    username: "testuser",
    email: "test@example.com",
    languagePreference: "en",
    cardDeckPreference: "french",
    walletBalance: 5000,
    loginStreakDays: 0,
    totalXp: 0,
    level: 0,
    honorScore: 80,
    honorTier: "fair",
    isNewPlayer: false,
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

/**
 * A complete `Room` for tests. Same reasoning as `makeUser`, and the same
 * history: Story 9.2 added coinBuyIn, 9.6 added isPrivate, 9.8 added
 * minHonor/allowNewPlayers, and each addition silently broke a pile of inline
 * literals that nothing type-checked (see below). Build rooms through here.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function makeRoom(overrides: Partial<Room> = {}): Room {
  return {
    id: 1,
    name: "Test Room",
    code: "XYZ123",
    ownerId: 10,
    ownerUsername: "owner",
    variant: "bitola",
    matchMode: "1001",
    timerStyle: "relaxed",
    timerDurationSeconds: null,
    coinBuyIn: 0,
    isPrivate: false,
    // Ungated by default (Story 9.8).
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
    seat: null,
    team: null,
    isBot: false,
    createdAt: "",
    ...overrides,
  };
}

/**
 * The `{room, players, returnedUserIds}` envelope GET /rooms/:id returns.
 * `returnedUserIds` defaults to every seated player's id, which is what a room
 * that was never in a match looks like — the presence layer only diverges from
 * that after a match ends (see D145 / return-to-room v2).
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
 * Asserts the app-wide dialog footer convention: the action-taking control
 * precedes the status-quo control in the DOM, which is what puts it on the left
 * of a `justify-end` row (and on top of a narrow-screen stack).
 *
 * Takes test ids rather than elements so a failure names the pair that broke.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function expectActionBeforeStatusQuo(actionTestId: string, statusQuoTestId: string) {
  const action = screen.getByTestId(actionTestId);
  const statusQuo = screen.getByTestId(statusQuoTestId);
  // Strict equality, not a bitwise AND: FOLLOWING is also set for
  // CONTAINED_BY (4|16) and for disconnected trees, so the AND form would pass
  // if the status-quo control ever ended up nested inside the action one. A
  // plain following sibling — including one wrapped in ButtonTimerRing — is
  // exactly FOLLOWING.
  expect(
    action.compareDocumentPosition(statusQuo),
    `expected ${actionTestId} to be followed by a sibling ${statusQuoTestId}`,
  ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
}
