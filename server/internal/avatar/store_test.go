package avatar

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	method string
	path   string
	header http.Header
	body   []byte
}

// fakeS3 records every request and answers with the status `respond` picks
// (default: 200 for PUT, 204 for DELETE).
type fakeS3 struct {
	mu       sync.Mutex
	requests []recordedRequest
	respond  func(r *http.Request) int
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body})
	f.mu.Unlock()

	status := http.StatusOK
	if r.Method == http.MethodDelete {
		status = http.StatusNoContent
	}
	if f.respond != nil {
		status = f.respond(r)
	}
	if status >= 400 {
		// A non-retryable S3 error, so the SDK makes exactly one attempt.
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
		return
	}
	w.Header().Set("ETag", `"abc"`)
	w.WriteHeader(status)
}

func newTestS3Store(t *testing.T, fake *fakeS3) *S3Store {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store, err := NewS3Store(context.Background(), S3Config{
		Endpoint:  srv.URL,
		Region:    "garage",
		AccessKey: "GKtest",
		SecretKey: "secret",
		Bucket:    "beljot-public",
	})
	require.NoError(t, err)
	return store
}

// PutObject goes path-style to /<bucket>/<key> with both headers and the
// body as-is: no aws-chunked encoding and no checksum headers or trailers,
// which not every S3-compatible store accepts.
func TestS3Store_PutIsPathStyleWithPlainBody(t *testing.T) {
	fake := &fakeS3{}
	store := newTestS3Store(t, fake)
	key := "avatars/0f8b6c1e-2a4d-4e6f-9a1b-3c5d7e9f1a2b/256.webp"
	body := []byte("RIFF....WEBPVP8 fake")

	require.NoError(t, store.Put(context.Background(), key, contentTypeWebP, cacheControlImmutable, body))

	require.Len(t, fake.requests, 1)
	req := fake.requests[0]
	assert.Equal(t, http.MethodPut, req.method)
	assert.Equal(t, "/beljot-public/"+key, req.path, "path-style addressing")
	assert.Equal(t, "image/webp", req.header.Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", req.header.Get("Cache-Control"))
	assert.Equal(t, body, req.body)
	assert.NotContains(t, req.header.Get("Content-Encoding"), "aws-chunked")
	for name := range req.header {
		lower := strings.ToLower(name)
		assert.False(t, strings.HasPrefix(lower, "x-amz-checksum"), "unexpected %s", name)
		assert.NotEqual(t, "x-amz-trailer", lower)
		assert.NotEqual(t, "x-amz-sdk-checksum-algorithm", lower)
	}
	assert.NotEmpty(t, req.header.Get("Authorization"), "requests are signed")
}

func TestS3Store_PutFailureIsReturned(t *testing.T) {
	fake := &fakeS3{respond: func(*http.Request) int { return http.StatusForbidden }}
	store := newTestS3Store(t, fake)
	err := store.Put(context.Background(), "avatars/x/128.webp", contentTypeWebP, cacheControlImmutable, []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "avatars/x/128.webp")
}

// Delete issues one DeleteObject per key, keeps going after a failure, and
// joins the failures into one error.
func TestS3Store_DeleteIsOneRequestPerKeyAndJoinsErrors(t *testing.T) {
	first, second := "avatars/p/256.webp", "avatars/p/128.webp"

	t.Run("all succeed", func(t *testing.T) {
		fake := &fakeS3{}
		store := newTestS3Store(t, fake)
		require.NoError(t, store.Delete(context.Background(), first, second))
		require.Len(t, fake.requests, 2)
		for i, key := range []string{first, second} {
			assert.Equal(t, http.MethodDelete, fake.requests[i].method)
			assert.Equal(t, "/beljot-public/"+key, fake.requests[i].path)
		}
	})

	t.Run("first fails, second still tried", func(t *testing.T) {
		fake := &fakeS3{respond: func(r *http.Request) int {
			if strings.HasSuffix(r.URL.Path, "/256.webp") {
				return http.StatusForbidden
			}
			return http.StatusNoContent
		}}
		store := newTestS3Store(t, fake)
		err := store.Delete(context.Background(), first, second)
		require.Error(t, err)
		assert.Contains(t, err.Error(), first)
		assert.NotContains(t, err.Error(), second)
		require.Len(t, fake.requests, 2, "the second key is attempted after the first failed")
		assert.Equal(t, "/beljot-public/"+second, fake.requests[1].path)
	})

	t.Run("both fail", func(t *testing.T) {
		fake := &fakeS3{respond: func(*http.Request) int { return http.StatusForbidden }}
		store := newTestS3Store(t, fake)
		err := store.Delete(context.Background(), first, second)
		require.Error(t, err)
		assert.Contains(t, err.Error(), first)
		assert.Contains(t, err.Error(), second, "both failures are joined")
	})
}
