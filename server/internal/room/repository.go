package room

type RoomRepository interface {
	Create(room *Room) error
	Update(room *Room) error
	FindByID(id uint) (*Room, error)
	// FindByIDForUpdate is the row-locking variant for use INSIDE a transaction
	// where the caller intends to mutate the room's status or player_count and
	// must serialize against concurrent writers (auto-start vs leave race,
	// AC3). The lock is released when the surrounding tx commits or rolls
	// back. Calling outside a tx degrades to FindByID semantics.
	FindByIDForUpdate(id uint) (*Room, error)
	FindByCode(code string) (*Room, error)
	FindByStatus(status string) ([]Room, error)
	AddPlayer(roomPlayer *RoomPlayer) error
	RemovePlayer(roomID uint, userID uint) error
	FindPlayersByRoomID(roomID uint) ([]RoomPlayer, error)
	FindPlayerRoom(userID uint) (*RoomPlayer, error)
	IncrementPlayerCount(roomID uint) error
	DecrementPlayerCount(roomID uint) error
	UpdatePlayerSeat(roomID uint, userID uint, seat int, team string) error
	ClearPlayerSeat(roomID uint, userID uint) error
	FindPlayerBySeat(roomID uint, seat int) (*RoomPlayer, error)
	// FindQuickPlayRoom finds the oldest waiting quick-play room in the given
	// affordability bracket (buyIn ∈ {0, 500}, Story 9.4). buyIn keys the pool:
	// players only ever match into a room whose coin_buy_in equals their own
	// bracket, keeping the two pools strictly separate.
	FindQuickPlayRoom(buyIn int) (*Room, error)
	// FindQuickPlayRoomExcluding skips room IDs already attempted in the
	// current retry loop. AC5: when counter drift traps the loop on the same
	// drifted room, exclusion lets FindQuickPlayRoom return a different room
	// or fall through to the create-new-room branch. buyIn (Story 9.4) filters
	// to the caller's affordability bracket.
	FindQuickPlayRoomExcluding(excludedRoomIDs map[uint]bool, buyIn int) (*Room, error)
	UpdateStatus(roomID uint, status string) error
	// FindUserIDsByRoomStatus returns the user IDs of every player currently
	// seated in a room whose status matches the provided value. Used by lobby
	// stats to bucket connected users into "in waiting room" vs "in game".
	FindUserIDsByRoomStatus(status string) ([]uint, error)
	// LoadOwnerUsernames populates each room's transient OwnerUsername field
	// via a single SELECT against the users table (IN clause). The handler
	// calls this before serializing room responses + WS broadcast payloads so
	// the lobby grid can show host avatars without N+1 round-trips per row.
	LoadOwnerUsernames(rooms []*Room) error
	// FindPlayersByRoomIDs returns a map of roomID → players for the given
	// rooms via a single JOIN against users. Used by the lobby grid so each
	// room card can render its 2×2 seat chips inline without an extra fetch
	// per visible row.
	FindPlayersByRoomIDs(roomIDs []uint) (map[uint][]RoomPlayer, error)
	// AddBot seats a bot in the room. Returns apperr.ErrSeatTaken when the
	// (room_id, seat) unique index rejects a concurrent double-add.
	AddBot(roomID uint, seat int) error
	// RemoveBot unseats the bot at the given seat. Returns
	// apperr.ErrNoBotOnSeat when no bot occupies that seat.
	RemoveBot(roomID uint, seat int) error
	// UpdateBotSeat moves a bot between seats (owner swap/move flows).
	// Returns apperr.ErrNoBotOnSeat when no bot occupies fromSeat.
	UpdateBotSeat(roomID uint, fromSeat, toSeat int) error
	FindBotsByRoomID(roomID uint) ([]RoomBot, error)
	// FindBotsByRoomIDs returns a map of roomID → bots for the given rooms in
	// a single query. Mirrors FindPlayersByRoomIDs for lobby previews.
	FindBotsByRoomIDs(roomIDs []uint) (map[uint][]RoomBot, error)
	RunInTransaction(fn func(RoomRepository) error) error
}
