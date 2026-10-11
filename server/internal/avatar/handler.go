package avatar

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/user"
)

// Users is the repository write the handler needs: UpdateAvatarKey sets or (nil) clears the prefix and returns the one it replaced.
// It is kept out of user.UserRepository so that interface's many test mocks need not grow.
type Users interface {
	UpdateAvatarKey(id uint, key *string) (previous *string, err error)
}

const (
	formField              = "avatar"
	multipartOverheadBytes = 16 << 10
	maxBodyBytes           = MaxFileBytes + multipartOverheadBytes

	decodeSlots    = 1
	decodeSlotWait = 10 * time.Second

	uploadsInFlightPerUser = 1
	uploadsInFlight        = 3

	bodyReadTimeout = 30 * time.Second

	uploadsPerWindow = 10
	uploadWindow     = time.Hour

	keyPrefix = "avatars/"

	contentTypeWebP       = "image/webp"
	cacheControlImmutable = "public, max-age=31536000, immutable"
)

// UploadResponse is the body of a successful PUT: the two derived public URLs
// of the new avatar. Nil (null) only when no public base URL is configured.
type UploadResponse struct {
	AvatarURL      *string `json:"avatarUrl"`
	AvatarLargeURL *string `json:"avatarLargeUrl"`
}

// Handler serves PUT and DELETE /users/:id/avatar; a nil store disables both routes with 503.
// Decodes run one at a time, since an accepted source may need maxDecodeBytes against a 256 MiB container limit.
type Handler struct {
	store    Store
	users    Users
	slots    chan struct{}
	slotWait time.Duration
	limiter  *uploadLimiter
	admit    *admission
	now      func() time.Time
}

// NewHandler wires the handler. Pass a nil store (an untyped nil, not a typed
// nil pointer) when object storage is not configured.
func NewHandler(store Store, users Users) *Handler {
	return &Handler{
		store:    store,
		users:    users,
		slots:    make(chan struct{}, decodeSlots),
		slotWait: decodeSlotWait,
		limiter:  newUploadLimiter(uploadsPerWindow, uploadWindow),
		admit:    newAdmission(uploadsInFlightPerUser, uploadsInFlight),
		now:      time.Now,
	}
}

// Upload handles PUT /users/:id/avatar, storing a re-encoded pair under a fresh prefix and recording it.
// The replaced pair is deleted only after the new one is recorded, and every cleanup delete is best-effort.
func (h *Handler) Upload(c echo.Context) (err error) {
	userID, err := authorize(c)
	if err != nil {
		return err
	}
	if h.store == nil {
		return apperr.ErrAvatarStorageUnavailable
	}
	if !h.limiter.allow(userID, h.now()) {
		return apperr.ErrAvatarUploadRateLimited
	}
	if !h.admit.enter(userID) {
		return apperr.ErrAvatarBusy
	}
	defer h.admit.leave(userID)

	data, err := readAvatarPartWithin(c, bodyReadTimeout)
	if err != nil {
		return err
	}
	src, err := Inspect(data)
	if err != nil {
		return err
	}

	reservedAt, ok := h.limiter.take(userID, h.now())
	if !ok {
		return apperr.ErrAvatarUploadRateLimited
	}
	defer func() {
		if err != nil && serverFault(err) {
			h.limiter.refund(userID, reservedAt)
		}
	}()

	ctx := c.Request().Context()
	large, small, err := h.render(ctx, src)
	if err != nil {
		return err
	}

	prefix, err := newPrefix()
	if err != nil {
		return fmt.Errorf("generating avatar prefix: %w", err)
	}
	if err := h.putPair(ctx, prefix, large, small); err != nil {
		h.deletePair(ctx, userID, prefix, "cleanup after failed upload")
		return fmt.Errorf("uploading avatar: %w", err)
	}

	previous, err := h.users.UpdateAvatarKey(userID, &prefix)
	if err != nil {
		h.deletePair(ctx, userID, prefix, "cleanup after failed key update")
		return fmt.Errorf("saving avatar key: %w", err)
	}
	if previous != nil && *previous != "" && *previous != prefix {
		h.deletePair(ctx, userID, *previous, "replaced by a new upload")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": UploadResponse{
			AvatarURL:      user.AvatarURL(&prefix, user.AvatarSizeSmall),
			AvatarLargeURL: user.AvatarURL(&prefix, user.AvatarSizeLarge),
		},
	})
}

// Remove handles DELETE /users/:id/avatar: it clears the column, then deletes the pair best-effort.
// It answers 204 whether or not an avatar was set and is not rate limited.
func (h *Handler) Remove(c echo.Context) error {
	userID, err := authorize(c)
	if err != nil {
		return err
	}
	if h.store == nil {
		return apperr.ErrAvatarStorageUnavailable
	}

	previous, err := h.users.UpdateAvatarKey(userID, nil)
	if err != nil {
		return fmt.Errorf("clearing avatar key: %w", err)
	}
	if previous != nil && *previous != "" {
		h.deletePair(c.Request().Context(), userID, *previous, "removed by the user")
	}
	return c.NoContent(http.StatusNoContent)
}

// authorize returns the caller's id when it equals the :id path param, the
// same self-only rule as PATCH /users/:id/username.
func authorize(c echo.Context) (uint, error) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return 0, apperr.ErrUnauthorized
	}
	paramID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || paramID == 0 {
		return 0, apperr.ErrBadRequest
	}
	if paramID != uint64(userID) {
		return 0, apperr.ErrForbidden
	}
	return userID, nil
}

// serverFault reports whether a failed upload was the server's doing rather
// than the file's: anything but a 4xx AppError (so 503 busy, a cancelled
// context and every 5xx count; a 415 for undecodable pixels does not).
func serverFault(err error) bool {
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		return appErr.Status >= 500
	}
	return true
}

// readAvatarPartWithin reads the avatar part under a connection read deadline, so a slow sender cannot hold an admission place.
// The deadline is lifted afterwards, and writers that cannot set one are read without it.
func readAvatarPartWithin(c echo.Context, timeout time.Duration) ([]byte, error) {
	rc := http.NewResponseController(c.Response())
	if err := rc.SetReadDeadline(time.Now().Add(timeout)); err == nil {
		defer func() { _ = rc.SetReadDeadline(time.Time{}) }()
	}
	return readAvatarPart(c)
}

// readAvatarPart streams the multipart body to the "avatar" part and returns its bytes, skipping other parts.
// The body and the part are both capped, and either overflow is 413.
func readAvatarPart(c echo.Context) ([]byte, error) {
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response(), req.Body, maxBodyBytes)

	mr, err := req.MultipartReader()
	if err != nil {
		return nil, apperr.ErrAvatarMissing
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, apperr.ErrAvatarMissing
		}
		if err != nil {
			return nil, bodyError(err)
		}
		if part.FormName() != formField {
			if _, err := io.Copy(io.Discard, part); err != nil {
				return nil, bodyError(err)
			}
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, MaxFileBytes+1))
		if err != nil {
			return nil, bodyError(err)
		}
		if len(data) > MaxFileBytes {
			return nil, apperr.ErrAvatarTooLarge
		}
		if len(data) == 0 {
			return nil, apperr.ErrAvatarMissing
		}
		return data, nil
	}
}

// bodyError maps a failure while reading the request body: the body cap is
// 413, anything else is a malformed request.
func bodyError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return apperr.ErrAvatarTooLarge
	}
	return apperr.ErrBadRequest
}

// render runs the decode under the process-wide slot semaphore. Only the
// decode and encode hold a slot; the uploads that follow do not.
func (h *Handler) render(ctx context.Context, src *Source) (large, small []byte, err error) {
	timer := time.NewTimer(h.slotWait)
	defer timer.Stop()
	select {
	case h.slots <- struct{}{}:
	case <-timer.C:
		return nil, nil, apperr.ErrAvatarBusy
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	defer func() { <-h.slots }()
	return src.Render()
}

// putPair writes the 256 then the 128 derivative under prefix.
func (h *Handler) putPair(ctx context.Context, prefix string, large, small []byte) error {
	if err := h.store.Put(ctx, user.AvatarObjectKey(prefix, user.AvatarSizeLarge), contentTypeWebP, cacheControlImmutable, large); err != nil {
		return err
	}
	return h.store.Put(ctx, user.AvatarObjectKey(prefix, user.AvatarSizeSmall), contentTypeWebP, cacheControlImmutable, small)
}

// deletePair removes both derivatives under prefix, best-effort: a failure is
// logged and never fails the request. It runs detached from the request's
// cancellation, so a client that hangs up cannot leave the cleanup half done.
func (h *Handler) deletePair(ctx context.Context, userID uint, prefix, reason string) {
	if !strings.HasPrefix(prefix, keyPrefix) {
		slog.Warn("avatar: not deleting objects outside the avatar prefix", "userID", userID, "prefix", prefix, "reason", reason)
		return
	}
	keys := []string{
		user.AvatarObjectKey(prefix, user.AvatarSizeLarge),
		user.AvatarObjectKey(prefix, user.AvatarSizeSmall),
	}
	if err := h.store.Delete(context.WithoutCancel(ctx), keys...); err != nil {
		slog.Error("avatar: best-effort delete failed", "userID", userID, "prefix", prefix, "reason", reason, "error", err)
	}
}

// newPrefix returns "avatars/<uuid v4>": 122 random bits, so a key can be
// neither guessed nor enumerated.
func newPrefix() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s%x-%x-%x-%x-%x", keyPrefix, b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// uploadLimiter is a per-user sliding-window counter: at most limit uploads
// in any window-long interval. Process-local, like the emote limiter.
type uploadLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[uint][]time.Time
}

func newUploadLimiter(limit int, window time.Duration) *uploadLimiter {
	return &uploadLimiter{limit: limit, window: window, hits: make(map[uint][]time.Time)}
}

// allow reports whether userID has budget left, without spending any.
func (l *uploadLimiter) allow(userID uint, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(userID, now)) < l.limit
}

// take spends one upload from userID's budget and returns the hit it recorded, or false when none is left.
// Check and record share one lock, so concurrent uploads cannot overdraw it.
func (l *uploadLimiter) take(userID uint, now time.Time) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.recent(userID, now)
	if len(recent) >= l.limit {
		return time.Time{}, false
	}
	l.hits[userID] = append(recent, now)
	return now, true
}

// refund hands back the hit take recorded at `at`, for an upload that failed
// through no fault of the user's. A hit that already aged out is a no-op.
func (l *uploadLimiter) refund(userID uint, at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	hits := l.hits[userID]
	for i, t := range hits {
		if t.Equal(at) {
			hits = append(hits[:i], hits[i+1:]...)
			break
		}
	}
	if len(hits) == 0 {
		delete(l.hits, userID)
		return
	}
	l.hits[userID] = hits
}

// admission bounds uploads in flight, per user and per process, before any body is read.
// Each admitted upload buffers its body while it waits for the decode slot, so an unbounded queue is a memory problem.
type admission struct {
	mu      sync.Mutex
	perUser int
	total   int
	inUser  map[uint]int
	inTotal int
}

func newAdmission(perUser, total int) *admission {
	return &admission{perUser: perUser, total: total, inUser: make(map[uint]int)}
}

// enter admits one upload for userID, or reports false when the user or the
// process is at its cap. Every true must be paired with one leave.
func (a *admission) enter(userID uint) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.inTotal >= a.total || a.inUser[userID] >= a.perUser {
		return false
	}
	a.inUser[userID]++
	a.inTotal++
	return true
}

func (a *admission) leave(userID uint) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.inUser[userID] <= 1 {
		delete(a.inUser, userID)
	} else {
		a.inUser[userID]--
	}
	a.inTotal--
}

// inFlight is the number of admitted uploads (tests).
func (a *admission) inFlight() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inTotal
}

// recent prunes userID's hits older than the window, returns the rest and deletes the entry once empty.
// Caller holds mu.
func (l *uploadLimiter) recent(userID uint, now time.Time) []time.Time {
	hits := l.hits[userID]
	cutoff := now.Add(-l.window)
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, userID)
		return nil
	}
	l.hits[userID] = kept
	return kept
}
