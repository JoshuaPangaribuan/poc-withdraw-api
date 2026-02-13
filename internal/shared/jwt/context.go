package jwt

import "context"

// contextKey is an unexported type to prevent collisions with keys
// defined in other packages.
type contextKey struct{}

// SetClaims returns a new context with the given claims attached.
// Intended for use in authentication middleware after a successful Verify.
func SetClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

// GetClaims extracts the claims from the context.
// Returns nil and false if no claims are present.
func GetClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(contextKey{}).(*Claims)
	return claims, ok
}
