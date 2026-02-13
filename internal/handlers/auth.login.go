package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
)

type AuthLoginService interface {
	Login(ctx context.Context, email, password string) (vo.AuthLogin, error)
}

type AuthLoginHandler struct {
	service AuthLoginService
	logger  *slog.Logger
}

type authLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewAuthLoginHandler(service AuthLoginService, logger *slog.Logger) *AuthLoginHandler {
	return &AuthLoginHandler{service: service, logger: logger}
}

func (h *AuthLoginHandler) Register(router fiber.Router) {
	router.Post("/auth/login", h.Handle)
}

func (h *AuthLoginHandler) Handle(c fiber.Ctx) error {
	var requestBody authLoginRequest
	if err := c.Bind().JSON(&requestBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if strings.TrimSpace(requestBody.Email) == "" || strings.TrimSpace(requestBody.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email and password are required",
		})
	}

	loginResult, err := h.service.Login(c.Context(), requestBody.Email, requestBody.Password)
	if err != nil {
		if errors.Is(err, vo.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid email or password",
			})
		}

		h.logger.Error("failed to login", "email", requestBody.Email, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal server error",
		})
	}

	return c.Status(fiber.StatusOK).JSON(loginResult)
}
