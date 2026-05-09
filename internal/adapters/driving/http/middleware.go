package http

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driving"
)

// Context keys
type contextKey string

const authContextKey contextKey = "auth_context"

// AuthMiddleware handles authentication and authorization
type AuthMiddleware struct {
	authService driving.AuthService
}

// NewAuthMiddleware creates a new AuthMiddleware
func NewAuthMiddleware(authService driving.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// Authenticate validates the request token and adds auth context
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			WriteError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}

		authCtx, err := m.authService.ValidateToken(r.Context(), token)
		if err != nil {
			switch err {
			case domain.ErrTokenExpired:
				WriteError(w, http.StatusUnauthorized, "token expired")
			case domain.ErrSessionNotFound:
				WriteError(w, http.StatusUnauthorized, "session not found")
			default:
				WriteError(w, http.StatusUnauthorized, "invalid token")
			}
			return
		}

		// Add auth context to request context
		ctx := context.WithValue(r.Context(), authContextKey, authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin ensures the authenticated user is an admin
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx := GetAuthContext(r.Context())
		if authCtx == nil {
			WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if !authCtx.IsAdmin() {
			WriteError(w, http.StatusForbidden, "admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireRole ensures the authenticated user has one of the specified roles
func (m *AuthMiddleware) RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx := GetAuthContext(r.Context())
			if authCtx == nil {
				WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			for _, role := range roles {
				if authCtx.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			WriteError(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

// GetAuthContext retrieves the auth context from request context
func GetAuthContext(ctx context.Context) *domain.AuthContext {
	if ctx == nil {
		return nil
	}
	authCtx, ok := ctx.Value(authContextKey).(*domain.AuthContext)
	if !ok {
		return nil
	}
	return authCtx
}

// WithAuthContext returns a copy of ctx with the supplied AuthContext
// attached so downstream consumers (services, ports, adapters) can retrieve
// it via GetAuthContext.
//
// AuthMiddleware uses this internally for HTTP-bearer-token requests; other
// driving adapters that establish caller identity through a different
// mechanism (for example an MCP server validating an OAuth2 access token)
// can call WithAuthContext directly to inject an equivalent AuthContext so
// downstream identity-resolving adapters work uniformly across entry points.
func WithAuthContext(ctx context.Context, authCtx *domain.AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, authCtx)
}

// extractBearerToken extracts the Bearer token from Authorization header
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

// Logging middleware

// LoggingMiddleware logs HTTP requests
type LoggingMiddleware struct{}

// NewLoggingMiddleware creates a new LoggingMiddleware
func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

// Handler wraps an http.Handler with request logging
func (m *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Recovery middleware

// RecoveryMiddleware recovers from panics
type RecoveryMiddleware struct{}

// NewRecoveryMiddleware creates a new RecoveryMiddleware
func NewRecoveryMiddleware() *RecoveryMiddleware {
	return &RecoveryMiddleware{}
}

// Handler wraps an http.Handler with panic recovery
func (m *RecoveryMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS middleware

// CORSMiddleware handles CORS
type CORSMiddleware struct {
	allowedOrigins []string
}

// NewCORSMiddleware creates a new CORSMiddleware
func NewCORSMiddleware(allowedOrigins []string) *CORSMiddleware {
	return &CORSMiddleware{
		allowedOrigins: allowedOrigins,
	}
}

// Handler wraps an http.Handler with CORS headers
func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range m.allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Sercha-Audit-Skip")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
