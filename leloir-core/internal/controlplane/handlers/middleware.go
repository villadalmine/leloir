package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// contextKey is a typed key for context values
type contextKey string

const (
	ctxKeyUser     contextKey = "user"
	ctxKeyTenant   contextKey = "tenant"
	ctxKeyRequest  contextKey = "request_id"
)

// loggingMiddleware adds a request ID and logs request timing.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = newRequestID()
		}
		ctx := context.WithValue(r.Context(), ctxKeyRequest, reqID)
		w.Header().Set("X-Request-ID", reqID)

		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(lrw, r.WithContext(ctx))

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lrw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqID,
		)
	})
}

// authMiddleware extracts the authenticated user from the request.
// M1: stub. M2: implement OIDC token verification.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// M2: verify OIDC bearer token, look up user
		user := "anonymous"
		if auth := r.Header.Get("Authorization"); auth != "" {
			// M2 real impl goes here
			user = "oidc-user-placeholder"
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// tenantScopeMiddleware determines the tenant for this request.
// M2: map user group memberships to tenants via OIDC claims.
func tenantScopeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// M2: look up tenant based on user groups
		// For now, accept ?tenant= query param (insecure, M2 will replace)
		tenant := r.URL.Query().Get("tenant")
		if tenant == "" {
			tenant = "default"
		}
		ctx := context.WithValue(r.Context(), ctxKeyTenant, tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ─── Helpers ────────────────────────────────────────────────────────────────

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.status = code
	l.ResponseWriter.WriteHeader(code)
}

// newRequestID generates a short unique ID for request correlation.
func newRequestID() string {
	// Use the same format as investigation IDs: "req-<short-hash>-<timestamp>"
	// Simplified for skeleton; use uuid in real impl
	return "req-" + time.Now().Format("20060102150405.000000")
}

// TenantFromContext returns the tenant set by tenantScopeMiddleware.
func TenantFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTenant).(string); ok {
		return v
	}
	return ""
}

// UserFromContext returns the authenticated user.
func UserFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUser).(string); ok {
		return v
	}
	return "anonymous"
}
