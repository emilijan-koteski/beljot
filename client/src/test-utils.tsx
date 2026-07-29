import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { BrowserRouter } from "react-router";

import type { User } from "@/shared/types/apiTypes";

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
