package main

import (
	"time"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/ratelimit"
)

const (
	credentialAttemptsPerMinute = 10
	passwordResetAttempts       = 5
	passwordResetWindow         = 15 * time.Minute
	refreshesPerMinute          = 60
	accountDeleteAttempts       = 5
	accountDeleteWindow         = 15 * time.Minute
)

// authRateLimits holds one middleware per auth endpoint family, each with its own budget.
type authRateLimits struct {
	login          echo.MiddlewareFunc
	register       echo.MiddlewareFunc
	sso            echo.MiddlewareFunc
	forgotPassword echo.MiddlewareFunc
	resetPassword  echo.MiddlewareFunc
	refresh        echo.MiddlewareFunc
	deleteAccount  echo.MiddlewareFunc
}

// newAuthRateLimits keys the public endpoints on the real client IP, and account
// deletion on the signed-in user, since its route sits behind the auth middleware.
func newAuthRateLimits() authRateLimits {
	perIP := func(limit int, window time.Duration) echo.MiddlewareFunc {
		return ratelimit.Middleware(ratelimit.New(limit, window), ratelimit.ByClientIP)
	}
	return authRateLimits{
		login:          perIP(credentialAttemptsPerMinute, time.Minute),
		register:       perIP(credentialAttemptsPerMinute, time.Minute),
		sso:            perIP(credentialAttemptsPerMinute, time.Minute),
		forgotPassword: perIP(passwordResetAttempts, passwordResetWindow),
		resetPassword:  perIP(passwordResetAttempts, passwordResetWindow),
		refresh:        perIP(refreshesPerMinute, time.Minute),
		deleteAccount:  ratelimit.Middleware(ratelimit.New(accountDeleteAttempts, accountDeleteWindow), ratelimit.ByUserOrIP),
	}
}
