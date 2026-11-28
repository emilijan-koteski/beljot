package auth

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
)

func AuthMiddleware(jwtSecret string) echo.MiddlewareFunc {
	return AuthMiddlewareWithRevocations(jwtSecret, nil)
}

// AuthMiddlewareWithRevocations is AuthMiddleware that also rejects tokens of users in revoked,
// so an erased account's still-valid access token stops working at once.
func AuthMiddlewareWithRevocations(jwtSecret string, revoked *RevokedUsers) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return apperr.ErrUnauthorized
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				return apperr.ErrUnauthorized
			}

			tokenString := parts[1]
			if tokenString == "" {
				return apperr.ErrUnauthorized
			}

			claims, err := ValidateToken(tokenString, jwtSecret)
			if err != nil {
				return apperr.ErrUnauthorized
			}

			if !slices.Contains([]string(claims.Audience), "access") {
				return apperr.ErrUnauthorized
			}

			userID, err := strconv.ParseUint(claims.Subject, 10, 64)
			if err != nil {
				return apperr.ErrUnauthorized
			}
			if revoked.IsRevoked(uint(userID)) {
				return apperr.ErrUnauthorized
			}

			c.Set("userID", uint(userID))
			return next(c)
		}
	}
}

func GetUserID(c echo.Context) (uint, error) {
	val := c.Get("userID")
	if val == nil {
		return 0, fmt.Errorf("userID not found in context")
	}

	userID, ok := val.(uint)
	if !ok {
		return 0, fmt.Errorf("userID has unexpected type")
	}

	return userID, nil
}
