package middleware

import (
	"crypto/subtle"
	"net/http"
)

// PipelineAuth validates the pipeline service token from the Authorization
// header. It uses constant-time comparison to prevent timing attacks.
//
// The token must be presented as "Bearer <token>".
func PipelineAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := extractBearer(r)
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				writeUnauthorized(w, "invalid pipeline token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
