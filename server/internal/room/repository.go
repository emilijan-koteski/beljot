package room

type RoomRepository interface {
	Create(room *Room) error
	Update(room *Room) error
	FindByID(id uint) (*Room, error)
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
	FindQuickPlayRoom(buyIn int) (*Room, error)
	FindQuickPlayRoomExcluding(excludedRoomIDs map[uint]bool, buyIn int) (*Room, error)
	UpdateStatus(roomID uint, status string) error
	FindUserIDsByRoomStatus(status string) ([]uint, error)
	LoadOwnerUsernames(rooms []*Room) error
	FindUserNameAndLanguage(userID uint) (username string, language string, err error)
	FindPlayersByRoomIDs(roomIDs []uint) (map[uint][]RoomPlayer, error)
	AddBot(roomID uint, seat int) error
	RemoveBot(roomID uint, seat int) error
	UpdateBotSeat(roomID uint, fromSeat, toSeat int) error
	FindBotsByRoomID(roomID uint) ([]RoomBot, error)
	FindBotsByRoomIDs(roomIDs []uint) (map[uint][]RoomBot, error)
	RunInTransaction(fn func(RoomRepository) error) error
}
