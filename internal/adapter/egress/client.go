// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Defaults applied by [New] to zero Config fields.
const (
	// DefaultMaxRedirects is the number of redirects followed (ADR-0029 §8).
	DefaultMaxRedirects = 10
	// DefaultMaxResponseBytes caps a response body (ADR-0029 §8): 64 MiB.
	DefaultMaxResponseBytes int64 = 64 << 20
	// DefaultMaxRequestBytes caps a request body: 64 MiB.
	DefaultMaxRequestBytes int64 = 64 << 20
	// DefaultConnectTimeout bounds one TCP connection attempt.
	DefaultConnectTimeout = 10 * time.Second
	// DefaultTLSHandshakeTimeout bounds the TLS handshake.
	DefaultTLSHandshakeTimeout = 10 * time.Second
	// DefaultResponseHeaderTimeout bounds the wait for response headers after
	// the request was sent.
	DefaultResponseHeaderTimeout = 30 * time.Second
	// DefaultTimeout bounds a whole exchange when the policy sets none.
	DefaultTimeout = 60 * time.Second

	maxResponseHeaderBytes = 1 << 20
)

// Resolver resolves host names. [net.Resolver] implements it; tests supply a
// stub.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Dialer opens TCP connections. [net.Dialer] implements it. A custom Dialer
// receives only addresses that already passed the policy.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Config is the instance-level configuration of a [Client]. Zero fields take
// the defaults above.
type Config struct {
	// Resolver resolves host names. Default: [net.DefaultResolver].
	Resolver Resolver
	// Dialer opens connections to checked addresses. Default: a [net.Dialer]
	// that also re-checks the address it connects to.
	Dialer Dialer

	// ProxyURL is the optional instance HTTP proxy (http, https, socks5 or
	// socks5h). Requests under a policy without AllowPrivateNetworks go
	// through it; requests that may reach private networks connect directly,
	// because a proxy cannot reach the internal hosts they were granted. The
	// target host is still resolved and checked locally before the proxy is
	// used, but the proxy resolves it again: the proxy must enforce its own
	// network restrictions. Credentials in the URL are never logged.
	ProxyURL string

	ConnectTimeout        time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	// Timeout is the default for Policy.Timeout.
	Timeout time.Duration

	// MaxResponseBytes is the default for Policy.MaxResponseBytes.
	MaxResponseBytes int64
	// MaxRequestBytes caps request bodies.
	MaxRequestBytes int64
	MaxRedirects    int

	// DenyHosts are instance-wide host patterns (same syntax as
	// Policy.Domains) that no policy can reach, on top of the built-in cloud
	// metadata names.
	DenyHosts []string
	// DenyNetworks are instance-wide address ranges that no policy can reach,
	// even with AllowPrivateNetworks.
	DenyNetworks []netip.Prefix

	// TLSConfig is cloned for every transport. Default: system roots, TLS 1.2
	// minimum.
	TLSConfig *tls.Config
	// UserAgent is sent when a request sets none.
	UserAgent string

	// Observer receives one event per request hop. Optional.
	Observer Observer
	// Logger receives one structured record per request hop with fixed fields
	// only (scheme, host, outcome, reason, status, duration). Optional.
	Logger *slog.Logger
}

// defaultDenyHosts are cloud metadata host names refused for every client.
var defaultDenyHosts = []string{
	"metadata", "metadata.google.internal", "metadata.goog",
	"instance-data", "instance-data.ec2.internal", "metadata.tencentyun.com",
}

// Client is the egress HTTP client. Build one with [New] and share it; it is
// safe for concurrent use.
type Client struct {
	guard    guard
	resolver Resolver
	dialer   Dialer
	proxy    *url.URL

	proxyAddr string

	connectTimeout, tlsTimeout, headerTimeout, timeout time.Duration
	maxResponse, maxRequest                            int64
	maxRedirects                                       int
	tlsConfig                                          *tls.Config
	userAgent                                          string
	observer                                           Observer
	logger                                             *slog.Logger

	mu              sync.Mutex // guards the lazily built transports
	strict, private *http.Transport
}

// New validates cfg and builds a Client.
func New(cfg Config) (*Client, error) {
	c := &Client{
		resolver:       cfg.Resolver,
		dialer:         cfg.Dialer,
		connectTimeout: orDuration(cfg.ConnectTimeout, DefaultConnectTimeout),
		tlsTimeout:     orDuration(cfg.TLSHandshakeTimeout, DefaultTLSHandshakeTimeout),
		headerTimeout:  orDuration(cfg.ResponseHeaderTimeout, DefaultResponseHeaderTimeout),
		timeout:        orDuration(cfg.Timeout, DefaultTimeout),
		maxResponse:    orInt64(cfg.MaxResponseBytes, DefaultMaxResponseBytes),
		maxRequest:     orInt64(cfg.MaxRequestBytes, DefaultMaxRequestBytes),
		maxRedirects:   cfg.MaxRedirects,
		userAgent:      cfg.UserAgent,
		observer:       cfg.Observer,
		logger:         cfg.Logger,
	}
	if cfg.ConnectTimeout < 0 || cfg.TLSHandshakeTimeout < 0 || cfg.ResponseHeaderTimeout < 0 ||
		cfg.Timeout < 0 || cfg.MaxResponseBytes < 0 || cfg.MaxRequestBytes < 0 || cfg.MaxRedirects < 0 {
		return nil, errors.New("egress config: timeouts and limits must not be negative")
	}
	if c.maxRedirects == 0 {
		c.maxRedirects = DefaultMaxRedirects
	}
	if c.resolver == nil {
		c.resolver = net.DefaultResolver
	}
	if cfg.TLSConfig != nil {
		c.tlsConfig = cfg.TLSConfig.Clone()
	} else {
		c.tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	for _, d := range append(append([]string(nil), defaultDenyHosts...), cfg.DenyHosts...) {
		if err := validatePattern(d); err != nil {
			return nil, fmt.Errorf("egress config: deny host %q: %w", d, err)
		}
		c.guard.denyHosts = append(c.guard.denyHosts, d)
	}
	for _, n := range cfg.DenyNetworks {
		if !n.IsValid() {
			return nil, errors.New("egress config: invalid deny network")
		}
		c.guard.denyNetworks = append(c.guard.denyNetworks, n.Masked())
	}

	if cfg.ProxyURL != "" {
		u, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			// url.Parse echoes the input, which may hold proxy credentials.
			return nil, errors.New("egress config: proxy URL is malformed")
		}
		switch u.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return nil, errors.New("egress config: proxy URL scheme must be http, https, socks5 or socks5h")
		}
		if u.Hostname() == "" {
			return nil, errors.New("egress config: proxy URL has no host")
		}
		c.proxy = u
		port := u.Port()
		if port == "" {
			port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
			if port == "" {
				port = "1080"
			}
		}
		c.proxyAddr = net.JoinHostPort(strings.ToLower(u.Hostname()), port)
	}
	return c, nil
}

func orDuration(v, def time.Duration) time.Duration {
	if v == 0 {
		return def
	}
	return v
}

func orInt64(v, def int64) int64 {
	if v == 0 {
		return def
	}
	return v
}

// Check reports whether policy would allow a request to rawURL, without
// connecting. It checks the URL, scheme, port, domain and literal addresses;
// resolved addresses are checked when connecting. Use it to validate a
// configured destination when it is saved.
func (c *Client) Check(policy Policy, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return denied(ReasonBadURL, "", "", "malformed URL")
	}
	_, err = c.guard.checkURL(&policy, u)
	return err
}

// HTTPClient returns an [*http.Client] that enforces policy on every request
// and redirect hop. Errors returned by its methods are [*url.Error] values
// that contain the full request URL, query included; callers that log errors
// should use [Client.Do] instead, which removes the URL.
func (c *Client) HTTPClient(policy Policy) *http.Client {
	timeout := policy.Timeout
	if timeout == 0 {
		timeout = c.timeout
	}
	return &http.Client{
		Transport: c.RoundTripper(policy),
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > c.maxRedirects {
				return denied(ReasonTooManyRedirects, strings.ToLower(req.URL.Scheme), req.URL.Hostname(), "")
			}
			if len(via) > 0 && !sameOrigin(via[0].URL, req.URL) {
				stripForeignHeaders(req.Header)
			}
			return nil
		},
	}
}

// RoundTripper returns an [http.RoundTripper] that enforces policy on every
// request it carries. Under [http.Client] it also sees every redirect hop.
func (c *Client) RoundTripper(policy Policy) http.RoundTripper {
	return &roundTripper{client: c, policy: policy}
}

// Do sends req under policy, following redirects. The returned error never
// contains the request URL's path or query.
func (c *Client) Do(ctx context.Context, policy Policy, req *http.Request) (*http.Response, error) {
	resp, err := c.HTTPClient(policy).Do(req.WithContext(ctx))
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		scheme, host := "", ""
		if req.URL != nil {
			scheme, host = strings.ToLower(req.URL.Scheme), req.URL.Host
		}
		return nil, fmt.Errorf("egress %s %s://%s: %w", req.Method, scheme, host, err)
	}
	return resp, nil
}

// CloseIdleConnections closes pooled connections.
func (c *Client) CloseIdleConnections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.strict != nil {
		c.strict.CloseIdleConnections()
	}
	if c.private != nil {
		c.private.CloseIdleConnections()
	}
}

// sameOrigin reports whether two URLs share scheme, host and port.
func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		portOf(a) == portOf(b)
}

func portOf(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	return strconv.Itoa(defaultPort(strings.ToLower(u.Scheme)))
}

// stripForeignHeaders keeps only headers that are safe to send to another
// origin after a redirect. The standard library already drops Authorization
// and Cookie across domains; this extends the rule to every credential-bearing
// custom header such as X-Api-Key.
func stripForeignHeaders(h http.Header) {
	for name := range h {
		switch http.CanonicalHeaderKey(name) {
		case "Accept", "Accept-Encoding", "Accept-Language", "Content-Type", "User-Agent":
		default:
			delete(h, name)
		}
	}
}

func (c *Client) transport(allowPrivate bool) *http.Transport {
	build := func() *http.Transport {
		t := &http.Transport{
			DialContext:            c.dialContext(allowPrivate),
			TLSClientConfig:        c.tlsConfig.Clone(),
			TLSHandshakeTimeout:    c.tlsTimeout,
			ResponseHeaderTimeout:  c.headerTimeout,
			ExpectContinueTimeout:  time.Second,
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    4,
			IdleConnTimeout:        90 * time.Second,
			MaxResponseHeaderBytes: maxResponseHeaderBytes,
			ForceAttemptHTTP2:      true,
		}
		if c.proxy != nil && !allowPrivate {
			t.Proxy = http.ProxyURL(c.proxy)
		}
		return t
	}
	// Two transports, never one: a pooled connection to a private address must
	// not be reused by a request whose policy forbids private networks.
	c.mu.Lock()
	defer c.mu.Unlock()
	if allowPrivate {
		if c.private == nil {
			c.private = build()
		}
		return c.private
	}
	if c.strict == nil {
		c.strict = build()
	}
	return c.strict
}

// dialContext resolves once, checks every answer, and connects to a checked
// address literal (resolve then pin): the name is never resolved a second
// time, so a changing DNS answer cannot redirect the connection.
func (c *Client) dialContext(allowPrivate bool) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		switch network {
		case "tcp", "tcp4", "tcp6":
		default:
			return nil, denied(ReasonUnsupportedNetworkType, "", "", "only TCP is supported")
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, denied(ReasonBadURL, "", "", "malformed address")
		}
		if c.proxy != nil && !allowPrivate && strings.EqualFold(address, c.proxyAddr) {
			// The proxy is operator configuration, not a request destination.
			return c.dialPinned(ctx, network, []string{address}, allowPrivate, true)
		}
		if ip, perr := netip.ParseAddr(host); perr == nil {
			if err := c.guard.checkAddr(allowPrivate, ip, "", host); err != nil {
				return nil, err
			}
			return c.dialPinned(ctx, network, []string{net.JoinHostPort(ip.String(), port)}, allowPrivate, false)
		}
		addrs, err := c.lookupChecked(ctx, host, allowPrivate)
		if err != nil {
			return nil, err
		}
		targets := make([]string, 0, len(addrs))
		for _, a := range addrs {
			targets = append(targets, net.JoinHostPort(a.String(), port))
		}
		return c.dialPinned(ctx, network, targets, allowPrivate, false)
	}
}

// lookupChecked resolves host and returns its addresses only if every one of
// them is allowed.
func (c *Client) lookupChecked(ctx context.Context, host string, allowPrivate bool) ([]netip.Addr, error) {
	addrs, err := c.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %s: no addresses", host)
	}
	for _, a := range addrs {
		if err := c.guard.checkAddr(allowPrivate, a, "", host); err != nil {
			return nil, err
		}
	}
	return addrs, nil
}

// dialPinned connects to the first reachable target. Targets are address
// literals that already passed the policy (or the operator's proxy address when
// direct is true). The default dialer re-checks the address of the socket it is
// about to connect, a second line of defense behind the resolution check.
func (c *Client) dialPinned(ctx context.Context, network string, targets []string, allowPrivate, direct bool) (net.Conn, error) {
	d := c.dialer
	if d == nil {
		nd := &net.Dialer{KeepAlive: 30 * time.Second}
		if !direct {
			nd.Control = func(_, address string, _ syscall.RawConn) error {
				ap, err := netip.ParseAddrPort(address)
				if err != nil {
					return denied(ReasonBadURL, "", "", "malformed address")
				}
				return c.guard.checkAddr(allowPrivate, ap.Addr(), "", "")
			}
		}
		d = nd
	}
	var errs []error
	for _, t := range targets {
		attempt, cancel := context.WithTimeout(ctx, c.connectTimeout)
		conn, err := d.DialContext(attempt, network, t)
		cancel()
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
		if ctx.Err() != nil {
			break
		}
	}
	return nil, errors.Join(errs...)
}

type roundTripper struct {
	client *Client
	policy Policy
}

// RoundTrip implements http.RoundTripper.
func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c := rt.client
	start := time.Now()
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	ev := Event{Method: method}
	if req.URL != nil {
		ev.Scheme = strings.ToLower(req.URL.Scheme)
	}
	closeBody := func() {
		if req.Body != nil {
			_ = req.Body.Close() // RoundTripper contract: the body is always closed; a close error changes nothing.
		}
	}

	host, err := c.guard.checkURL(&rt.policy, req.URL)
	if err != nil {
		closeBody()
		c.denyEvent(req.Context(), ev, start, err)
		return nil, err
	}
	ev.Host = host
	if req.Host != "" {
		// The Host header must name the host that was checked: a different
		// one would let a granted domain front for another virtual host.
		if !hostHeaderMatches(req.Host, req.URL, host) {
			derr := denied(ReasonBadURL, ev.Scheme, host, "the Host header differs from the URL host")
			closeBody()
			c.denyEvent(req.Context(), ev, start, derr)
			return nil, derr
		}
	}

	if req.ContentLength > c.maxRequest {
		closeBody()
		ev.Outcome, ev.Duration = OutcomeTooLarge, time.Since(start)
		c.emit(req.Context(), ev)
		return nil, ErrRequestTooLarge
	}
	proxied := c.proxy != nil && !rt.policy.AllowPrivateNetworks
	if proxied {
		// The proxy resolves the name itself, so the local answer cannot be
		// pinned; checking it at least refuses names that point inside.
		if _, literal := hostLiteral(host); !literal {
			if _, lerr := c.lookupChecked(req.Context(), host, false); lerr != nil {
				closeBody()
				c.denyEvent(req.Context(), ev, start, lerr)
				return nil, lerr
			}
		}
	}

	out := req.Clone(req.Context())
	out.Body = req.Body
	if out.Body != nil && out.Body != http.NoBody && out.ContentLength <= 0 {
		out.Body = &limitedReader{rc: req.Body, remaining: c.maxRequest, err: ErrRequestTooLarge}
	}
	if c.userAgent != "" && out.Header.Get("User-Agent") == "" {
		out.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.transport(rt.policy.AllowPrivateNetworks).RoundTrip(out)
	ev.Duration = time.Since(start)
	if err != nil {
		var de *DeniedError
		if errors.As(err, &de) {
			// Refused while connecting (resolved address or socket check).
			de.Scheme, de.Host = ev.Scheme, host
			c.denyEvent(req.Context(), ev, start, de)
			return nil, de
		}
		ev.Outcome = OutcomeFailed
		c.emit(req.Context(), ev)
		return nil, err
	}

	limit := rt.policy.MaxResponseBytes
	if limit == 0 {
		limit = c.maxResponse
	}
	if resp.ContentLength > limit {
		_ = resp.Body.Close() // the response is discarded.
		ev.Outcome = OutcomeTooLarge
		c.emit(req.Context(), ev)
		return nil, ErrResponseTooLarge
	}
	ctx := req.Context()
	resp.Body = &limitedReader{rc: resp.Body, remaining: limit, err: ErrResponseTooLarge, onExceed: func() {
		e := ev
		e.Outcome = OutcomeTooLarge
		c.emit(ctx, e)
	}}
	ev.Outcome, ev.Status = OutcomeCompleted, resp.StatusCode
	c.emit(req.Context(), ev)
	return resp, nil
}

func (c *Client) denyEvent(ctx context.Context, ev Event, start time.Time, err error) {
	ev.Outcome, ev.Duration = OutcomeDenied, time.Since(start)
	var de *DeniedError
	if errors.As(err, &de) {
		ev.Reason = de.Reason
		if ev.Host == "" {
			ev.Host = de.Host
		}
		if ev.Scheme == "" {
			ev.Scheme = de.Scheme
		}
	}
	c.emit(ctx, ev)
}

// hostHeaderMatches reports whether a Host header value names the checked
// host and, when it carries a port, the port of the URL. Anything that is not
// a plain host or host:port (path, query, fragment, userinfo, whitespace or
// control characters) is rejected.
func hostHeaderMatches(header string, u *url.URL, host string) bool {
	for _, r := range header {
		if r <= ' ' || r == 0x7f || strings.ContainsRune("/?#@\\", r) {
			return false
		}
	}
	name, port := header, ""
	switch {
	case strings.HasPrefix(header, "["):
		end := strings.Index(header, "]")
		if end < 0 {
			return false
		}
		name = header[1:end]
		if !strings.Contains(name, ":") {
			return false // brackets are for IPv6 literals only
		}
		if rest := header[end+1:]; rest != "" {
			if rest[0] != ':' {
				return false
			}
			port = rest[1:]
			if port == "" {
				return false
			}
		}
	case strings.Count(header, ":") == 1:
		name, port, _ = strings.Cut(header, ":")
		if port == "" {
			return false
		}
	case strings.Contains(header, ":"):
		return false // a bare IPv6 address needs brackets
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}
	if port != "" && port != portOf(u) {
		return false
	}
	norm, _, err := normalizeHost(name)
	return err == nil && norm == host
}

func hostLiteral(host string) (netip.Addr, bool) {
	a, err := netip.ParseAddr(host)
	return a, err == nil
}

// limitedReader reads at most remaining bytes, then fails with err instead of
// returning a silently truncated body.
type limitedReader struct {
	rc        io.ReadCloser
	remaining int64
	err       error
	exceeded  bool
	onExceed  func()
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.exceeded {
		return 0, l.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if int64(len(p)) > l.remaining+1 {
		p = p[:l.remaining+1]
	}
	n, err := l.rc.Read(p)
	if int64(n) > l.remaining {
		n = int(l.remaining)
		l.remaining = 0
		l.exceeded = true
		if l.onExceed != nil {
			l.onExceed()
		}
		return n, l.err
	}
	l.remaining -= int64(n)
	return n, err
}

func (l *limitedReader) Close() error { return l.rc.Close() }
