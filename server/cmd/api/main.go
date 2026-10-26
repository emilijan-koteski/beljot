package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/avatar"
	"github.com/emilijan/beljot/server/internal/chat"
	"github.com/emilijan/beljot/server/internal/config"
	"github.com/emilijan/beljot/server/internal/dbmigrate"
	"github.com/emilijan/beljot/server/internal/emote"
	"github.com/emilijan/beljot/server/internal/friend"
	"github.com/emilijan/beljot/server/internal/identity"
	"github.com/emilijan/beljot/server/internal/lobby"
	"github.com/emilijan/beljot/server/internal/mailer"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/passwordreset"
	"github.com/emilijan/beljot/server/internal/refreshtoken"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/season"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/wallet"
	"github.com/emilijan/beljot/server/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	schemaVersion, err := dbmigrate.Up(cfg.DatabaseURL)
	if err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database schema up to date", "version", schemaVersion)

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.New(
			log.New(os.Stdout, "", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				IgnoreRecordNotFoundError: true,
				LogLevel:                  gormlogger.Warn,
			},
		),
	})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to access database pool", "error", err)
		os.Exit(1)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	userRepo := user.NewGormUserRepository(db)
	refreshRepo := refreshtoken.NewGormRepository(db)
	identityRepo := identity.NewGormRepository(db)

	ssoProviders := identity.Registry{}
	if cfg.GoogleClientID != "" {
		google := identity.NewGoogleProvider(cfg.GoogleClientID)
		ssoProviders[google.Name()] = google
	} else {
		slog.Warn("BELJOT_GOOGLE_CLIENT_ID not set — Google sign-in disabled")
	}

	authHandler := auth.NewAuthHandler(userRepo, refreshRepo, identityRepo, ssoProviders, cfg.JWTSecret, cfg.Environment, cfg.AccessTokenTTL, cfg.RefreshIdleTTL, cfg.RefreshAbsoluteTTL)

	var appMailer mailer.Mailer
	if cfg.SMTPConfigured() {
		appMailer = mailer.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, cfg.SMTPFromName)
		slog.Info("SMTP mailer configured", "host", cfg.SMTPHost, "port", cfg.SMTPPort)
	} else {
		appMailer = mailer.NewLogMailer()
		slog.Warn("SMTP not configured — password reset links will be logged, not emailed")
	}
	resetRepo := passwordreset.NewGormRepository(db)
	passwordResetHandler := auth.NewPasswordResetHandler(userRepo, resetRepo, appMailer, cfg.AppBaseURL, time.Hour)

	var avatarStore avatar.Store
	if cfg.AvatarStorageConfigured() {
		s3Store, err := avatar.NewS3Store(context.Background(), avatar.S3Config{
			Endpoint:  cfg.S3Endpoint,
			Region:    cfg.S3Region,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			Bucket:    cfg.S3PublicBucket,
		})
		if err != nil {
			slog.Error("failed to build the avatar object-store client", "error", err)
			os.Exit(1)
		}
		avatarStore = s3Store
		user.SetPublicAssetsURL(cfg.PublicAssetsURL)
		slog.Info("avatar storage configured", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3PublicBucket, "publicURL", cfg.PublicAssetsURL)
	} else {
		slog.Warn("avatar storage not configured — avatar uploads answer 503")
	}

	e := echo.New()
	e.HideBanner = true
	e.IPExtractor = clientIPExtractor()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogMethod:   true,
		LogError:    true,
		LogRemoteIP: true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []any{"method", v.Method, "uri", v.URI, "status", v.Status, "ip", v.RemoteIP}
			if v.Error != nil {
				attrs = append(attrs, "error", v.Error.Error())
			}
			switch {
			case v.Status >= 500:
				slog.Error("request", attrs...)
			case v.Status >= 400:
				slog.Warn("request", attrs...)
			default:
				slog.Info("request", attrs...)
			}
			return nil
		},
	}))

	e.HTTPErrorHandler = appErrorHandler

	health := healthHandler(sqlDB)
	e.GET("/health", health)
	e.HEAD("/health", health)
	e.GET("/healthz", health)
	e.HEAD("/healthz", health)

	authGroup := e.Group("/api/v1/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", authHandler.Logout)
	authGroup.POST("/forgot-password", passwordResetHandler.ForgotPassword)
	authGroup.POST("/reset-password", passwordResetHandler.ResetPassword)
	authGroup.POST("/sso/:provider", authHandler.SSOLogin)
	authGroup.POST("/sso/:provider/link", authHandler.SSOLink)

	matchRepo := match.NewGormMatchRepository(db)
	seasonRepo := season.NewGormRepository(db)
	seasonService := season.NewService(seasonRepo)
	seasonService.SetAvatarURLResolver(user.SmallAvatarURL)
	userHandler := user.NewUserHandler(userRepo, matchRepo, seasonService)
	api := e.Group("/api/v1", auth.AuthMiddleware(cfg.JWTSecret))
	api.GET("/users", userHandler.SearchUsers)
	api.GET("/users/:id/profile", userHandler.GetProfile)
	api.GET("/users/:id/career", userHandler.GetCareer)
	api.GET("/users/:id/matches", userHandler.ListMatches)
	api.PATCH("/users/:id/preferences", userHandler.UpdatePreferences)
	api.PATCH("/users/:id/username", userHandler.UpdateUsername)
	avatarHandler := avatar.NewHandler(avatarStore, userRepo)
	api.PUT("/users/:id/avatar", avatarHandler.Upload)
	api.DELETE("/users/:id/avatar", avatarHandler.Remove)
	api.GET("/users/:id/identities", authHandler.ListIdentities)
	api.POST("/users/:id/identities/:provider", authHandler.LinkIdentity)
	api.DELETE("/users/:id/identities/:provider", authHandler.UnlinkIdentity)

	walletRepo := wallet.NewGormRepository(db)
	walletService := wallet.NewService(walletRepo)
	walletHandler := wallet.NewWalletHandler(walletService)
	api.POST("/wallet/daily-login", walletHandler.ProcessDailyLogin)

	hub := ws.NewHub()
	go hub.Run()
	wsHandler := &ws.WSHandler{
		Hub:             hub,
		JWTSecret:       cfg.JWTSecret,
		AcceptedOrigins: cfg.CORSOrigins,
		ValidateToken: func(token string) ([]string, string, error) {
			claims, err := auth.ValidateToken(token, cfg.JWTSecret)
			if err != nil {
				return nil, "", err
			}
			return []string(claims.Audience), claims.Subject, nil
		},
	}
	e.GET("/ws", wsHandler.HandleWS)

	roomRepo := room.NewGormRepository(db)
	sessionManager := match.NewManager(hub, matchRepo)
	sessionManager.SetBotSearch(match.BotSearchSettings{
		Workers:   cfg.BotSearchWorkers,
		Deadline:  cfg.BotSearchDeadline,
		FromTrick: cfg.BotSearchFromTrick,
	})
	sessionManager.SetRoomUpdater(&room.RoomStatusAdapter{Repo: roomRepo})
	sessionManager.SetWalletSettler(walletService)
	sessionManager.SetXPAwarder(user.NewXPService(userRepo))
	honorService := user.NewHonorService(userRepo)
	sessionManager.SetHonorRecorder(honorService)
	sessionManager.SetSPAwarder(seasonService)

	seasonRollover := season.NewRollover(seasonRepo, 0, nil)
	seasonRollover.Start()

	recalculations, err := season.RunPendingRecalculations(db, time.Now())
	for _, r := range recalculations {
		logSeasonRecalculation(r)
	}
	if err != nil {
		slog.Error("season recalculation failed; it stays pending and retries on the next start", "error", err)
	}

	if err := sessionManager.ReconcileStaleRooms(&room.StaleRoomRepositoryAdapter{Repo: roomRepo}); err != nil {
		slog.Error("startup reconciliation failed", "error", err)
	}

	roomMembership := &chatRoomMembership{repo: roomRepo}
	chatHandler := chat.NewHandler(hub, userRepo, sessionManager, roomMembership)
	emoteHandler := emote.NewHandler(hub, sessionManager)
	sessionManager.AddUserRemovedHook(emoteHandler.RemoveUser)

	friendRepo := friend.NewGormRepository(db)
	whisperHandler := chat.NewWhisperHandler(hub, userRepo, friendRepo, &whisperPresenceLocator{repo: roomRepo})
	hub.SetActionHandler(func(client *ws.Client, msg ws.WSMessage) {
		if msg.Type == ws.ActionChatMessage {
			chatHandler.HandleAction(client, msg)
			return
		}
		if msg.Type == ws.ActionEmote {
			emoteHandler.HandleAction(client, msg)
			return
		}
		if msg.Type == ws.ActionWhisper {
			whisperHandler.HandleAction(client, msg)
			return
		}
		sessionManager.HandleAction(client, msg)
	})

	presenceRegistry := room.NewPresenceRegistry()

	inviteRegistry := room.NewInviteRegistry()

	lobbyDisconnectHandler := room.NewLobbyDisconnectHandler(roomRepo, hub, presenceRegistry, inviteRegistry)
	hub.SetConnectHandler(func(userID uint) {
		sessionManager.HandleReconnect(userID)
		sessionManager.SyncStateOnConnect(userID)
		lobbyDisconnectHandler.HandleReconnect(userID)
	})
	hub.SetDisconnectHandler(func(userID uint) {
		sessionManager.HandleDisconnect(userID)
		lobbyDisconnectHandler.HandleDisconnect(userID)
		inviteRegistry.VoidUser(userID)
	})

	roomHandler := room.NewRoomHandler(roomRepo, sessionManager, hub, presenceRegistry, walletService, honorService, inviteRegistry)
	api.POST("/rooms", roomHandler.CreateRoom)
	api.GET("/rooms", roomHandler.ListRooms)
	api.POST("/rooms/quick-play", roomHandler.QuickPlay)
	api.GET("/rooms/code/:code", roomHandler.GetRoomByCode)
	api.GET("/rooms/:id", roomHandler.GetRoom)
	api.POST("/rooms/:id/join", roomHandler.JoinRoom)
	api.POST("/rooms/:id/quick-join", roomHandler.QuickJoin)
	api.POST("/rooms/:id/leave", roomHandler.LeaveRoom)
	api.POST("/rooms/:id/return", roomHandler.ReturnToRoom)
	api.POST("/rooms/:id/seat", roomHandler.SelectSeat)
	api.POST("/rooms/:id/leave-seat", roomHandler.LeaveSeat)
	api.POST("/rooms/:id/start", roomHandler.StartMatch)
	api.POST("/rooms/:id/kick", roomHandler.KickPlayer)
	api.POST("/rooms/:id/swap-seats", roomHandler.SwapSeats)
	api.POST("/rooms/:id/transfer-ownership", roomHandler.TransferOwnership)
	api.POST("/rooms/:id/privacy", roomHandler.UpdateRoomPrivacy)
	api.POST("/rooms/:id/bots", roomHandler.AddBot)
	api.DELETE("/rooms/:id/bots/:seat", roomHandler.RemoveBot)
	api.GET("/rooms/:id/last-match", userHandler.GetRoomLastMatch)

	inviteDirectory := &inviteFriendDirectory{friends: friendRepo, users: userRepo}
	inviteHandler := room.NewInviteHandler(
		roomRepo,
		inviteRegistry,
		inviteDirectory,
		inviteDirectory,
		hub,
		sessionManager,
		hub,
	)
	api.GET("/rooms/:id/invitable-friends", inviteHandler.ListInvitableFriends)
	api.GET("/rooms/:id/invitable-players", inviteHandler.ListInvitablePlayers)
	api.POST("/rooms/:id/invite", inviteHandler.InviteToRoom)
	api.POST("/rooms/:id/invite/decline", inviteHandler.DeclineInvite)

	friendHandler := friend.NewHandler(friendRepo, userRepo, hub)
	api.POST("/friends/request", friendHandler.SendRequest)
	api.GET("/friends", friendHandler.ListFriends)
	api.GET("/friends/requests", friendHandler.ListRequests)
	api.GET("/friends/status/:id", friendHandler.GetStatus)
	api.POST("/friends/:id/accept", friendHandler.Accept)
	api.POST("/friends/:id/decline", friendHandler.Decline)
	api.DELETE("/friends/:id", friendHandler.Unfriend)

	seasonHandler := season.NewHandler(seasonService)
	api.GET("/seasons/current", seasonHandler.GetCurrentSeason)
	api.GET("/seasons", seasonHandler.GetSeasons)
	api.GET("/users/:id/seasons", seasonHandler.GetPlayerSeasonArchive)
	api.GET("/leaderboard", seasonHandler.GetLeaderboard)

	lobbyHandler := lobby.NewHandler(hub, sessionManager, roomRepo, userRepo)
	api.GET("/lobby/stats", lobbyHandler.GetStats)

	e.GET("/api/v1/stats", lobbyHandler.GetPublicStats)

	go func() {
		slog.Info("starting server", "port", cfg.Port)
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	hub.Shutdown()
	seasonRollover.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// logSeasonRecalculation logs one finished recalculation job: how many rows it
// rewrote, what the replay skipped, and any row of the season it left as it was.
func logSeasonRecalculation(r season.RecalculationResult) {
	start := r.SeasonStartedAt.UTC().Format(time.RFC3339)
	if !r.SeasonFound {
		slog.Warn("season recalculation: no season starts at the job's start; marked done, nothing written",
			"seasonStartedAt", start)
		return
	}
	slog.Info("season recalculation: done",
		"season", r.SeasonName,
		"seasonStartedAt", start,
		"playersRewritten", r.Rewritten,
		"matchesLoaded", r.Summary.Loaded,
		"matchesScored", r.Summary.Scored,
		"matchesSkipped", r.Summary.Skipped)
	if len(r.Untouched) > 0 {
		slog.Warn("season recalculation: rows the replay did not cover were left as they are",
			"season", r.SeasonName,
			"userIds", r.Untouched)
	}
}

// chatRoomMembership adapts room.RoomRepository to chat.RoomMembership.
// It returns members only while the room is "waiting", gating room chat to pre-match.
type chatRoomMembership struct {
	repo room.RoomRepository
}

func (a *chatRoomMembership) RoomMembers(roomID uint) ([]uint, bool) {
	r, err := a.repo.FindByID(roomID)
	if err != nil || r == nil || r.Status != "waiting" {
		return nil, false
	}
	players, err := a.repo.FindPlayersByRoomID(roomID)
	if err != nil {
		return nil, false
	}
	ids := make([]uint, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.UserID)
	}
	return ids, true
}

// whisperPresenceLocator adapts room.RoomRepository to chat.PresenceLocator for
// the whisper anti-collusion check, reporting the user's waiting or playing room.
type whisperPresenceLocator struct {
	repo room.RoomRepository
}

func (p *whisperPresenceLocator) ActiveRoomID(userID uint) (uint, bool, error) {
	rp, err := p.repo.FindPlayerRoom(userID)
	if err != nil {
		return 0, false, err
	}
	if rp == nil {
		return 0, false, nil
	}
	return rp.RoomID, true, nil
}

// inviteFriendDirectory adapts friend.Repository and user.UserRepository to
// room.FriendDirectory and room.PlayerDirectory, so the room package never imports friend.
type inviteFriendDirectory struct {
	friends friend.Repository
	users   user.UserRepository
}

func (d *inviteFriendDirectory) AreFriends(a, b uint) (bool, error) {
	return d.friends.AreFriends(a, b)
}

// ListFriends returns the viewer's accepted friends resolved to usernames and
// avatar URLs, omitting friends whose user row was soft-deleted.
func (d *inviteFriendDirectory) ListFriends(userID uint) ([]room.FriendSummary, error) {
	rows, err := d.friends.ListAccepted(userID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, f := range rows {
		other := f.UserID
		if other == userID {
			other = f.FriendID
		}
		ids = append(ids, other)
	}
	if len(ids) == 0 {
		return []room.FriendSummary{}, nil
	}

	users, err := d.users.FindManyByIDs(ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]user.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}

	out := make([]room.FriendSummary, 0, len(ids))
	for _, id := range ids {
		u, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, room.FriendSummary{UserID: id, Username: u.Username, AvatarURL: user.SmallAvatarURL(u.AvatarKey)})
	}
	return out, nil
}

// FindPlayers resolves user IDs to invite-panel rows with their non-friend invite opt-in,
// omitting IDs whose user row is missing or soft-deleted.
func (d *inviteFriendDirectory) FindPlayers(userIDs []uint) ([]room.PlayerSummary, error) {
	if len(userIDs) == 0 {
		return []room.PlayerSummary{}, nil
	}
	users, err := d.users.FindManyByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	out := make([]room.PlayerSummary, 0, len(users))
	for _, u := range users {
		out = append(out, room.PlayerSummary{
			UserID:                 u.ID,
			Username:               u.Username,
			AvatarURL:              user.SmallAvatarURL(u.AvatarKey),
			AllowsNonFriendInvites: u.AllowNonFriendInvites,
		})
	}
	return out, nil
}

func appErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		if writeErr := c.JSON(appErr.Status, map[string]interface{}{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		}); writeErr != nil {
			slog.Error("failed to write error response", "error", writeErr)
		}
		return
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		msg := "An error occurred"
		if m, ok := echoErr.Message.(string); ok {
			msg = m
		}
		if writeErr := c.JSON(echoErr.Code, map[string]interface{}{
			"error": map[string]string{
				"code":    "HTTP_ERROR",
				"message": msg,
			},
		}); writeErr != nil {
			slog.Error("failed to write error response", "error", writeErr)
		}
		return
	}

	slog.Error("unhandled error", "error", err)
	if writeErr := c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]string{
			"code":    apperr.ErrInternal.Code,
			"message": apperr.ErrInternal.Message,
		},
	}); writeErr != nil {
		slog.Error("failed to write error response", "error", writeErr)
	}
}
