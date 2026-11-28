package auth

import (
	"sync"
	"time"
)

// RevokedUsers remembers erased users until every access token minted before the erasure has expired.
// It lives in memory, which is enough for a single API instance.
type RevokedUsers struct {
	mu    sync.Mutex
	until map[uint]time.Time
	ttl   time.Duration
	now   func() time.Time
}

// NewRevokedUsers keeps each entry for ttl, which must be at least the access-token lifetime.
func NewRevokedUsers(ttl time.Duration) *RevokedUsers {
	return &RevokedUsers{until: make(map[uint]time.Time), ttl: ttl, now: time.Now}
}

// Revoke rejects every access token of userID from now until the ttl elapses.
func (r *RevokedUsers) Revoke(userID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.until[userID] = r.now().Add(r.ttl)
}

// Unrevoke lifts a revocation whose erasure did not go through.
func (r *RevokedUsers) Unrevoke(userID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.until, userID)
}

// IsRevoked reports whether userID was revoked within the ttl, pruning expired entries as it goes.
func (r *RevokedUsers) IsRevoked(userID uint) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for id, until := range r.until {
		if now.After(until) {
			delete(r.until, id)
		}
	}
	_, revoked := r.until[userID]
	return revoked
}
