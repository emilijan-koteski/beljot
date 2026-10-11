package room_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/room"
)

func TestInviteRegistry_IssueAndConsumeHostGrantOnce(t *testing.T) {
	r := room.NewInviteRegistry()

	assert.False(t, r.Pending(1, 42))
	assert.False(t, r.ConsumeHostGrant(1, 42))

	id, expiresAt := r.Issue(1, 42, 7, true)
	assert.NotZero(t, id)
	assert.True(t, expiresAt.After(time.Now()))
	assert.True(t, r.Pending(1, 42))

	assert.True(t, r.ConsumeHostGrant(1, 42))
	assert.False(t, r.ConsumeHostGrant(1, 42))
	assert.False(t, r.Pending(1, 42))
}

func TestInviteRegistry_NonHostGrantIsNeverConsumable(t *testing.T) {
	r := room.NewInviteRegistry()

	r.Issue(1, 42, 7, false)

	assert.True(t, r.Pending(1, 42))
	assert.False(t, r.ConsumeHostGrant(1, 42))
	assert.True(t, r.Pending(1, 42), "a rejected non-host consume must not void the invite")
}

func TestInviteRegistry_GrantsAreScopedToRoomAndInvitee(t *testing.T) {
	r := room.NewInviteRegistry()

	r.Issue(1, 42, 7, true)

	assert.False(t, r.ConsumeHostGrant(2, 42))
	assert.False(t, r.ConsumeHostGrant(1, 43))
	assert.True(t, r.ConsumeHostGrant(1, 42))
}

func TestInviteRegistry_ExpiryVoidsTheGrant(t *testing.T) {
	r := room.NewInviteRegistryWithTTL(20 * time.Millisecond)

	r.Issue(1, 42, 7, true)
	assert.True(t, r.Pending(1, 42))

	time.Sleep(60 * time.Millisecond)

	assert.False(t, r.Pending(1, 42))
	assert.False(t, r.ConsumeHostGrant(1, 42), "an expired grant must never bypass the password")
}

func TestInviteRegistry_ReissueReplacesAndRefreshesTheGrant(t *testing.T) {
	r := room.NewInviteRegistryWithTTL(60 * time.Millisecond)

	first, _ := r.Issue(1, 42, 7, true)
	time.Sleep(40 * time.Millisecond)
	second, _ := r.Issue(1, 42, 7, true)
	assert.NotEqual(t, first, second, "each invite gets its own id")

	time.Sleep(40 * time.Millisecond)
	assert.True(t, r.Pending(1, 42))
	assert.True(t, r.ConsumeHostGrant(1, 42))
}

func TestInviteRegistry_VoidAndVoidRoomAndVoidUser(t *testing.T) {
	r := room.NewInviteRegistry()

	r.Issue(1, 42, 7, true)
	r.Void(1, 42)
	assert.False(t, r.ConsumeHostGrant(1, 42))
	r.Void(1, 42)

	r.Issue(1, 42, 7, true)
	r.Issue(1, 43, 7, true)
	r.Issue(2, 42, 9, true)
	r.VoidRoom(1)
	assert.False(t, r.ConsumeHostGrant(1, 42))
	assert.False(t, r.ConsumeHostGrant(1, 43))
	assert.True(t, r.ConsumeHostGrant(2, 42), "VoidRoom must not touch other rooms")

	r.Issue(1, 42, 7, true)
	r.Issue(2, 42, 9, true)
	r.Issue(2, 43, 9, true)
	r.VoidUser(42)
	assert.False(t, r.ConsumeHostGrant(1, 42))
	assert.False(t, r.ConsumeHostGrant(2, 42))
	assert.True(t, r.ConsumeHostGrant(2, 43), "VoidUser must not touch other invitees")
}

// Concurrent access must not race (run with -race).
func TestInviteRegistry_ConcurrentAccess(t *testing.T) {
	r := room.NewInviteRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			uid := uint(n)
			r.Issue(1, uid, 7, true)
			_ = r.Pending(1, uid)
			_ = r.ConsumeHostGrant(1, uid)
			r.Void(1, uid)
		}(i)
	}
	wg.Wait()
	for i := 0; i < 50; i++ {
		assert.False(t, r.Pending(1, uint(i)))
	}
}
