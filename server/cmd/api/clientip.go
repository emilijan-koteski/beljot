package main

import (
	"net"

	"github.com/labstack/echo/v4"
)

var cloudflareEdgeRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
}

// clientIPExtractor reads the client IP from X-Forwarded-For, trusting a hop only if it is
// private or a Cloudflare edge (https://www.cloudflare.com/ips/), so clients cannot spoof it.
func clientIPExtractor() echo.IPExtractor {
	opts := []echo.TrustOption{
		echo.TrustLoopback(true),
		echo.TrustLinkLocal(true),
		echo.TrustPrivateNet(true),
	}
	for _, cidr := range cloudflareEdgeRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid Cloudflare range " + cidr + ": " + err.Error())
		}
		opts = append(opts, echo.TrustIPRange(ipNet))
	}
	return echo.ExtractIPFromXFFHeader(opts...)
}
