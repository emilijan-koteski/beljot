package wallet

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/emilijan/beljot/server/internal/apperr"
)

type WalletHandler struct {
	service *Service
}

func NewWalletHandler(service *Service) *WalletHandler {
	return &WalletHandler{service: service}
}

// getUserID reads the authenticated user id the auth middleware stored on the
// echo context. Mirrors user.getUserID (unexported there, so duplicated here).
func getUserID(c echo.Context) (uint, error) {
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

// ProcessDailyLogin handles POST /api/v1/wallet/daily-login. It is the single
// grant point for the daily bonus: idempotent and safe to call on every app
// bootstrap (and to retry), it grants at most once per UTC day and returns the
// outcome in the { "data": ... } envelope.
func (h *WalletHandler) ProcessDailyLogin(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return apperr.ErrUnauthorized
	}

	result, err := h.service.ProcessDailyLogin(userID)
	if err != nil {
		return fmt.Errorf("processing daily login: %w", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": result,
	})
}
