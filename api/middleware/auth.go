package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/donaldgifford/bark/pkg/types"
)

// callerKey is the context key for the authenticated caller identity.
const callerKey contextKey = "caller"

// Caller holds the identity of the authenticated API caller extracted from
// the validated JWT.
type Caller struct {
	// Subject is the "sub" claim — the unique identifier of the authenticated user or service.
	Subject string
	// Email is the "email" claim if present in the token.
	Email string
}

// AuthConfig holds configuration for the JWT validation middleware.
type AuthConfig struct {
	// JWKSURL is the URL of the OIDC provider's JWKS endpoint.
	// Example: https://your-org.okta.com/oauth2/default/v1/keys
	JWKSURL string
	// Audience is the expected "aud" claim in the JWT.
	Audience string
}

// JWTAuth validates Bearer tokens against the configured OIDC JWKS endpoint.
// Requests without a valid token receive 401. The authenticated caller is
// available via GetCaller(r.Context()).
func JWTAuth(cfg AuthConfig, logger *slog.Logger) (func(http.Handler) http.Handler, error) {
	jwks, err := keyfunc.NewDefaultCtx(context.Background(), []string{cfg.JWKSURL})
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeUnauthorized(w, "missing authorization header")
				return
			}

			claims := jwt.MapClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, jwks.Keyfunc,
				jwt.WithAudience(cfg.Audience),
				jwt.WithExpirationRequired(),
			)
			if err != nil || !token.Valid {
				logger.Warn("invalid JWT", "error", err, "request_id", GetRequestID(r.Context()))
				writeUnauthorized(w, "invalid or expired token")
				return
			}

			sub, err := claims.GetSubject()
			if err != nil {
				sub = ""
			}

			email, ok := claims["email"].(string)
			if !ok {
				email = ""
			}

			ctx := context.WithValue(r.Context(), callerKey, Caller{
				Subject: sub,
				Email:   email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// GetCaller extracts the authenticated Caller from the context.
// Returns a zero-value Caller if no identity is present.
func GetCaller(ctx context.Context) Caller {
	caller, ok := ctx.Value(callerKey).(Caller)
	if !ok {
		return Caller{}
	}
	return caller
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	//nolint:errcheck,errchkjson // response write error after headers are sent; not actionable
	_ = json.NewEncoder(w).Encode(types.ErrorResponse{Error: msg})
}
