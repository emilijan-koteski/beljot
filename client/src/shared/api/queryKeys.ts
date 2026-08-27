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
  // Story 13.1: the viewer's own standing in the active season. NOT keyed by
  // user id — the endpoint answers for the JWT subject only, so a per-id key
  // would imply a lookup the API does not offer. Invalidated by the
  // event:season_points_awarded WS handler rather than polled.
  season: {
    current: () => ["season", "current"] as const,
    // Story 13.3: the seasons list (the leaderboard picker's feed) and a
    // player's prior-season archive. The archive is keyed per subject id — the
    // endpoint takes one, unlike `current` above.
    list: () => ["season", "list"] as const,
    archive: (userId: number) => ["season", "archive", userId] as const,
    // Story 13.2: the seasonal leaderboard, keyed by PAGE SIZE and (since
    // Story 13.3) the SEASON SELECTOR — a season id, or "current" for the
    // active window — so a prior season's frozen standings and the live
    // ladder are separate cache entries.
    //
    // No `offset` in the key, deliberately. Neither consumer needs one: the
    // lobby widget only ever reads offset 0, and the full page is a single
    // useInfiniteQuery whose pages live inside ONE cache entry with the offsets
    // as page params — an offset in the key would fragment that entry per page
    // and defeat the point. It previously took an `offset` argument that both
    // callers passed as 0, which described a cache layout the code did not have.
    //
    // `limit` does separate entries, so two surfaces asking for differently
    // sized pages cannot overwrite each other's lists.
    //
    // NOT invalidated from the socket — standings are pull-only (page load plus
    // poll), by epic decision. The ONE non-pull invalidation is the season
    // rollover: `useSeasonWindowWatch` invalidates `leaderboardAll()` (this
    // whole prefix) at the boundary so no surface keeps rendering the dead
    // window's ladder as current.
    leaderboard: (limit: number, season: number | "current") =>
      ["season", "leaderboard", limit, season] as const,
    leaderboardAll: () => ["season", "leaderboard"] as const,
  },
  stats: {
    public: ["stats", "public"] as const,
  },
} as const;
