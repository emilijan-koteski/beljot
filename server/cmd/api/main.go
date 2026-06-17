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
	"github.com/emilijan/beljot/server/internal/chat"
	"github.com/emilijan/beljot/server/internal/config"
	"github.com/emilijan/beljot/server/internal/emote"
	"github.com/emilijan/beljot/server/internal/lobby"
	"github.com/emilijan/beljot/server/internal/mailer"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/passwordreset"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

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
	userRepo := user.NewGormUserRepository(db)
	authHandler := auth.NewAuthHandler(userRepo, cfg.JWTSecret, cfg.Environment)

	// Mailer: real SMTP when fully configured, otherwise a log-only fallback so
	// the forgot-password flow stays testable in dev without SMTP credentials.
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

	e := echo.New()
	e.HideBanner = true

	// Middleware registration order is load-bearing: CORS -> Logging -> Error Handler -> Auth
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			slog.Info("request",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
			)
			return nil
		},
	}))

	e.HTTPErrorHandler = appErrorHandler

	// Routes
	// HEAD is registered explicitly so health-check probes (UptimeRobot,
	// load balancers, k8s) that default to HEAD don't get 405s.
	e.GET("/health", healthHandler)
	e.HEAD("/health", healthHandler)

	// Auth routes — public, no auth middleware
	authGroup := e.Group("/api/v1/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/logout", authHandler.Logout)
	authGroup.POST("/forgot-password", passwordResetHandler.ForgotPassword)
	authGroup.POST("/reset-password", passwordResetHandler.ResetPassword)

	// Authenticated route group
	matchRepo := match.NewGormMatchRepository(db)
	userHandler := user.NewUserHandler(userRepo, matchRepo)
	api := e.Group("/api/v1", auth.AuthMiddleware(cfg.JWTSecret))
	api.GET("/users/:id/profile", userHandler.GetProfile)
	api.GET("/users/:id/career", userHandler.GetCareer)
	api.GET("/users/:id/matches", userHandler.ListMatches)
	api.PATCH("/users/:id/preferences", userHandler.UpdatePreferences)

	// Lobby stats — wired after hub + sessionManager + roomRepo so the handler
	// can read the four data sources it bucket-counts.

	// WebSocket hub and endpoint
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

	// Session manager + room repo (repo needed before handler wiring)
	roomRepo := room.NewGormRepository(db)
	sessionManager := match.NewManager(hub, matchRepo)
	sessionManager.SetRoomUpdater(&room.RoomStatusAdapter{Repo: roomRepo})

	// Reconcile rooms left in status="playing" by a previous process. Sessions
	// live only in process memory, so any "playing" row at boot has no live
	// session — its players would be stranded by FindPlayerRoom (which gates
	// quick-play / create-room on "no active room"). Best-effort: log + keep
	// going if reconciliation fails so a transient DB hiccup doesn't block
	// boot entirely.
	if err := sessionManager.ReconcileStaleRooms(&room.StaleRoomRepositoryAdapter{Repo: roomRepo}); err != nil {
		slog.Error("startup reconciliation failed", "error", err)
	}

	// Chat handler — composed with sessionManager.HandleAction so a single
	// hub action handler can route both game actions and chat messages.
	// roomMembership is an inline adapter that resolves room recipients
	// for room-scoped chat: returns members only while the room is in
	// "waiting" status (pre-match).
	roomMembership := &chatRoomMembership{repo: roomRepo}
	chatHandler := chat.NewHandler(hub, userRepo, sessionManager, roomMembership)
	emoteHandler := emote.NewHandler(hub, sessionManager)
	sessionManager.AddUserRemovedHook(emoteHandler.RemoveUser)
	hub.SetActionHandler(func(client *ws.Client, msg ws.WSMessage) {
		if msg.Type == ws.ActionChatMessage {
			chatHandler.HandleAction(client, msg)
			return
		}
		// Emote handler is wired BEFORE sessionManager.HandleAction so the
		// rules engine never sees action:emote — parseAction would otherwise
		// reject it as an unknown action type and emit error:invalid_action.
		if msg.Type == ws.ActionEmote {
			emoteHandler.HandleAction(client, msg)
			return
		}
		sessionManager.HandleAction(client, msg)
	})

	// Presence registry — tracks which users are actually "back" in a reopened
	// room (returned / freshly joined) vs still on the match result dialog.
	// Shared between the room handler (add/remove/clear + payload) and the lobby
	// disconnect handler (drop on lobby-timeout close). In-memory, not durable.
	presenceRegistry := room.NewPresenceRegistry()

	// Lobby disconnect handler — frees seats after 10s when players disconnect in room lobby
	lobbyDisconnectHandler := room.NewLobbyDisconnectHandler(roomRepo, hub, presenceRegistry)
	hub.SetConnectHandler(func(userID uint) {
		sessionManager.HandleReconnect(userID)
		// Always follow with a direct state push: when the hub replaced a
		// still-registered socket (no disconnect ever fired) HandleReconnect
		// no-ops, and the refreshed client has no other way to obtain state.
		sessionManager.SyncStateOnConnect(userID)
		lobbyDisconnectHandler.HandleReconnect(userID)
	})
	hub.SetDisconnectHandler(func(userID uint) {
		sessionManager.HandleDisconnect(userID)
		lobbyDisconnectHandler.HandleDisconnect(userID)
	})

	// Room routes
	roomHandler := room.NewRoomHandler(roomRepo, sessionManager, hub, presenceRegistry)
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
	api.POST("/rooms/:id/bots", roomHandler.AddBot)
	api.DELETE("/rooms/:id/bots/:seat", roomHandler.RemoveBot)

	// Lobby stats endpoint — bucket-counts connected users into in-lobby /
	// in-room / in-game and reports registered totals.
	lobbyHandler := lobby.NewHandler(hub, sessionManager, roomRepo, userRepo)
	api.GET("/lobby/stats", lobbyHandler.GetStats)

	// Public landing-page stats — unauthenticated, registered on the bare echo
	// instance (outside the auth-protected api group). Returns only aggregate
	// counts (online players, open rooms), nothing user-identifying.
	e.GET("/api/v1/stats", lobbyHandler.GetPublicStats)

	// Graceful shutdown
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func healthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// chatRoomMembership adapts room.RoomRepository to chat.RoomMembership.
// Returns userIDs only when the room exists AND its status is "waiting"
// (pre-match). Enforces the server-side status gate for room-scoped chat.
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
