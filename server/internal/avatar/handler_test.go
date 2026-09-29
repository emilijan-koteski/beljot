package avatar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/auth"
	"github.com/emilijan/beljot/server/internal/user"
)

const (
	testJWTSecret = "avatar-test-secret"
	testAssetsURL = "https://assets.test"
	ownerID       = uint(7)
	otherID       = uint(8)
	oldPrefix     = "avatars/11111111-1111-4111-8111-111111111111"
)

var prefixPattern = regexp.MustCompile(`^avatars/[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// --- fakes ----------------------------------------------------------------

type storedObject struct {
	contentType  string
	cacheControl string
	body         []byte
}

// fakeStore is an in-memory Store. failPut makes Put fail for any key with
// that suffix; deleteErr makes every Delete fail (after recording the keys).
type fakeStore struct {
	mu        sync.Mutex
	objects   map[string]storedObject
	puts      []string
	deletes   []string
	failPut   string
	deleteErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string]storedObject{}}
}

func (s *fakeStore) Put(_ context.Context, key, contentType, cacheControl string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts = append(s.puts, key)
	if s.failPut != "" && strings.HasSuffix(key, s.failPut) {
		return errors.New("fake put failure")
	}
	s.objects[key] = storedObject{contentType: contentType, cacheControl: cacheControl, body: body}
	return nil
}

func (s *fakeStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes = append(s.deletes, keys...)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for _, k := range keys {
		delete(s.objects, k)
	}
	return nil
}

// fakeUsers holds each user's avatar prefix. err makes every update fail
// without writing.
type fakeUsers struct {
	mu    sync.Mutex
	keys  map[uint]*string
	err   error
	calls int
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{keys: map[uint]*string{}}
}

func (u *fakeUsers) UpdateAvatarKey(id uint, key *string) (*string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls++
	if u.err != nil {
		return nil, u.err
	}
	previous := u.keys[id]
	u.keys[id] = key
	return previous, nil
}

func (u *fakeUsers) key(id uint) *string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.keys[id]
}

// --- harness --------------------------------------------------------------

// testErrorHandler mirrors main.go's appErrorHandler.
func testErrorHandler(err error, c echo.Context) {
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Status, map[string]any{"error": map[string]string{"code": appErr.Code, "message": appErr.Message}})
		return
	}
	_ = c.JSON(http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": apperr.ErrInternal.Code}})
}

type harness struct {
	h     *Handler
	e     *echo.Echo
	store *fakeStore
	users *fakeUsers
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	user.SetPublicAssetsURL(testAssetsURL)
	t.Cleanup(func() { user.SetPublicAssetsURL("") })

	store := newFakeStore()
	users := newFakeUsers()
	return newHarnessWith(t, store, users, NewHandler(store, users))
}

func newHarnessWith(t *testing.T, store *fakeStore, users *fakeUsers, h *Handler) *harness {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = testErrorHandler
	api := e.Group("/api/v1", auth.AuthMiddleware(testJWTSecret))
	api.PUT("/users/:id/avatar", h.Upload)
	api.DELETE("/users/:id/avatar", h.Remove)
	return &harness{h: h, e: e, store: store, users: users}
}

type part struct {
	field, filename, contentType string
	data                         []byte
}

func multipartBody(t *testing.T, parts ...part) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		hdr := textproto.MIMEHeader{}
		disposition := fmt.Sprintf(`form-data; name=%q`, p.field)
		if p.filename != "" {
			disposition += fmt.Sprintf(`; filename=%q`, p.filename)
		}
		hdr.Set("Content-Disposition", disposition)
		if p.contentType != "" {
			hdr.Set("Content-Type", p.contentType)
		}
		pw, err := w.CreatePart(hdr)
		require.NoError(t, err)
		_, err = pw.Write(p.data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func (hs *harness) do(t *testing.T, method string, id uint, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := auth.GenerateAccessToken(ownerID, testJWTSecret)
	require.NoError(t, err)
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, "/api/v1/users/"+strconv.FormatUint(uint64(id), 10)+"/avatar", body)
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	hs.e.ServeHTTP(rec, req)
	return rec
}

func (hs *harness) upload(t *testing.T, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, part{field: "avatar", filename: "photo.jpg", contentType: "image/jpeg", data: data})
	return hs.do(t, http.MethodPut, ownerID, body, ct)
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), rec.Body.String())
	return resp.Error.Code
}

func uploadResponse(t *testing.T, rec *httptest.ResponseRecorder) UploadResponse {
	t.Helper()
	var resp struct {
		Data UploadResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func pairKeys(prefix string) []string {
	return []string{prefix + "/256.webp", prefix + "/128.webp"}
}

// --- happy paths ----------------------------------------------------------

func TestUpload_StoresTwoImmutableWebPsAndReturnsBothURLs(t *testing.T) {
	hs := newHarness(t)
	rec := hs.upload(t, jpegBytes(t, gradientImage(1600, 1200)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	prefix := hs.users.key(ownerID)
	require.NotNil(t, prefix)
	assert.Regexp(t, prefixPattern, *prefix)

	resp := uploadResponse(t, rec)
	require.NotNil(t, resp.AvatarURL)
	require.NotNil(t, resp.AvatarLargeURL)
	assert.Equal(t, testAssetsURL+"/"+*prefix+"/128.webp", *resp.AvatarURL)
	assert.Equal(t, testAssetsURL+"/"+*prefix+"/256.webp", *resp.AvatarLargeURL)

	// The literal wire keys: the typed decode above would follow a renamed tag.
	var wire map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &wire))
	keys := make([]string, 0, len(wire["data"]))
	for k := range wire["data"] {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"avatarLargeUrl", "avatarUrl"}, keys)

	assert.Equal(t, pairKeys(*prefix), hs.store.puts, "256 first, then 128")
	for key, size := range map[string]int{*prefix + "/256.webp": 256, *prefix + "/128.webp": 128} {
		obj, ok := hs.store.objects[key]
		require.True(t, ok, key)
		assert.Equal(t, "image/webp", obj.contentType)
		assert.Equal(t, "public, max-age=31536000, immutable", obj.cacheControl)
		decodeSquareWebP(t, obj.body, size)
	}
	assert.Empty(t, hs.store.deletes, "nothing to replace")
}

func TestUpload_ReplaceDeletesThePreviousPairAfterTheKeyUpdate(t *testing.T) {
	hs := newHarness(t)
	old := oldPrefix
	hs.users.keys[ownerID] = &old

	rec := hs.upload(t, pngBytes(t, gradientImage(300, 300)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	fresh := hs.users.key(ownerID)
	require.NotNil(t, fresh)
	assert.NotEqual(t, oldPrefix, *fresh, "every upload gets a new prefix")
	assert.Equal(t, pairKeys(oldPrefix), hs.store.deletes)
	assert.Len(t, hs.store.objects, 2, "the new pair stays")
}

func TestUpload_ReplaceDeleteFailureIsLoggedNotFatal(t *testing.T) {
	hs := newHarness(t)
	old := oldPrefix
	hs.users.keys[ownerID] = &old
	hs.store.deleteErr = errors.New("garage unreachable")

	rec := hs.upload(t, pngBytes(t, gradientImage(300, 300)))
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, pairKeys(oldPrefix), hs.store.deletes, "the delete was still attempted")
}

func TestRemove_ClearsTheColumnAndDeletesThePair(t *testing.T) {
	hs := newHarness(t)
	old := oldPrefix
	hs.users.keys[ownerID] = &old

	rec := hs.do(t, http.MethodDelete, ownerID, nil, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Nil(t, hs.users.key(ownerID))
	assert.Equal(t, pairKeys(oldPrefix), hs.store.deletes)
}

func TestRemove_WithoutAvatarIsStill204(t *testing.T) {
	hs := newHarness(t)
	rec := hs.do(t, http.MethodDelete, ownerID, nil, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Nil(t, hs.users.key(ownerID))
	assert.Empty(t, hs.store.deletes)
}

// Exactly 2 MiB, the most the client lets through, must not be rejected for
// its multipart framing (the body cap has 16 KiB of headroom).
func TestUpload_ExactlyTheFileCapIsAccepted(t *testing.T) {
	hs := newHarness(t)
	data := jpegBytes(t, gradientImage(400, 400))
	data = append(data, make([]byte, MaxFileBytes-len(data))...) // trailing bytes after EOI
	require.Len(t, data, MaxFileBytes)

	rec := hs.upload(t, data)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// --- rejections -----------------------------------------------------------

func TestUpload_SomeoneElsesIDIsForbidden(t *testing.T) {
	hs := newHarness(t)
	body, ct := multipartBody(t, part{field: "avatar", data: pngBytes(t, gradientImage(200, 200))})
	rec := hs.do(t, http.MethodPut, otherID, body, ct)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "FORBIDDEN", errorCode(t, rec))

	rec = hs.do(t, http.MethodDelete, otherID, nil, "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "FORBIDDEN", errorCode(t, rec))
	assert.Zero(t, hs.users.calls)
}

func TestUpload_NoFilePart(t *testing.T) {
	hs := newHarness(t)

	body, ct := multipartBody(t, part{field: "picture", filename: "a.png", data: pngBytes(t, gradientImage(200, 200))})
	rec := hs.do(t, http.MethodPut, ownerID, body, ct)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "AVATAR_MISSING", errorCode(t, rec))

	body, ct = multipartBody(t, part{field: "avatar", filename: "empty.png"})
	rec = hs.do(t, http.MethodPut, ownerID, body, ct)
	assert.Equal(t, "AVATAR_MISSING", errorCode(t, rec), "an empty file part")

	rec = hs.do(t, http.MethodPut, ownerID, bytes.NewBufferString(`{"avatar":"x"}`), "application/json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "AVATAR_MISSING", errorCode(t, rec), "not multipart at all")
}

func TestUpload_TooManyBytes(t *testing.T) {
	hs := newHarness(t)

	// The file part one byte over 2 MiB: the body is still under its cap, so
	// the part limit is what trips.
	rec := hs.upload(t, make([]byte, MaxFileBytes+1))
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "AVATAR_TOO_LARGE", errorCode(t, rec))

	// A body over its cap before the avatar part is even reached: the
	// MaxBytesReader is what trips.
	body, ct := multipartBody(t,
		part{field: "padding", data: make([]byte, maxBodyBytes)},
		part{field: "avatar", data: pngBytes(t, gradientImage(200, 200))},
	)
	rec = hs.do(t, http.MethodPut, ownerID, body, ct)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "AVATAR_TOO_LARGE", errorCode(t, rec))
	assert.Empty(t, hs.store.puts)
}

func TestUpload_WrongTypeIsDecidedByBytesNotName(t *testing.T) {
	cases := map[string][]byte{
		"gif":           gifBytes(t),
		"svg":           []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300"></svg>`),
		"pdf":           []byte("%PDF-1.7\n%%EOF"),
		"text":          []byte("hello, I am definitely a jpeg"),
		"animated webp": animatedWebPBytes(t),
		"garbage .jpg":  append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{0x42}, 400)...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			hs := newHarness(t)
			rec := hs.upload(t, data) // always named photo.jpg, declared image/jpeg
			assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
			assert.Equal(t, "AVATAR_UNSUPPORTED_TYPE", errorCode(t, rec))
			assert.Empty(t, hs.store.puts)
		})
	}
}

func TestUpload_TooSmall(t *testing.T) {
	hs := newHarness(t)
	rec := hs.upload(t, pngBytes(t, gradientImage(64, 64)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "AVATAR_TOO_SMALL", errorCode(t, rec))
}

func TestUpload_OversizedDimensionsAreRejectedBeforeDecode(t *testing.T) {
	for name, data := range map[string][]byte{
		"5000 px side": pngHeaderOnly(5000, 400),
		"4000x4001":    pngHeaderOnly(4000, 4001),
	} {
		t.Run(name, func(t *testing.T) {
			hs := newHarness(t)
			// The decode slot is taken: a request that needed it would wait and
			// answer 503. A header-only rejection never asks for it.
			for i := 0; i < decodeSlots; i++ {
				hs.h.slots <- struct{}{}
			}
			hs.h.slotWait = time.Second

			rec := hs.upload(t, data)
			assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
			assert.Equal(t, "AVATAR_DIMENSIONS_TOO_LARGE", errorCode(t, rec))
		})
	}
}

func TestUpload_RateLimitedAfterTenPerRollingHour(t *testing.T) {
	hs := newHarness(t)
	clock := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	hs.h.now = func() time.Time { return clock }
	img := pngBytes(t, gradientImage(160, 160))

	// Rejected files spend no budget.
	for i := 0; i < 12; i++ {
		require.Equal(t, http.StatusBadRequest, hs.upload(t, pngBytes(t, gradientImage(64, 64))).Code)
	}

	for i := 0; i < uploadsPerWindow; i++ {
		clock = clock.Add(time.Minute)
		require.Equal(t, http.StatusOK, hs.upload(t, img).Code, "upload %d", i+1)
	}
	clock = clock.Add(time.Minute)
	rec := hs.upload(t, img)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "AVATAR_UPLOAD_RATE_LIMITED", errorCode(t, rec))

	// DELETE is never limited.
	assert.Equal(t, http.StatusNoContent, hs.do(t, http.MethodDelete, ownerID, nil, "").Code)

	// The window rolls: an hour after the first counted upload, one slot frees.
	clock = time.Date(2026, 9, 29, 11, 1, 0, 1, time.UTC)
	assert.Equal(t, http.StatusOK, hs.upload(t, img).Code)
	assert.Equal(t, http.StatusTooManyRequests, hs.upload(t, img).Code)
}

func TestUpload_OneDecodeInFlight(t *testing.T) {
	assert.Equal(t, 1, decodeSlots)
	assert.Equal(t, 1, cap(NewHandler(newFakeStore(), newFakeUsers()).slots))
}

func TestUpload_BusyWhenTheDecodeSlotDoesNotFreeInTime(t *testing.T) {
	hs := newHarness(t)
	for i := 0; i < decodeSlots; i++ {
		hs.h.slots <- struct{}{}
	}
	hs.h.slotWait = 20 * time.Millisecond

	rec := hs.upload(t, pngBytes(t, gradientImage(200, 200)))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "AVATAR_BUSY", errorCode(t, rec))
	assert.Empty(t, hs.store.puts)
}

// A 503 AVATAR_BUSY is the server's problem, not the user's: it must not spend
// one of the ten hourly uploads.
func TestUpload_BusySpendsNoBudget(t *testing.T) {
	hs := newHarness(t)
	hs.h.slotWait = 5 * time.Millisecond
	img := pngBytes(t, gradientImage(160, 160))

	hs.h.slots <- struct{}{}
	for i := 0; i < uploadsPerWindow+2; i++ {
		require.Equal(t, http.StatusServiceUnavailable, hs.upload(t, img).Code, "busy %d", i+1)
	}
	<-hs.h.slots

	for i := 0; i < uploadsPerWindow; i++ {
		require.Equal(t, http.StatusOK, hs.upload(t, img).Code, "upload %d after the busy spell", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, hs.upload(t, img).Code)
}

// The budget is reserved once the file passes the header checks, so a file
// that only fails in the decode (a valid header over missing pixels) is
// charged: it cost a decode slot.
func TestUpload_UndecodablePixelsSpendBudget(t *testing.T) {
	hs := newHarness(t)
	broken := pngHeaderOnly(300, 300)
	for i := 0; i < uploadsPerWindow; i++ {
		rec := hs.upload(t, broken)
		require.Equal(t, http.StatusUnsupportedMediaType, rec.Code, "broken %d", i+1)
	}
	rec := hs.upload(t, pngBytes(t, gradientImage(160, 160)))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "AVATAR_UPLOAD_RATE_LIMITED", errorCode(t, rec))
}

// A storage failure is the server's fault, so the reservation is refunded.
func TestUpload_PutFailureRefundsBudget(t *testing.T) {
	hs := newHarness(t)
	img := pngBytes(t, gradientImage(160, 160))
	hs.store.failPut = "/256.webp"
	for i := 0; i < uploadsPerWindow+2; i++ {
		require.Equal(t, http.StatusInternalServerError, hs.upload(t, img).Code, "failed put %d", i+1)
	}
	hs.store.failPut = ""
	for i := 0; i < uploadsPerWindow; i++ {
		require.Equal(t, http.StatusOK, hs.upload(t, img).Code, "upload %d after the outage", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, hs.upload(t, img).Code)
}

// An exhausted budget answers 429 before the body is read or the decode slot
// is asked for: with the slot held, a request that got that far would be 503.
func TestUpload_OverBudgetIsRefusedBeforeTheBody(t *testing.T) {
	hs := newHarness(t)
	img := pngBytes(t, gradientImage(160, 160))
	for i := 0; i < uploadsPerWindow; i++ {
		require.Equal(t, http.StatusOK, hs.upload(t, img).Code)
	}
	puts := len(hs.store.puts)

	hs.h.slots <- struct{}{}
	defer func() { <-hs.h.slots }()
	hs.h.slotWait = time.Second

	rec := hs.upload(t, img)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "AVATAR_UPLOAD_RATE_LIMITED", errorCode(t, rec))
	assert.Len(t, hs.store.puts, puts, "nothing was written")
}

// A second upload from a user who already has one in flight is refused at
// once, and spends no budget.
func TestUpload_OneInFlightPerUser(t *testing.T) {
	hs := newHarness(t)
	require.True(t, hs.h.admit.enter(ownerID)) // the upload already in flight

	rec := hs.upload(t, pngBytes(t, gradientImage(160, 160)))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "AVATAR_BUSY", errorCode(t, rec))
	assert.Empty(t, hs.store.puts)
	assert.Empty(t, hs.h.limiter.hits[ownerID], "no budget spent")

	hs.h.admit.leave(ownerID)
	assert.Equal(t, http.StatusOK, hs.upload(t, pngBytes(t, gradientImage(160, 160))).Code)
}

// A fourth concurrent upload process-wide is refused at once.
func TestUpload_ThreeInFlightPerProcess(t *testing.T) {
	hs := newHarness(t)
	for _, other := range []uint{100, 101, 102} {
		require.True(t, hs.h.admit.enter(other))
	}

	rec := hs.upload(t, pngBytes(t, gradientImage(160, 160)))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "AVATAR_BUSY", errorCode(t, rec))
	assert.Empty(t, hs.h.limiter.hits[ownerID], "no budget spent")

	hs.h.admit.leave(101)
	assert.Equal(t, http.StatusOK, hs.upload(t, pngBytes(t, gradientImage(160, 160))).Code)
}

// Every way out of Upload after admission releases the place.
func TestUpload_AdmissionReleasedOnEveryReturn(t *testing.T) {
	img := pngBytes(t, gradientImage(160, 160))
	cases := []struct {
		name  string
		setup func(hs *harness)
		body  func(t *testing.T) (*bytes.Buffer, string)
		want  int
	}{
		{"success", nil, nil, http.StatusOK},
		{"missing part", nil, func(t *testing.T) (*bytes.Buffer, string) {
			return multipartBody(t, part{field: "other", data: img})
		}, http.StatusBadRequest},
		{"too large", nil, func(t *testing.T) (*bytes.Buffer, string) {
			return multipartBody(t, part{field: "avatar", data: make([]byte, MaxFileBytes+1)})
		}, http.StatusRequestEntityTooLarge},
		{"unsupported", nil, func(t *testing.T) (*bytes.Buffer, string) {
			return multipartBody(t, part{field: "avatar", data: []byte("not an image")})
		}, http.StatusUnsupportedMediaType},
		{"undecodable pixels", nil, func(t *testing.T) (*bytes.Buffer, string) {
			return multipartBody(t, part{field: "avatar", data: pngHeaderOnly(300, 300)})
		}, http.StatusUnsupportedMediaType},
		{"busy", func(hs *harness) {
			hs.h.slots <- struct{}{}
			hs.h.slotWait = 5 * time.Millisecond
		}, nil, http.StatusServiceUnavailable},
		{"put failure", func(hs *harness) { hs.store.failPut = "/128.webp" }, nil, http.StatusInternalServerError},
		{"key update failure", func(hs *harness) { hs.users.err = errors.New("db down") }, nil, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := newHarness(t)
			if tc.setup != nil {
				tc.setup(hs)
			}
			var rec *httptest.ResponseRecorder
			if tc.body == nil {
				rec = hs.upload(t, img)
			} else {
				body, ct := tc.body(t)
				rec = hs.do(t, http.MethodPut, ownerID, body, ct)
			}
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			assert.Zero(t, hs.h.admit.inFlight(), "the admission place is released")
		})
	}
}

func TestServerFault(t *testing.T) {
	assert.True(t, serverFault(apperr.ErrAvatarBusy))
	assert.True(t, serverFault(context.Canceled))
	assert.True(t, serverFault(fmt.Errorf("uploading avatar: %w", errors.New("s3 down"))))
	assert.False(t, serverFault(apperr.ErrAvatarUnsupportedType))
	assert.False(t, serverFault(fmt.Errorf("saving avatar key: %w", apperr.ErrUserNotFound)))
}

func TestHandler_StorageDisabledAnswers503(t *testing.T) {
	users := newFakeUsers()
	hs := newHarnessWith(t, nil, users, NewHandler(nil, users))

	rec := hs.upload(t, pngBytes(t, gradientImage(200, 200)))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "AVATAR_STORAGE_UNAVAILABLE", errorCode(t, rec))

	rec = hs.do(t, http.MethodDelete, ownerID, nil, "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "AVATAR_STORAGE_UNAVAILABLE", errorCode(t, rec))
	assert.Zero(t, users.calls)
}

// --- failures after the first write --------------------------------------

func TestUpload_PutFailureLeavesNoOrphansAndNoKeyChange(t *testing.T) {
	for _, failing := range []string{"/256.webp", "/128.webp"} {
		t.Run(failing, func(t *testing.T) {
			hs := newHarness(t)
			old := oldPrefix
			hs.users.keys[ownerID] = &old
			hs.store.failPut = failing

			rec := hs.upload(t, pngBytes(t, gradientImage(200, 200)))
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, "INTERNAL_ERROR", errorCode(t, rec))

			assert.Zero(t, hs.users.calls, "the key is never touched")
			assert.Equal(t, oldPrefix, *hs.users.key(ownerID))
			assert.Empty(t, hs.store.objects, "whatever was written is deleted again")
			assert.NotContains(t, hs.store.deletes, oldPrefix+"/256.webp", "the old pair is kept")
		})
	}
}

func TestUpload_KeyUpdateFailureDeletesTheFreshPair(t *testing.T) {
	hs := newHarness(t)
	hs.users.err = errors.New("connection reset")

	rec := hs.upload(t, pngBytes(t, gradientImage(200, 200)))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "INTERNAL_ERROR", errorCode(t, rec))
	assert.Len(t, hs.store.puts, 2)
	assert.Empty(t, hs.store.objects, "both fresh objects are deleted")
	assert.ElementsMatch(t, hs.store.puts, hs.store.deletes)
}

// --- helpers --------------------------------------------------------------

func TestNewPrefixIsAVersion4UUID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		p, err := newPrefix()
		require.NoError(t, err)
		assert.Regexp(t, prefixPattern, p)
		assert.False(t, seen[p], "prefixes never repeat")
		seen[p] = true
	}
}

func TestUploadLimiter(t *testing.T) {
	l := newUploadLimiter(2, time.Hour)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	took := func(user uint, at time.Time) bool {
		_, ok := l.take(user, at)
		return ok
	}
	assert.True(t, took(1, t0))
	assert.True(t, took(1, t0.Add(time.Minute)))
	assert.False(t, l.allow(1, t0.Add(2*time.Minute)))
	assert.False(t, took(1, t0.Add(2*time.Minute)))
	assert.True(t, took(2, t0), "budgets are per user")
	assert.True(t, l.allow(1, t0.Add(time.Hour+time.Second)), "the oldest hit left the window")
	l.recent(2, t0.Add(2*time.Hour))
	_, kept := l.hits[2]
	assert.False(t, kept, "an idle user's entry is dropped")
}

func TestUploadLimiter_RefundHandsBackExactlyOneHit(t *testing.T) {
	l := newUploadLimiter(2, time.Hour)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first, ok := l.take(1, t0)
	require.True(t, ok)
	_, ok = l.take(1, t0.Add(time.Minute))
	require.True(t, ok)
	assert.False(t, l.allow(1, t0.Add(2*time.Minute)))

	l.refund(1, first)
	assert.True(t, l.allow(1, t0.Add(2*time.Minute)), "the refunded hit frees one upload")
	assert.Len(t, l.hits[1], 1)

	l.refund(1, first) // already gone: no-op
	assert.Len(t, l.hits[1], 1)
}

func TestAdmission(t *testing.T) {
	a := newAdmission(1, 3)
	assert.True(t, a.enter(1))
	assert.False(t, a.enter(1), "one upload per user")
	assert.True(t, a.enter(2))
	assert.True(t, a.enter(3))
	assert.False(t, a.enter(4), "three per process")
	a.leave(1)
	assert.True(t, a.enter(4))
	assert.Equal(t, 3, a.inFlight())
	for _, u := range []uint{2, 3, 4} {
		a.leave(u)
	}
	assert.Zero(t, a.inFlight())
	assert.Empty(t, a.inUser)
}
