package api

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimitWindow struct {
	started time.Time
	count   int
}

type fixedWindowLimiter struct {
	mu          sync.Mutex
	maxRequests int
	window      time.Duration
	now         func() time.Time
	clients     map[string]rateLimitWindow
	lastCleanup time.Time
}

func newFixedWindowLimiter(maxRequests int, window time.Duration, now func() time.Time) *fixedWindowLimiter {
	return &fixedWindowLimiter{
		maxRequests: maxRequests,
		window:      window,
		now:         now,
		clients:     make(map[string]rateLimitWindow),
	}
}

func (l *fixedWindowLimiter) allow(client string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.window {
		for key, current := range l.clients {
			if now.Sub(current.started) >= l.window {
				delete(l.clients, key)
			}
		}
		l.lastCleanup = now
	}

	current, exists := l.clients[client]
	if !exists || now.Sub(current.started) >= l.window {
		l.clients[client] = rateLimitWindow{started: now, count: 1}
		return true, 0
	}
	if current.count >= l.maxRequests {
		return false, current.started.Add(l.window).Sub(now)
	}
	current.count++
	l.clients[client] = current
	return true, 0
}

func (h *Handler) rateLimitAnonymousUploads(next http.Handler) http.Handler {
	if h.limitUploads == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := requestClient(r)
		if h.limitAll != nil {
			if allowed, retryAfter := h.limitAll.allow("all"); !allowed {
				h.writeRateLimit(w, client, retryAfter)
				return
			}
		}
		allowed, retryAfter := h.limitUploads.allow(client)
		if allowed {
			next.ServeHTTP(w, r)
			return
		}
		h.writeRateLimit(w, client, retryAfter)
	})
}

func (h *Handler) writeRateLimit(w http.ResponseWriter, client string, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	h.logger.Warn("anonymous upload rate limited", "client", clientFingerprint(client))
	writeError(w, http.StatusTooManyRequests, "rate_limited", "too many anonymous uploads; try again later")
}

func requestClient(r *http.Request) string {
	// Google frontends append the verified client path to X-Forwarded-For.
	// Use the first syntactically valid address so the public web proxy keeps
	// distinct browser clients. A separate per-instance ceiling limits callers
	// that rotate or forge this prefix.
	for _, value := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(r.RemoteAddr)); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func clientFingerprint(client string) string {
	sum := sha256.Sum256([]byte(client))
	return hex.EncodeToString(sum[:6])
}
