package httpapi

import (
	"context"

	"github.com/VaudKK/shield/backend/internal/domain"
)

type contextKey string

const userContextKey contextKey = "shield_user"

func withUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext returns the authenticated user for the request, or nil if
// the request was not processed by RequireAuth.
func UserFromContext(ctx context.Context) *domain.User {
	user, _ := ctx.Value(userContextKey).(*domain.User)
	return user
}
