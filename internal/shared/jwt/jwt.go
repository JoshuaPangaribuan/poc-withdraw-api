package jwt

import (
	"context"
	"fmt"
	"time"
)

// Strategy defines which signing algorithm family to use.
type Strategy string

const (
	StrategyHMAC Strategy = "hmac"
	// Future strategies:
	// StrategyRSA   Strategy = "rsa"
	// StrategyECDSA Strategy = "ecdsa"
	// StrategyEdDSA Strategy = "eddsa"
)

// Options configures the token manager.
type Options struct {
	// Strategy selects the signing algorithm family.
	Strategy Strategy

	// ── HMAC options ──

	// Secret is the shared key for HMAC-based strategies.
	// Must be at least 32 bytes. Required when Strategy is StrategyHMAC.
	Secret []byte

	// ── Asymmetric options (future) ──

	// PrivateKeyPEM is the PEM-encoded private key (RSA/ECDSA/EdDSA).
	// Required for signing with asymmetric strategies.
	// PrivateKeyPEM []byte

	// PublicKeyPEM is the PEM-encoded public key (RSA/ECDSA/EdDSA).
	// Required for verification with asymmetric strategies.
	// If provided without PrivateKeyPEM, only verification is available.
	// PublicKeyPEM []byte

	// ── Common options ──

	// Algorithm specifies the exact signing algorithm within the strategy.
	// HMAC: "HS256" (default), "HS384", "HS512".
	// If empty, defaults to the strategy's recommended algorithm.
	Algorithm string

	// Issuer sets the default "iss" claim on generated tokens.
	Issuer string

	// Audience sets the default "aud" claim on generated tokens.
	Audience []string

	// TTL is the token time-to-live. Determines the "exp" claim.
	// Zero means tokens do not expire (not recommended for production).
	TTL time.Duration
}

// Claims represents the standard JWT registered claims (RFC 7519 §4.1).
// This type is library-agnostic; the underlying JWT library is an implementation detail.
type Claims struct {
	// Subject identifies the principal (e.g., user ID).
	Subject string

	// Issuer identifies who issued the token.
	// Defaults to Options.Issuer if empty during signing.
	Issuer string

	// Audience identifies the recipients the token is intended for.
	// Defaults to Options.Audience if nil during signing.
	Audience []string

	// ExpiresAt is the expiration time.
	// Defaults to time.Now() + Options.TTL during signing (if TTL > 0).
	ExpiresAt time.Time

	// IssuedAt is the time at which the token was issued.
	// Defaults to time.Now() during signing.
	IssuedAt time.Time

	// NotBefore is the time before which the token is not valid.
	// Zero means not set.
	NotBefore time.Time

	// ID is the unique token identifier (jti claim).
	// If empty, no jti is set.
	ID string
}

// Signer creates signed JWT tokens.
// Implementations must be safe for concurrent use.
type Signer interface {
	// Sign creates a signed JWT from the given claims.
	// Fields left zero in claims are filled with defaults from Options
	// (Issuer, Audience, TTL). IssuedAt defaults to time.Now().
	Sign(ctx context.Context, claims Claims) (string, error)
}

// Verifier validates and parses JWT tokens.
// Implementations must be safe for concurrent use.
type Verifier interface {
	// Verify parses and validates the token string.
	// Returns the claims on success, or an error if the token is
	// invalid, expired, or its signature does not match.
	Verify(ctx context.Context, tokenString string) (*Claims, error)
}

// TokenManager combines signing and verification capabilities.
// Implementations must be safe for concurrent use.
type TokenManager interface {
	Signer
	Verifier
}

// New creates a TokenManager based on the provided options.
// Returns an error if the strategy is unknown or configuration is invalid.
func New(opts Options) (TokenManager, error) {
	switch opts.Strategy {
	case StrategyHMAC:
		return NewHMAC(opts)
	default:
		return nil, fmt.Errorf("jwt: unknown strategy %q", opts.Strategy)
	}
}
