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
	refreshRepo := refreshtoken.NewGormRepository(db)
	identityRepo := identity.NewGormRepository(db)

	// SSO provider registry — a provider is registered only when its config is
	// present; the handlers stay provider-agnostic and future providers are a
	// new identity.Provider impl plus an entry here.
	ssoProviders := identity.Registry{}
	if cfg.GoogleClientID != "" {
		google := identity.NewGoogleProvider(cfg.GoogleClientID)
		ssoProviders[google.Name()] = google
	} else {
		slog.Warn("BELJOT_GOOGLE_CLIENT_ID not set — Google sign-in disabled")
	}

	authHandler := auth.NewAuthHandler(userRepo, refreshRepo, identityRepo, ssoProviders, cfg.JWTSecret, cfg.Environment, cfg.AccessTokenTTL, cfg.RefreshIdleTTL, cfg.RefreshAbsoluteTTL)

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

	// HandleError is load-bearing, not decoration: Echo invokes HTTPErrorHandler
	// AFTER the middleware chain unwinds, so without it this logger reads the
	// response before the error handler has written a status and records the
	// default 200 for EVERY failure. A 404 room, a 401 login and a 409 join gate
	// all logged as success, which means no error rate is visible in production
	// logs at all. HandleError routes the error through appErrorHandler inside the
	// chain so v.Status is the status the client actually received.
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogMethod:   true,
		LogError:    true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			// Failures log at ERROR/WARN so they surface without grepping every
			// request line. 5xx is ours, 4xx is the caller's.
			attrs := []any{"method", v.Method, "uri", v.URI, "status", v.Status}
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
	authGroup.POST("/sso/:provider", authHandler.SSOLogin)
	authGroup.POST("/sso/:provider/link", authHandler.SSOLink)

	// Authenticated route group
	matchRepo := match.NewGormMatchRepository(db)
	// Season repo + service (Story 13.1/13.3). Constructed HERE -- above the user
	// handler -- because Story 13.3 injects the service into it as the narrow
	// SeasonRankReader behind the profile's seasonRank block. The same single
	// instance is also the match manager's SPAwarder (set further down, once the
	// session manager exists) and the season handler's service: one service,
	// several narrow consumers, exactly the honorService shape.
	seasonRepo := season.NewGormRepository(db)
	seasonService := season.NewService(seasonRepo)
	userHandler := user.NewUserHandler(userRepo, matchRepo, seasonService)
	api := e.Group("/api/v1", auth.AuthMiddleware(cfg.JWTSecret))
	api.GET("/users", userHandler.SearchUsers)
	api.GET("/users/:id/profile", userHandler.GetProfile)
	api.GET("/users/:id/career", userHandler.GetCareer)
	api.GET("/users/:id/matches", userHandler.ListMatches)
	api.PATCH("/users/:id/preferences", userHandler.UpdatePreferences)
	api.PATCH("/users/:id/username", userHandler.UpdateUsername)
	// Profile-side SSO identity management (authed, self-only). The public
	// /auth/sso/* routes handle login/register/link-during-login; these manage
	// an already-authenticated user's linked accounts.
	api.GET("/users/:id/identities", authHandler.ListIdentities)
	api.POST("/users/:id/identities/:provider", authHandler.LinkIdentity)
	api.DELETE("/users/:id/identities/:provider", authHandler.UnlinkIdentity)

	// Wallet — daily-login bonus grant. POST (it mutates) on the authed group;
	// the client calls it once per app session on bootstrap (covers explicit
	// login and refresh-token auto-login). Idempotent: at most one grant per UTC day.
	walletRepo := wallet.NewGormRepository(db)
	walletService := wallet.NewService(walletRepo)
	walletHandler := wallet.NewWalletHandler(walletService)
	api.POST("/wallet/daily-login", walletHandler.ProcessDailyLogin)

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
	// Story 9.2: the match manager credits winning human seats + reads balances
	// at match end via the wallet service.
	sessionManager.SetWalletSettler(walletService)
	// Story 9.5: the match manager awards lifetime XP at match end via the
	// user-side XP service (built from the already-constructed userRepo).
	sessionManager.SetXPAwarder(user.NewXPService(userRepo))
	// Story 9.7: the match manager records honor (completed vs abandoned) at
	// match end via the user-side honor service. Same injection shape as the XP
	// awarder above, and for the same reason — user imports match, so match
	// must never import user.
	//
	// Constructed once into a local because Story 9.8 injects the SAME service into
	// the room handler below as its HonorService (the per-join honor gate). The two
	// consumers use different subsets of it — match takes the write path
	// (ApplyHonorEvents), room takes the read path (HonorForUsers) — and each
	// declares its own narrow interface, so one instance satisfies both.
	honorService := user.NewHonorService(userRepo)
	sessionManager.SetHonorRecorder(honorService)
	// Story 13.1: the match manager accrues Season Points and refreshes the rank
	// tier at match end via the season service. Same injection shape as the XP
	// awarder and honor recorder above, and for the same reason — season imports
	// match, so match must never import season. The service itself is built
	// earlier (above the user handler, which consumes it as its SeasonRankReader).
	sessionManager.SetSPAwarder(seasonService)

	// Story 13.3: the nightly rollover job — a thin wrapper over the same lazy
	// resolver every read uses, so a ZERO-TRAFFIC deployment still gets its new
	// quarter row (and a log line proving it ran). Idempotent by construction
	// (ON CONFLICT (started_at) DO NOTHING); correctness never depends on it.
	// Stopped in the graceful-shutdown path below, after hub.Shutdown().
	seasonRollover := season.NewRollover(seasonRepo, 0, nil)
	seasonRollover.Start()

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

	// Whisper handler (Story 11.4) — private friend-to-friend messages, composed
	// into the action router alongside chat/emote. Reuses friendRepo (created
	// here so it is in scope for both the whisper handler and the friend routes
	// below) for the friends-only check, and a presence adapter over roomRepo for
	// the anti-collusion "same room/match" check.
	friendRepo := friend.NewGormRepository(db)
	whisperHandler := chat.NewWhisperHandler(hub, userRepo, friendRepo, &whisperPresenceLocator{repo: roomRepo})
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
		// Whisper handler, same reasoning as emote: routed before the session
		// manager so the rules engine never sees action:whisper.
		if msg.Type == ws.ActionWhisper {
			whisperHandler.HandleAction(client, msg)
			return
		}
		sessionManager.HandleAction(client, msg)
	})

	// Presence registry — tracks which users are actually "back" in a reopened
	// room (returned / freshly joined) vs still on the match result dialog.
	// Shared between the room handler (add/remove/clear + payload) and the lobby
	// disconnect handler (drop on lobby-timeout close). In-memory, not durable.
	presenceRegistry := room.NewPresenceRegistry()

	// Invite registry — holds the one-time host-invite grants that carry a
	// friend past a private room's password (Story 11.5). Shared between the
	// invite handler (issues grants) and the room handler (consumes / voids
	// them), so it MUST be the same instance in both. In-memory, not durable.
	inviteRegistry := room.NewInviteRegistry()

	// Lobby disconnect handler — frees seats after 10s when players disconnect in room lobby
	lobbyDisconnectHandler := room.NewLobbyDisconnectHandler(roomRepo, hub, presenceRegistry, inviteRegistry)
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
		// A friend who drops off the lobby cannot act on their invite popup, so
		// their outstanding grants die with the connection (Story 11.5 AC2).
		// Wired here rather than inside LobbyDisconnectHandler because that
		// handler returns early for users who are NOT in a room — which is
		// precisely every invitee.
		inviteRegistry.VoidUser(userID)
	})

	// Room routes
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
	// The room's most recent match, in the profile's viewer-relative
	// MatchListItem shape. Owned by userHandler, not roomHandler, despite the
	// /rooms path: building that DTO needs the batched username hydration on
	// the user repository, which roomHandler does not have. Authorization is
	// match participation (enforced inside the repository query), not room
	// membership — the end-of-match dialog reads it while the room is still
	// "completed", which the room-member guard rejects outright.
	api.GET("/rooms/:id/last-match", userHandler.GetRoomLastMatch)

	// Friend room-invite routes (Story 11.5, FR62). Registered on the same
	// /rooms/:id action group so the invite endpoints share the room's auth and
	// membership semantics. inviteFriendDirectory does the friend-rows →
	// usernames join here in main.go, which keeps the import direction room →
	// friend one-way (friend must never import room).
	inviteHandler := room.NewInviteHandler(
		roomRepo,
		inviteRegistry,
		&inviteFriendDirectory{friends: friendRepo, users: userRepo},
		hub,
		sessionManager,
		hub,
	)
	api.GET("/rooms/:id/invitable-friends", inviteHandler.ListInvitableFriends)
	api.POST("/rooms/:id/invite", inviteHandler.InviteToRoom)
	// Declining is called by the INVITEE, who is not in the room — hence a
	// separate route rather than a member-guarded one.
	api.POST("/rooms/:id/invite/decline", inviteHandler.DeclineInvite)

	// Friend routes (Story 11.2, FR6) — friend requests, friend list, and the
	// best-effort per-user system:friend_request push. Placed after the hub
	// (line ~160) so the notifier is in scope, and co-located with the room /
	// lobby block since it shares userRepo + hub. The friend queries live on a
	// dedicated friend.Repository (NOT extensions of user.UserRepository) so no
	// existing user-repo mock changes. Echo resolves the static
	// /friends/requests and /friends/status/:id segments ahead of the param
	// route /friends/:id/accept (static > param), so there is no collision.
	// friendRepo is created earlier (near the whisper handler wiring) so both the
	// whisper friend-check and these friend routes share one instance.
	friendHandler := friend.NewHandler(friendRepo, userRepo, hub)
	api.POST("/friends/request", friendHandler.SendRequest)
	api.GET("/friends", friendHandler.ListFriends)
	api.GET("/friends/requests", friendHandler.ListRequests)
	api.GET("/friends/status/:id", friendHandler.GetStatus)
	api.POST("/friends/:id/accept", friendHandler.Accept)
	api.POST("/friends/:id/decline", friendHandler.Decline)
	// Unfriend — party-agnostic removal of an accepted friendship. DELETE is a
	// distinct method from the GET/POST /friends routes above, so no collision.
	api.DELETE("/friends/:id", friendHandler.Unfriend)

	// Seasonal rank (Story 13.1) — the active season window plus the CALLER'S OWN
	// record, keyed off the JWT subject rather than any path/query id. Feeds the
	// lobby RankBanner; the WS event:season_points_awarded push invalidates the
	// query rather than this being polled. Story 13.2's leaderboard endpoint sits
	// beside it, on the same seasonService constructed above.
	seasonHandler := season.NewHandler(seasonService)
	api.GET("/seasons/current", seasonHandler.GetCurrentSeason)
	// Seasons list (Story 13.3) — every window newest-first, feeding the
	// leaderboard page's season picker. Echo resolves the static
	// /seasons/current above ahead of nothing here: both are static segments,
	// no collision.
	api.GET("/seasons", seasonHandler.GetSeasons)
	// Prior-season archive (Story 13.3) — the SUBJECT's ended, played seasons.
	// A /users path served by the season handler (like /rooms/:id/last-match is
	// served by userHandler): the data is season-domain, only the URL is
	// user-shaped. No user-existence 404 here — the profile query owns that.
	api.GET("/users/:id/seasons", seasonHandler.GetPlayerSeasonArchive)
	// Seasonal leaderboard (Story 13.2) — an SP-ordered page of the active season
	// plus the CALLER'S OWN position under the same order. Same service, same
	// repository, no new construction. Story 13.3 extended ?season= to accept a
	// prior season's id.
	//
	// PULL-ONLY BY DESIGN: the lobby widget polls it and the /leaderboard page
	// re-reads it on mount. Standings get no WebSocket push (epic decision), so
	// nothing in ws/events.go corresponds to this route — do not add one.
	api.GET("/leaderboard", seasonHandler.GetLeaderboard)

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
	// Stop the rollover ticker after the hub, per the shutdown order the job
	// documents; waits for any in-flight pass, which is one bounded DB read.
	seasonRollover.Shutdown()
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

// whisperPresenceLocator adapts room.RoomRepository to chat.PresenceLocator for
// the whisper anti-collusion check (Story 11.4). FindPlayerRoom returns the
// user's row for a room in "waiting" OR "playing" status (nil when in neither),
// so a nil result means "not in any active room/match".
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

// inviteFriendDirectory adapts friend.Repository + user.UserRepository to
// room.FriendDirectory for the room-invite panel (Story 11.5). The friend-rows →
// usernames join lives HERE so `room` never imports `friend`: room -> friend
// would be fine today, but keeping the dependency in main.go matches how chat
// and whisper reach across domains and leaves both packages independently
// testable.
type inviteFriendDirectory struct {
	friends friend.Repository
	users   user.UserRepository
}

func (d *inviteFriendDirectory) AreFriends(a, b uint) (bool, error) {
	return d.friends.AreFriends(a, b)
}

// ListFriends returns the viewer's accepted friends resolved to usernames. The
// friendship rows are directional, so the "other" party is whichever side is not
// the viewer. A friend whose user row was soft-deleted is omitted rather than
// surfaced with a blank name.
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
	names := make(map[uint]string, len(users))
	for _, u := range users {
		names[u.ID] = u.Username
	}

	out := make([]room.FriendSummary, 0, len(ids))
	for _, id := range ids {
		name, ok := names[id]
		if !ok {
			continue
		}
		out = append(out, room.FriendSummary{UserID: id, Username: name})
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
