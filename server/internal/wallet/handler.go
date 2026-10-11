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

// getUserID reads the authenticated user id the auth middleware stored on the echo context.
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

// ProcessDailyLogin handles POST /api/v1/wallet/daily-login, granting the bonus at most once per UTC day.
// It is idempotent and safe to call on every app bootstrap.
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
