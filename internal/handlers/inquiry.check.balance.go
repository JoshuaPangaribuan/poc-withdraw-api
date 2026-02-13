package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
)

type BalanceInquiryService interface {
	CheckBalance(ctx context.Context, userID string) (vo.BalanceInquiry, error)
}

type InquiryCheckBalanceHandler struct {
	service BalanceInquiryService
	logger  *slog.Logger
}

func NewInquiryCheckBalanceHandler(service BalanceInquiryService, logger *slog.Logger) *InquiryCheckBalanceHandler {
	return &InquiryCheckBalanceHandler{service: service, logger: logger}
}

func (h *InquiryCheckBalanceHandler) Register(router fiber.Router) {
	router.Get("/inquiries/balance", h.Handle)
}

func (h *InquiryCheckBalanceHandler) Handle(c fiber.Ctx) error {
	userIDValue := c.Locals("user_id")
	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authenticated user",
		})
	}

	balance, err := h.service.CheckBalance(c.Context(), userID)
	if err != nil {
		if errors.Is(err, vo.ErrWalletNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "wallet not found",
			})
		}

		h.logger.Error("failed to check balance", "user_id", userID, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	return c.Status(fiber.StatusOK).JSON(balance)
}
