export const queryKeys = {
  rooms: {
    all: ["rooms"] as const,
    list: (status: string) => ["rooms", "list", status] as const,
    detail: (id: number) => ["rooms", "detail", id] as const,
    byCode: (code: string) => ["rooms", "byCode", code] as const,
  },
  profile: {
    detail: (userId: number) => ["profile", userId] as const,
  },
  // Story 11.3: a DISTINCT namespace from `profile`, because GET
  // /users/:id/profile returns a narrower PublicProfileResponse for a non-self
  // id. Keying public reads under their own root means the self full-shape
  // entry and a public narrow-shape entry can never collide for the same id.
  publicProfile: {
    detail: (userId: number) => ["publicProfile", userId] as const,
  },
  career: {
    detail: (userId: number) => ["career", userId] as const,
  },
  identities: {
    detail: (userId: number) => ["identities", userId] as const,
  },
  matches: {
    byUser: (userId: number, outcome: string, sort: string) =>
      ["matches", "byUser", userId, outcome, sort] as const,
  },
  // Story 11.1: player search. The query string is part of the key so each
  // distinct (debounced) term is its own cache entry.
  users: {
    search: (query: string) => ["users", "search", query] as const,
  },
  lobby: {
    stats: ["lobby", "stats"] as const,
  },
  stats: {
    public: ["stats", "public"] as const,
  },
} as const;
