export const queryKeys = {
  rooms: {
    all: ["rooms"] as const,
    list: (status: string) => ["rooms", "list", status] as const,
    detail: (id: number) => ["rooms", "detail", id] as const,
    byCode: (code: string) => ["rooms", "byCode", code] as const,
    invitableFriends: (roomId: number) => ["rooms", "invitableFriends", roomId] as const,
    invitablePlayers: (roomId: number, search: string) =>
      ["rooms", "invitablePlayers", roomId, search] as const,
    invitablePlayersAll: (roomId: number) => ["rooms", "invitablePlayers", roomId] as const,
  },
  profile: {
    detail: (userId: number) => ["profile", userId] as const,
  },
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
    lastByRoom: (roomId: number) => ["matches", "lastByRoom", roomId] as const,
    lastByRoomAll: () => ["matches", "lastByRoom"] as const,
  },
  users: {
    search: (query: string) => ["users", "search", query] as const,
  },
  friends: {
    list: () => ["friends", "list"] as const,
    requests: () => ["friends", "requests"] as const,
    status: (id: number) => ["friends", "status", id] as const,
  },
  lobby: {
    stats: ["lobby", "stats"] as const,
  },
  season: {
    current: () => ["season", "current"] as const,
    list: () => ["season", "list"] as const,
    archive: (userId: number) => ["season", "archive", userId] as const,
    leaderboard: (limit: number, season: number | "current") =>
      ["season", "leaderboard", limit, season] as const,
    leaderboardAll: () => ["season", "leaderboard"] as const,
  },
  stats: {
    public: ["stats", "public"] as const,
  },
} as const;
