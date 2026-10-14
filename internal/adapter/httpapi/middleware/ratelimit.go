// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// Class is the rate class of an operation (RATE-1).
type Class string

// The rate classes.
const (
	// ClassRead covers GET operations.
	ClassRead Class = "read"
	// ClassWrite covers mutations that do not start work.
	ClassWrite Class = "write"
	// ClassRun covers operations that start executions and similar work.
	ClassRun Class = "run"
	// ClassStream counts concurrent event streams and MCP sessions.
	ClassStream Class = "stream"
)

// Limits are the per-minute quotas of a [Limiter] (ADR-0053 section 6). They are
// configuration with instance floors and organization overrides; the
// defaults below are the ones the ADR names.
type Limits struct {
	// Read, Write and Run are requests per minute per principal.
	Read, Write, Run int
	// StreamConcurrent is the number of concurrent streams per principal.
	StreamConcurrent int
	// InteractiveFactor multiplies the quotas of interactive sessions, which have
	// their own profile.
	InteractiveFactor int
	// IP is requests per minute per client address before authentication.
	IP int
}

// DefaultLimits returns the defaults of ADR-0053 section 6: read 1200/min,
// write 300/min, run 600/min, 20 concurrent streams, interactive sessions at
// twice the quota. The per-address limit is not numbered by the ADR; 600/min is
// the provisional value.
func DefaultLimits() Limits {
	return Limits{Read: 1200, Write: 300, Run: 600, StreamConcurrent: 20, InteractiveFactor: 2, IP: 600}
}

const (
	window             = time.Minute
	defaultMaxEntries  = 100_000
	evictFractionDenom = 10
)

// Limiter enforces token buckets per key and class, in memory. It is
// approximate by design when several api processes run: each applies the
// configured rate divided by the number of live api processes (Divisor).
type Limiter struct {
	clk     clock.Clock
	limits  Limits
	divisor func() int
	maxKeys int
	mu      sync.Mutex
	buckets map[string]*bucket
	streams map[string]int
}

type bucket struct {
	tokens float64
	last   time.Time
}

// LimiterOption configures a [Limiter].
type LimiterOption func(*Limiter)

// WithDivisor sets how many live api processes share the configured rates; the
// function is called on every decision and values below 1 count as 1.
func WithDivisor(divisor func() int) LimiterOption {
	return func(l *Limiter) { l.divisor = divisor }
}

// WithMaxKeys bounds the number of tracked keys; the default is 100000.
func WithMaxKeys(n int) LimiterOption {
	return func(l *Limiter) { l.maxKeys = n }
}

// NewLimiter returns a limiter with the given quotas.
func NewLimiter(clk clock.Clock, limits Limits, opts ...LimiterOption) *Limiter {
	l := &Limiter{
		clk:     clk,
		limits:  limits,
		divisor: func() int { return 1 },
		maxKeys: defaultMaxEntries,
		buckets: map[string]*bucket{},
		streams: map[string]int{},
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Tracked returns the number of keys the limiter currently remembers; metrics
// and tests use it to see that the table stays bounded.
func (l *Limiter) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// Decision is the outcome of one rate limit check.
type Decision struct {
	// Allowed reports whether the request may proceed.
	Allowed bool
	// Quota is the number of requests per window.
	Quota int
	// Remaining is what is left of the quota after this request.
	Remaining int
	// Reset is the time until the quota is whole again.
	Reset time.Duration
	// RetryAfter is the time until one more request is allowed; zero when allowed.
	RetryAfter time.Duration
}

func (l *Limiter) quota(class Class, interactive bool) int {
	var q int
	switch class {
	case ClassRead:
		q = l.limits.Read
	case ClassWrite:
		q = l.limits.Write
	case ClassRun:
		q = l.limits.Run
	case ClassStream:
		q = l.limits.StreamConcurrent
	}
	if interactive && l.limits.InteractiveFactor > 1 {
		q *= l.limits.InteractiveFactor
	}
	return q
}

// Allow takes one token of the bucket of key for class.
func (l *Limiter) Allow(key string, class Class, interactive bool) Decision {
	return l.take(key+"|"+string(class), l.quota(class, interactive))
}

// AllowAddress takes one token of the per-address bucket used before authentication.
func (l *Limiter) AllowAddress(address string) Decision {
	return l.take("ip|"+address, l.limits.IP)
}

func (l *Limiter) take(key string, quota int) Decision {
	if quota < 1 {
		return Decision{Allowed: false, Quota: 0, RetryAfter: window}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clk.Now()
	effective := max(1, quota/max(1, l.divisor()))
	b, ok := l.buckets[key]
	if !ok {
		l.makeRoom(now)
		b = &bucket{tokens: float64(effective), last: now}
		l.buckets[key] = b
	}
	perSecond := float64(effective) / window.Seconds()
	b.tokens = math.Min(float64(effective), b.tokens+now.Sub(b.last).Seconds()*perSecond)
	b.last = now

	d := Decision{Quota: effective}
	if b.tokens >= 1 {
		b.tokens--
		d.Allowed = true
	} else {
		d.RetryAfter = time.Duration((1 - b.tokens) / perSecond * float64(time.Second))
	}
	d.Remaining = int(b.tokens)
	d.Reset = time.Duration((float64(effective) - b.tokens) / perSecond * float64(time.Second))
	return d
}

// makeRoom keeps the table bounded: when it is full, buckets that are whole
// again are dropped, and if that is not enough a tenth of the table is evicted
// (the evicted keys start over with a full bucket, a deliberate trade for
// bounded memory).
func (l *Limiter) makeRoom(now time.Time) {
	if len(l.buckets) < l.maxKeys {
		return
	}
	for k, b := range l.buckets {
		if now.Sub(b.last) >= window {
			delete(l.buckets, k)
		}
	}
	if len(l.buckets) < l.maxKeys {
		return
	}
	drop := max(1, len(l.buckets)/evictFractionDenom)
	for k := range l.buckets {
		delete(l.buckets, k)
		drop--
		if drop == 0 {
			break
		}
	}
}

// AcquireStream counts one more concurrent stream of key. It returns ok false
// when the principal is at its limit; otherwise the caller must call release
// when the stream ends.
func (l *Limiter) AcquireStream(key string, interactive bool) (release func(), quota int, ok bool) {
	quota = l.quota(ClassStream, interactive)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.streams[key] >= quota {
		return nil, quota, false
	}
	l.streams[key]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.streams[key] <= 1 {
				delete(l.streams, key)
				return
			}
			l.streams[key]--
		})
	}, quota, true
}

// LimitAddress is the first step of the pipeline: a per-address limit before
// authentication, which protects the token and sign-in endpoints. It uses the
// connection address; behind a reverse proxy every client shares the proxy's
// address until trusted-proxy handling is configured.
func LimitAddress(l *Limiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if d := l.AllowAddress(host); !d.Allowed {
				problem.Write(w, r, problem.RateLimited, problem.RetryAfter(d.RetryAfter))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LimitPrincipal limits the authenticated principal per rate class (pipeline
// step 4). Every answer carries RateLimit-Policy and RateLimit; a refused
// request gets 429 rate-limited with Retry-After. It must run after
// [Authenticate].
func LimitPrincipal(l *Limiter, class Class) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFrom(r.Context())
			if !ok {
				unauthenticated(w, r)
				return
			}
			key := principal.ID.String()
			interactive := principal.CredentialType == identity.CredentialSession
			if class == ClassStream {
				release, quota, allowed := l.AcquireStream(key, interactive)
				setRateHeaders(w, class, quota, quota, 0)
				if !allowed {
					problem.Write(w, r, problem.RateLimited, problem.RetryAfter(time.Second))
					return
				}
				defer release()
				next.ServeHTTP(w, r)
				return
			}
			d := l.Allow(key, class, interactive)
			setRateHeaders(w, class, d.Quota, d.Remaining, d.Reset)
			if !d.Allowed {
				problem.Write(w, r, problem.RateLimited, problem.RetryAfter(d.RetryAfter))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// setRateHeaders writes RateLimit-Policy and RateLimit as structured fields
// (IETF httpapi ratelimit headers): policy "read";q=1200;w=60, state
// "read";r=1199;t=3.
func setRateHeaders(w http.ResponseWriter, class Class, quota, remaining int, reset time.Duration) {
	policy := fmt.Sprintf("%q;q=%d;w=%d", string(class), quota, int(window.Seconds()))
	if class == ClassStream {
		policy = fmt.Sprintf("%q;q=%d", string(class), quota)
	}
	w.Header().Set("RateLimit-Policy", policy)
	seconds := int64(math.Ceil(reset.Seconds()))
	w.Header().Set("RateLimit", fmt.Sprintf("%q;r=%s;t=%s", string(class), strconv.Itoa(max(0, remaining)), strconv.FormatInt(max(0, seconds), 10)))
}
