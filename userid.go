package hippocampus

import "context"

type userIDKeyType struct{}

// userIDKey is the context key under which the end-user account ID is stored.
var userIDKey userIDKeyType

// WithUserID returns a copy of ctx carrying the end-user account ID. Set this
// once at your request boundary (e.g. a gRPC auth interceptor); model adapters
// that support end-user attribution read it automatically and forward it to the
// provider (OpenAI "user", Anthropic "metadata.user_id"). Adapters that don't
// support attribution ignore it.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext returns the end-user account ID set via WithUserID, and
// whether it was present.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}
