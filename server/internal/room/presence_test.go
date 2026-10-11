package room_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/room"
)

func TestPresenceRegistry_AddPresentRemoveClear(t *testing.T) {
	r := room.NewPresenceRegistry()

	assert.Empty(t, r.Present(1))

	r.Add(1, 200)
	r.Add(1, 100)
	r.Add(1, 200)
	assert.Equal(t, []uint{100, 200}, r.Present(1))

	r.Add(2, 999)
	assert.Equal(t, []uint{100, 200}, r.Present(1))
	assert.Equal(t, []uint{999}, r.Present(2))

	r.Remove(1, 100)
	assert.Equal(t, []uint{200}, r.Present(1))
	r.Remove(1, 200)
	assert.Empty(t, r.Present(1))
	r.Remove(1, 12345)

	r.Add(3, 1)
	r.Add(3, 2)
	r.Clear(3)
	assert.Empty(t, r.Present(3))
}

// Concurrent access must not race (run with -race).
func TestPresenceRegistry_ConcurrentAccess(t *testing.T) {
	r := room.NewPresenceRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			uid := uint(n)
			r.Add(1, uid)
			_ = r.Present(1)
			r.Remove(1, uid)
		}(i)
	}
	wg.Wait()
	assert.Empty(t, r.Present(1))
}
