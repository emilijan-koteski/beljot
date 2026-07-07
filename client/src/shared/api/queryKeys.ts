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
  lobby: {
    stats: ["lobby", "stats"] as const,
  },
  stats: {
    public: ["stats", "public"] as const,
  },
} as const;
