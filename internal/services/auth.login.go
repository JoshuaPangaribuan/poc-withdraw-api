package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/joshuarp/withdraw-api/internal/domain"
	"github.com/joshuarp/withdraw-api/internal/domain/vo"
	sharedhash "github.com/joshuarp/withdraw-api/internal/shared/hash"
	sharedjwt "github.com/joshuarp/withdraw-api/internal/shared/jwt"
)

type AuthLoginRepository interface {
	GetUserAuthByEmail(ctx context.Context, email string) (domain.UserAuth, error)
}

type AuthLoginService struct {
	repository   AuthLoginRepository
	hasher       sharedhash.Hasher
	tokenManager sharedjwt.TokenManager
}

func NewAuthLoginService(
	repository AuthLoginRepository,
	hasher sharedhash.Hasher,
	tokenManager sharedjwt.TokenManager,
) *AuthLoginService {
	return &AuthLoginService{
		repository:   repository,
		hasher:       hasher,
		tokenManager: tokenManager,
	}
}

func (s *AuthLoginService) Login(ctx context.Context, email, password string) (vo.AuthLogin, error) {
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" || strings.TrimSpace(password) == "" {
		return vo.AuthLogin{}, vo.ErrInvalidCredentials
	}

	user, err := s.repository.GetUserAuthByEmail(ctx, normalizedEmail)
	if err != nil {
		return vo.AuthLogin{}, err
	}

	if err := s.hasher.Compare(ctx, user.PasswordHash, password); err != nil {
		return vo.AuthLogin{}, vo.ErrInvalidCredentials
	}

	token, err := s.tokenManager.Sign(ctx, sharedjwt.Claims{Subject: user.ID})
	if err != nil {
		return vo.AuthLogin{}, fmt.Errorf("service: failed to issue token: %w", err)
	}

	return vo.AuthLogin{
		AccessToken: token,
		TokenType:   "Bearer",
	}, nil
}
