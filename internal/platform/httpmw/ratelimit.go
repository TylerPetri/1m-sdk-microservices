package httpmw

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// In-memory per-IP rate limiter.
// NOTE: For multi-instance deployments, back this with Redis (sliding window / token bucket).
type IPLimiter struct {
	rate    rate.Limit
	burst   int
	ttl     time.Duration
	mu      sync.Mutex
	clients map[string]*ipClient
}

type ipClient struct {
	lim  *rate.Limiter
	last time.Time
}

func NewIPLimiter(r rate.Limit, burst int, ttl time.Duration) *IPLimiter {
	return &IPLimiter{
		rate:    r,
		burst:   burst,
		ttl:     ttl,
		clients: make(map[string]*ipClient),
	}
}

func (l *IPLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	// opportunistic cleanup
	for k, c := range l.clients {
		if now.Sub(c.last) > l.ttl {
			delete(l.clients, k)
		}
	}

	c, ok := l.clients[ip]
	if !ok {
		c = &ipClient{lim: rate.NewLimiter(l.rate, l.burst), last: now}
		l.clients[ip] = c
		return c.lim
	}
	c.last = now
	return c.lim
}

func (l *IPLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" {
			ip = "unknown"
		}
		if !l.get(ip).Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// Prefer RFC 7239 Forwarded? We'll keep this minimal.
	// If you run behind a trusted proxy, terminate and set X-Forwarded-For there.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	// RemoteAddr may already be a host.
	if net.ParseIP(r.RemoteAddr) != nil {
		return r.RemoteAddr
	}
	return ""
}
