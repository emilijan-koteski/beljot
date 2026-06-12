package room

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// roomPlayerRow is a scan-target struct without gorm:"-" on Username,
// allowing GORM's Scan to populate the username from a JOIN query.
type roomPlayerRow struct {
	ID        uint      `gorm:"column:id"`
	RoomID    uint      `gorm:"column:room_id"`
	UserID    uint      `gorm:"column:user_id"`
	Username  string    `gorm:"column:username"`
	Seat      *int      `gorm:"column:seat"`
	Team      *string   `gorm:"column:team"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (r roomPlayerRow) toRoomPlayer() RoomPlayer {
	// Field-by-field (not a struct conversion): RoomPlayer carries the
	// transient IsBot flag, which has no column. Rows scanned from
	// room_players are always humans — IsBot stays false.
	return RoomPlayer{
		ID:        r.ID,
		RoomID:    r.RoomID,
		UserID:    r.UserID,
		Username:  r.Username,
		Seat:      r.Seat,
		Team:      r.Team,
		CreatedAt: r.CreatedAt,
	}
}

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(room *Room) error {
	if err := r.db.Create(room).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "idx_rooms_name_active") {
				return apperr.ErrRoomNameTaken
			}
			if strings.Contains(pgErr.ConstraintName, "idx_rooms_code") {
				return apperr.ErrRoomCodeTaken
			}
		}
		return fmt.Errorf("creating room: %w", err)
	}
	return nil
}

func (r *GormRepository) FindByStatus(status string) ([]Room, error) {
	var rooms []Room
	if err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&rooms).Error; err != nil {
		return nil, fmt.Errorf("finding rooms by status: %w", err)
	}
	return rooms, nil
}

func (r *GormRepository) FindByCode(code string) (*Room, error) {
	var room Room
	if err := r.db.Where("code = ?", code).First(&room).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding room by code: %w", err)
	}
	return &room, nil
}

func (r *GormRepository) FindByID(id uint) (*Room, error) {
	var room Room
	if err := r.db.First(&room, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &room, nil
}

func (r *GormRepository) FindByIDForUpdate(id uint) (*Room, error) {
	var room Room
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&room, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &room, nil
}

func (r *GormRepository) Update(room *Room) error {
	if err := r.db.Save(room).Error; err != nil {
		return fmt.Errorf("updating room: %w", err)
	}
	return nil
}

func (r *GormRepository) UpdateStatus(roomID uint, status string) error {
	if err := r.db.Model(&Room{}).Where("id = ?", roomID).Update("status", status).Error; err != nil {
		return fmt.Errorf("updating room status: %w", err)
	}
	return nil
}

func (r *GormRepository) AddPlayer(roomPlayer *RoomPlayer) error {
	if err := r.db.Create(roomPlayer).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "idx_room_players_room_user") {
				return apperr.ErrAlreadyInRoom
			}
		}
		return fmt.Errorf("adding player to room: %w", err)
	}
	return nil
}

func (r *GormRepository) RemovePlayer(roomID uint, userID uint) error {
	result := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).Delete(&RoomPlayer{})
	if result.Error != nil {
		return fmt.Errorf("removing player from room: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrNotInRoom
	}
	return nil
}

func (r *GormRepository) FindPlayersByRoomID(roomID uint) ([]RoomPlayer, error) {
	var rows []roomPlayerRow
	err := r.db.Table("room_players").
		Select("room_players.*, users.username").
		Joins("JOIN users ON users.id = room_players.user_id").
		Where("room_players.room_id = ?", roomID).
		Order("room_players.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("finding players for room %d: %w", roomID, err)
	}
	players := make([]RoomPlayer, len(rows))
	for i, row := range rows {
		players[i] = row.toRoomPlayer()
	}
	return players, nil
}

func (r *GormRepository) FindPlayerRoom(userID uint) (*RoomPlayer, error) {
	var player RoomPlayer
	err := r.db.Table("room_players").
		Joins("JOIN rooms ON rooms.id = room_players.room_id").
		Where("room_players.user_id = ? AND rooms.status IN (?, ?) AND rooms.deleted_at IS NULL", userID, "waiting", "playing").
		First(&player).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding player room for user %d: %w", userID, err)
	}
	return &player, nil
}

func (r *GormRepository) UpdatePlayerSeat(roomID uint, userID uint, seat int, team string) error {
	result := r.db.Model(&RoomPlayer{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(map[string]interface{}{"seat": seat, "team": team})
	if result.Error != nil {
		return fmt.Errorf("updating player seat: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrNotInRoom
	}
	return nil
}

func (r *GormRepository) ClearPlayerSeat(roomID uint, userID uint) error {
	result := r.db.Model(&RoomPlayer{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(map[string]interface{}{"seat": nil, "team": nil})
	if result.Error != nil {
		return fmt.Errorf("clearing player seat: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrNotInRoom
	}
	return nil
}

func (r *GormRepository) FindPlayerBySeat(roomID uint, seat int) (*RoomPlayer, error) {
	var row roomPlayerRow
	err := r.db.Table("room_players").
		Select("room_players.*, users.username").
		Joins("JOIN users ON users.id = room_players.user_id").
		Where("room_players.room_id = ? AND room_players.seat = ?", roomID, seat).
		Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("finding player by seat: %w", err)
	}
	if row.ID == 0 {
		return nil, nil
	}
	player := row.toRoomPlayer()
	return &player, nil
}

func (r *GormRepository) FindQuickPlayRoom() (*Room, error) {
	return r.FindQuickPlayRoomExcluding(nil)
}

func (r *GormRepository) FindQuickPlayRoomExcluding(excludedRoomIDs map[uint]bool) (*Room, error) {
	var room Room
	q := r.db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("is_quick_play = ? AND status = ? AND player_count < 4", true, "waiting")
	if len(excludedRoomIDs) > 0 {
		ids := make([]uint, 0, len(excludedRoomIDs))
		for id := range excludedRoomIDs {
			ids = append(ids, id)
		}
		q = q.Where("id NOT IN ?", ids)
	}
	err := q.Order("created_at ASC").First(&room).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding quick play room: %w", err)
	}
	return &room, nil
}

func (r *GormRepository) FindUserIDsByRoomStatus(status string) ([]uint, error) {
	var ids []uint
	err := r.db.Table("room_players").
		Select("room_players.user_id").
		Joins("JOIN rooms ON rooms.id = room_players.room_id").
		Where("rooms.status = ? AND rooms.deleted_at IS NULL", status).
		Pluck("room_players.user_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("finding user ids by room status %q: %w", status, err)
	}
	return ids, nil
}

// LoadOwnerUsernames hydrates the transient OwnerUsername field on each room
// via a single SELECT against the users table. Skips the query for an empty
// slice and short-circuits when every room already has a username (the WS
// broadcast path occasionally pre-populates the field).
func (r *GormRepository) LoadOwnerUsernames(rooms []*Room) error {
	if len(rooms) == 0 {
		return nil
	}
	ids := make(map[uint]struct{}, len(rooms))
	for _, rm := range rooms {
		if rm != nil && rm.OwnerUsername == "" && rm.OwnerID != 0 {
			ids[rm.OwnerID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	idList := make([]uint, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	type ownerRow struct {
		ID       uint
		Username string
	}
	var rows []ownerRow
	if err := r.db.Table("users").Select("id, username").Where("id IN ?", idList).Scan(&rows).Error; err != nil {
		return fmt.Errorf("loading owner usernames: %w", err)
	}
	byID := make(map[uint]string, len(rows))
	for _, row := range rows {
		byID[row.ID] = row.Username
	}
	for _, rm := range rooms {
		if rm == nil || rm.OwnerUsername != "" {
			continue
		}
		if username, ok := byID[rm.OwnerID]; ok {
			rm.OwnerUsername = username
		}
	}
	return nil
}

// FindPlayersByRoomIDs returns players for every supplied room id in a single
// JOIN query. Empty input → empty map. Rooms with no players are simply
// missing from the returned map (callers should use the zero-value slice).
func (r *GormRepository) FindPlayersByRoomIDs(roomIDs []uint) (map[uint][]RoomPlayer, error) {
	out := make(map[uint][]RoomPlayer)
	if len(roomIDs) == 0 {
		return out, nil
	}
	var rows []roomPlayerRow
	err := r.db.Table("room_players").
		Select("room_players.*, users.username").
		Joins("JOIN users ON users.id = room_players.user_id").
		Where("room_players.room_id IN ?", roomIDs).
		Order("room_players.room_id ASC, room_players.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("finding players for %d rooms: %w", len(roomIDs), err)
	}
	for _, row := range rows {
		out[row.RoomID] = append(out[row.RoomID], row.toRoomPlayer())
	}
	return out, nil
}

func (r *GormRepository) AddBot(roomID uint, seat int) error {
	bot := &RoomBot{RoomID: roomID, Seat: seat}
	if err := r.db.Create(bot).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			strings.Contains(pgErr.ConstraintName, "idx_room_bots_room_seat") {
			return apperr.ErrSeatTaken
		}
		return fmt.Errorf("adding bot to room %d seat %d: %w", roomID, seat, err)
	}
	return nil
}

func (r *GormRepository) RemoveBot(roomID uint, seat int) error {
	result := r.db.Where("room_id = ? AND seat = ?", roomID, seat).Delete(&RoomBot{})
	if result.Error != nil {
		return fmt.Errorf("removing bot from room %d seat %d: %w", roomID, seat, result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrNoBotOnSeat
	}
	return nil
}

func (r *GormRepository) UpdateBotSeat(roomID uint, fromSeat, toSeat int) error {
	result := r.db.Model(&RoomBot{}).
		Where("room_id = ? AND seat = ?", roomID, fromSeat).
		Update("seat", toSeat)
	if result.Error != nil {
		var pgErr *pgconn.PgError
		if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" &&
			strings.Contains(pgErr.ConstraintName, "idx_room_bots_room_seat") {
			return apperr.ErrSeatTaken
		}
		return fmt.Errorf("moving bot in room %d from seat %d to %d: %w", roomID, fromSeat, toSeat, result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrNoBotOnSeat
	}
	return nil
}

func (r *GormRepository) FindBotsByRoomID(roomID uint) ([]RoomBot, error) {
	var bots []RoomBot
	if err := r.db.Where("room_id = ?", roomID).Order("seat ASC").Find(&bots).Error; err != nil {
		return nil, fmt.Errorf("finding bots for room %d: %w", roomID, err)
	}
	return bots, nil
}

// FindBotsByRoomIDs returns bots for every supplied room id in a single query.
// Empty input → empty map. Rooms with no bots are missing from the map.
func (r *GormRepository) FindBotsByRoomIDs(roomIDs []uint) (map[uint][]RoomBot, error) {
	out := make(map[uint][]RoomBot)
	if len(roomIDs) == 0 {
		return out, nil
	}
	var bots []RoomBot
	err := r.db.Where("room_id IN ?", roomIDs).
		Order("room_id ASC, seat ASC").
		Find(&bots).Error
	if err != nil {
		return nil, fmt.Errorf("finding bots for %d rooms: %w", len(roomIDs), err)
	}
	for _, b := range bots {
		out[b.RoomID] = append(out[b.RoomID], b)
	}
	return out, nil
}

func (r *GormRepository) RunInTransaction(fn func(RoomRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &GormRepository{db: tx}
		return fn(txRepo)
	})
}

func (r *GormRepository) IncrementPlayerCount(roomID uint) error {
	result := r.db.Model(&Room{}).Where("id = ?", roomID).Update("player_count", gorm.Expr("player_count + 1"))
	if result.Error != nil {
		return fmt.Errorf("incrementing player count: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrRoomNotFound
	}
	return nil
}

func (r *GormRepository) DecrementPlayerCount(roomID uint) error {
	result := r.db.Model(&Room{}).Where("id = ? AND player_count > 0", roomID).Update("player_count", gorm.Expr("player_count - 1"))
	if result.Error != nil {
		return fmt.Errorf("decrementing player count: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.ErrRoomNotFound
	}
	return nil
}
