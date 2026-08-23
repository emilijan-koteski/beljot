export const queryKeys = {
  rooms: {
    all: ["rooms"] as const,
    list: (status: string) => ["rooms", "list", status] as const,
    detail: (id: number) => ["rooms", "detail", id] as const,
    byCode: (code: string) => ["rooms", "byCode", code] as const,
    // Story 11.5: the invite panel's friend roster for one room. Keyed by room
    // because availability is answered in that room's context (a full room
    // disables every row).
    invitableFriends: (roomId: number) => ["rooms", "invitableFriends", roomId] as const,
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
    // The room's single most recent match. Keyed by room, not by user: the
    // response is viewer-relative but the viewer is the authenticated caller,
    // and the cache is per-session anyway.
    //
    // Because the key is per-ROOM and not per-match, an entry populated in the
    // room lobby describes match N-1 the instant match N starts. `lastByRoomAll`
    // is the prefix the match_end handler removes so nothing can serve that
    // stale row to the end-of-match overlay.
    lastByRoom: (roomId: number) => ["matches", "lastByRoom", roomId] as const,
    lastByRoomAll: () => ["matches", "lastByRoom"] as const,
  },
  // Story 11.1: player search. The query string is part of the key so each
  // distinct (debounced) term is its own cache entry.
  users: {
    search: (query: string) => ["users", "search", query] as const,
  },
  // Story 11.2: friends. `list` and `requests` are the viewer's own
  // collections; `status` is keyed per subject id — the public-profile
  // friendship button. The WS system:friend_request push invalidates
  // `requests()`.
  friends: {
    list: () => ["friends", "list"] as const,
    requests: () => ["friends", "requests"] as const,
    status: (id: number) => ["friends", "status", id] as const,
  },
  lobby: {
    stats: ["lobby", "stats"] as const,
  },
  stats: {
    public: ["stats", "public"] as const,
  },
} as const;
