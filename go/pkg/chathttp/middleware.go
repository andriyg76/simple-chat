package chathttp

import "net/http"

const testProfileHeader = "X-Test-Profile"

// EmbeddedProfileMiddleware sets chat participant from X-Test-Profile (E2E / embedded demo).
func EmbeddedProfileMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get(testProfileHeader); id != "" {
			r = r.WithContext(WithParticipant(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}
