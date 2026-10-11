//go:build !amd64 && !arm64

package main

var _ int = "32-bit build detected — see deferred-work.md D86"
