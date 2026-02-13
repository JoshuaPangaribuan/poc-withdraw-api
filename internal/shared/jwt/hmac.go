package jwt

import (
	"context"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

var _ TokenManager = (*hmacManager)(nil)

type hmacManager struct {
	secret   []byte
	method   jwtlib.SigningMethod
	issuer   string
	audience []string
	ttl      time.Duration
}

// NewHMAC creates an HMAC-based TokenManager.
// Secret must be at least 32 bytes.
// Algorithm defaults to "HS256" if empty. Supported: "HS256", "HS384", "HS512".
func NewHMAC(opts Options) (TokenManager, error) {
	if len(opts.Secret) == 0 {
		return nil, fmt.Errorf("jwt: HMAC secret must not be empty")
	}
	if len(opts.Secret) < 32 {
		return nil, fmt.Errorf("jwt: HMAC secret must be at least 32 bytes, got %d", len(opts.Secret))
	}

	method, err := resolveHMACMethod(opts.Algorithm)
	if err != nil {
		return nil, err
	}

	return &hmacManager{
		secret:   opts.Secret,
		method:   method,
		issuer:   opts.Issuer,
		audience: opts.Audience,
		ttl:      opts.TTL,
	}, nil
}

func resolveHMACMethod(alg string) (jwtlib.SigningMethod, error) {
	switch alg {
	case "", "HS256":
		return jwtlib.SigningMethodHS256, nil
	case "HS384":
		return jwtlib.SigningMethodHS384, nil
	case "HS512":
		return jwtlib.SigningMethodHS512, nil
	default:
		return nil, fmt.Errorf("jwt: unsupported HMAC algorithm %q", alg)
	}
}

func (m *hmacManager) Sign(_ context.Context, claims Claims) (string, error) {
	now := time.Now()

	registered := jwtlib.RegisteredClaims{
		Subject: claims.Subject,
		ID:      claims.ID,
	}

	// Apply defaults from Options, allow per-call overrides.
	if claims.Issuer != "" {
		registered.Issuer = claims.Issuer
	} else {
		registered.Issuer = m.issuer
	}

	if claims.Audience != nil {
		registered.Audience = jwtlib.ClaimStrings(claims.Audience)
	} else if m.audience != nil {
		registered.Audience = jwtlib.ClaimStrings(m.audience)
	}

	if !claims.IssuedAt.IsZero() {
		registered.IssuedAt = jwtlib.NewNumericDate(claims.IssuedAt)
	} else {
		registered.IssuedAt = jwtlib.NewNumericDate(now)
	}

	if !claims.ExpiresAt.IsZero() {
		registered.ExpiresAt = jwtlib.NewNumericDate(claims.ExpiresAt)
	} else if m.ttl > 0 {
		registered.ExpiresAt = jwtlib.NewNumericDate(now.Add(m.ttl))
	}

	if !claims.NotBefore.IsZero() {
		registered.NotBefore = jwtlib.NewNumericDate(claims.NotBefore)
	}

	token := jwtlib.NewWithClaims(m.method, registered)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("jwt: failed to sign token: %w", err)
	}
	return signed, nil
}

func (m *hmacManager) Verify(_ context.Context, tokenString string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(
		tokenString,
		&jwtlib.RegisteredClaims{},
		func(token *jwtlib.Token) (any, error) {
			// Ensure the signing method matches what we expect.
			if token.Method.Alg() != m.method.Alg() {
				return nil, fmt.Errorf("jwt: unexpected signing method %q", token.Method.Alg())
			}
			return m.secret, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("jwt: token validation failed: %w", err)
	}

	registered, ok := token.Claims.(*jwtlib.RegisteredClaims)
	if !ok {
		return nil, fmt.Errorf("jwt: unexpected claims type")
	}

	return registeredToClaims(registered), nil
}

func registeredToClaims(r *jwtlib.RegisteredClaims) *Claims {
	c := &Claims{
		Subject:  r.Subject,
		Issuer:   r.Issuer,
		Audience: []string(r.Audience),
		ID:       r.ID,
	}
	if r.ExpiresAt != nil {
		c.ExpiresAt = r.ExpiresAt.Time
	}
	if r.IssuedAt != nil {
		c.IssuedAt = r.IssuedAt.Time
	}
	if r.NotBefore != nil {
		c.NotBefore = r.NotBefore.Time
	}
	return c
}
