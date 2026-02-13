package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
	"github.com/joshuarp/withdraw-api/internal/middlewares"
)

type BalanceWithdrawService interface {
	WithdrawBalance(ctx context.Context, userID string, amountMinor int64, chainID string) (vo.WalletWithdrawal, error)
}

type InquiryWithdrawBalanceHandler struct {
	service BalanceWithdrawService
	logger  *slog.Logger
}

type withdrawalRequest struct {
	AmountMinor int64 `json:"amount_minor"`
}

func NewInquiryWithdrawBalanceHandler(service BalanceWithdrawService, logger *slog.Logger) *InquiryWithdrawBalanceHandler {
	return &InquiryWithdrawBalanceHandler{service: service, logger: logger}
}

func (h *InquiryWithdrawBalanceHandler) Register(router fiber.Router) {
	router.Post("/withdrawals", h.Handle)
}

func (h *InquiryWithdrawBalanceHandler) Handle(c fiber.Ctx) error {
	userIDValue := c.Locals("user_id")
	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authenticated user",
		})
	}

	var requestBody withdrawalRequest
	if err := c.Bind().JSON(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	chainID := middlewares.ChainIDFromContext(c)
	result, err := h.service.WithdrawBalance(c.Context(), userID, requestBody.AmountMinor, chainID)
	if err != nil {
		switch {
		case errors.Is(err, vo.ErrInvalidAmount):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount_minor must be greater than 0"})
		case errors.Is(err, vo.ErrWalletNotFound):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "wallet not found"})
		case errors.Is(err, vo.ErrInsufficientBalance):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "insufficient balance"})
		default:
			h.logger.Error("failed to withdraw balance", "user_id", userID, "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
		}
	}

	return c.Status(fiber.StatusOK).JSON(result)
}
