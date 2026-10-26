package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/friend"
	"github.com/emilijan/beljot/server/internal/room"
	"github.com/emilijan/beljot/server/internal/user"
)

// fakeFriendRows serves ListAccepted only; the embedded interface panics on
// anything else, which this adapter must never call.
type fakeFriendRows struct {
	friend.Repository
	rows []friend.Friendship
}

func (f *fakeFriendRows) ListAccepted(uint) ([]friend.Friendship, error) { return f.rows, nil }

// fakeUserRows serves FindManyByIDs only.
type fakeUserRows struct {
	user.UserRepository
	users []user.User
}

func (f *fakeUserRows) FindManyByIDs(ids []uint) ([]user.User, error) {
	wanted := make(map[uint]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	var out []user.User
	for _, u := range f.users {
		if wanted[u.ID] {
			out = append(out, u)
		}
	}
	return out, nil
}

// The invite panel's friend rows carry each friend's 128 avatar URL, derived
// from the stored prefix, and null for a friend without one.
func TestInviteFriendDirectory_ListFriendsCarriesAvatarURL(t *testing.T) {
	user.SetPublicAssetsURL("https://assets.test")
	t.Cleanup(func() { user.SetPublicAssetsURL("") })

	prefix := "avatars/99999999-9999-4999-8999-999999999999"
	dir := &inviteFriendDirectory{
		friends: &fakeFriendRows{rows: []friend.Friendship{
			{UserID: 1, FriendID: 2, Status: friend.FriendStatusAccepted},
			{UserID: 3, FriendID: 1, Status: friend.FriendStatusAccepted},
		}},
		users: &fakeUserRows{users: []user.User{
			{ID: 2, Username: "pictured", AvatarKey: &prefix},
			{ID: 3, Username: "plain"},
		}},
	}

	got, err := dir.ListFriends(1)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byID := map[uint]*string{}
	for _, f := range got {
		byID[f.UserID] = f.AvatarURL
	}
	require.NotNil(t, byID[2])
	assert.Equal(t, "https://assets.test/"+prefix+"/128.webp", *byID[2])
	assert.Nil(t, byID[3])
}

func TestInviteFriendDirectory_FindPlayersCarriesInvitePreference(t *testing.T) {
	user.SetPublicAssetsURL("https://assets.test")
	t.Cleanup(func() { user.SetPublicAssetsURL("") })

	prefix := "avatars/88888888-8888-4888-8888-888888888888"
	dir := &inviteFriendDirectory{
		users: &fakeUserRows{users: []user.User{
			{ID: 2, Username: "open", AvatarKey: &prefix, AllowNonFriendInvites: true},
			{ID: 3, Username: "private", AllowNonFriendInvites: false},
		}},
	}

	got, err := dir.FindPlayers([]uint{2, 3, 4})
	require.NoError(t, err)
	require.Len(t, got, 2, "an unknown or deleted user is omitted")

	byID := map[uint]room.PlayerSummary{}
	for _, p := range got {
		byID[p.UserID] = p
	}
	assert.Equal(t, "open", byID[2].Username)
	assert.True(t, byID[2].AllowsNonFriendInvites)
	require.NotNil(t, byID[2].AvatarURL)
	assert.Equal(t, "https://assets.test/"+prefix+"/128.webp", *byID[2].AvatarURL)
	assert.False(t, byID[3].AllowsNonFriendInvites)
	assert.Nil(t, byID[3].AvatarURL)
}

func TestInviteFriendDirectory_FindPlayersWithNoIDsIsAnEmptySlice(t *testing.T) {
	dir := &inviteFriendDirectory{users: &fakeUserRows{}}

	got, err := dir.FindPlayers(nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}
