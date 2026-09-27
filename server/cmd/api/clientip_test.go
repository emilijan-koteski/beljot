package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPExtractor(t *testing.T) {
	extract := clientIPExtractor()
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"traefik on the overlay behind cloudflare", "10.0.1.5:4321", "203.0.113.9, 104.16.1.1", "203.0.113.9"},
		{"traefik only", "10.0.1.5:4321", "203.0.113.9", "203.0.113.9"},
		{"spoofed value ahead of the real client", "10.0.1.5:4321", "1.2.3.4, 203.0.113.9", "203.0.113.9"},
		{"direct connection without a proxy", "198.51.100.7:4321", "", "198.51.100.7"},
		{"header from an untrusted peer is ignored", "198.51.100.7:4321", "1.2.3.4", "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := extract(req); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
