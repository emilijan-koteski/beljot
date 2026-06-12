// API response types — keep in sync with server models

export interface ApiResponse<T> {
  data: T;
}

export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

export interface User {
  id: number;
  username: string;
  email: string;
  languagePreference: string;
  createdAt: string;
}

export interface Room {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  /**
   * Display username of the room's owner, hydrated by the server via a JOIN
   * to the `users` table at response time. Lets the lobby card render a host
   * avatar without an extra round-trip per row.
   */
  ownerUsername: string;
  /**
   * Embedded players, populated only by the GET /rooms list endpoint so the
   * lobby grid can render seat chips inline. The detail endpoint
   * (GET /rooms/:id) keeps its own `{room, players}` envelope and leaves
   * this field undefined on the inner room.
   */
  players?: RoomPlayer[];
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
  isQuickPlay: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateRoomRequest {
  name: string;
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
}

export interface RoomPlayer {
  id: number;
  roomId: number;
  userId: number;
  username: string;
  seat: number | null;
  team: string | null;
  // Synthetic bot entries arrive as {id:0, userId:0, username:"", isBot:true}.
  // Always check with `isBot === true` — never infer from a falsy userId.
  isBot: boolean;
  createdAt: string;
}

export interface RoomDetail {
  room: Room;
  players: RoomPlayer[];
}

export interface SelectSeatResponse {
  players: RoomPlayer[];
  matchStarted: boolean;
}

export interface QuickPlayResponse {
  room: Room;
  seat: number;
  matchStarted: boolean;
}
