package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// contextKey is a private type for context keys in this package.
// Using a custom type prevents collisions with other packages.
type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID is a middleware that assigns a unique ID to every incoming request.
// The ID is injected into the request context and returned in the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Honor existing request ID if provided by upstream proxy
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Inject into context so handlers and orchestrator can read it
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)

		// Return it in the response header for client-side tracing
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext extracts the request ID from the context.
// Returns an empty string if not present.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// generateRequestID generates a time-based unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}