package user

import (
	"strconv"
	"sync/atomic"
)

const (
	AvatarSizeLarge = 256
	AvatarSizeSmall = 128
)

var publicAssetsBase atomic.Value

// SetPublicAssetsURL sets the base every avatar URL is derived from. Called
// once at startup, before the server accepts requests.
func SetPublicAssetsURL(base string) {
	publicAssetsBase.Store(base)
}

// AvatarObjectKey is the object key of one derivative under a stored prefix:
// "<prefix>/<size>.webp". The avatar package writes with it and AvatarURL
// reads with it, so the two can never disagree on the layout.
func AvatarObjectKey(prefix string, size int) string {
	return prefix + "/" + strconv.Itoa(size) + ".webp"
}

// AvatarURL derives the public URL of one derivative of a stored avatar prefix:
// "<base>/<prefix>/<size>.webp". It returns nil when the user has no avatar or
// no public base is configured, so the wire value is null in both cases.
func AvatarURL(key *string, size int) *string {
	if key == nil || *key == "" {
		return nil
	}
	base, _ := publicAssetsBase.Load().(string)
	if base == "" {
		return nil
	}
	url := base + "/" + AvatarObjectKey(*key, size)
	return &url
}

// SmallAvatarURL is AvatarURL at the list/seat size, the one every identity
// DTO except the profile hero carries.
func SmallAvatarURL(key *string) *string {
	return AvatarURL(key, AvatarSizeSmall)
}
