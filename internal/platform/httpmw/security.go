package httpmw

import "net/http"

// SecurityHeaders sets a small set of safe default security headers for API responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We serve JSON APIs; prevent MIME sniffing.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Clickjacking defense for any accidental HTML responses.
		w.Header().Set("X-Frame-Options", "DENY")
		// Reduce referrer leakage.
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
