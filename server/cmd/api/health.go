package main

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type pinger interface {
	PingContext(ctx context.Context) error
}

// healthHandler answers 200 while the database is reachable and 503 otherwise,
// so Swarm's health check and the uptime monitor see a dead database as a dead
// app instead of a green process that fails every request.
func healthHandler(db pinger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}
