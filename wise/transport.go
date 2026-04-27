package wise

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iamkanishka/wise/wise/internal/ratelimit"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
	defaultUserAgent  = "wise-go/1.0.0 (+https://github.com/iamkanishka/wise)"

	headerAuthorization  = "Authorization"
	headerContentType    = "Content-Type"
	headerUserAgent      = "User-Agent"
	headerRequestID      = "X-Request-Id"
	headerIdempotencyKey = "X-Idempotency-Key"

	contentTypeJSON = "application/json"

	// MaxBodyBytes is the maximum response body size (10 MiB) to guard against unexpectedly large payloads.
	maxBodyBytes = 10 << 20 // 10 MiB
)

// -----------------------------------------------------------------------------
// Retry transport
// -----------------------------------------------------------------------------

// retryTransport wraps an inner [http.RoundTripper] with:
//   - Token-bucket rate limiting.
//   - Exponential backoff with full jitter on 429 and 5xx.
type retryTransport struct {
	inner      http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
	limiter    ratelimit.Limiter
}

func newRetryTransport(cfg *config) *retryTransport {
	rl := cfg.rateLimiter
	if rl == nil {
		rl = ratelimit.DefaultBucket()
	}

	return &retryTransport{
		inner:      http.DefaultTransport,
		maxRetries: cfg.maxRetries,
		baseDelay:  cfg.retryDelay,
		limiter:    rl,
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("wise: rate limiter: %w", err)
	}

	var (
		resp    *http.Response
		lastErr error
	)

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			delay := t.backoff(attempt)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("wise: request canceled: %w", ctx.Err())
			case <-time.After(delay):
			}

			if err := t.limiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("wise: rate limiter: %w", err)
			}
		}

		cloned, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}

		resp, lastErr = t.inner.RoundTrip(cloned)
		if lastErr != nil {
			if isRetryableNetErr(lastErr) {
				continue
			}

			return nil, fmt.Errorf("wise: http transport: %w", lastErr)
		}

		if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// On the last attempt, return the response for the caller to parse.
		if attempt == t.maxRetries {
			return resp, nil
		}

		// Drain body to allow connection reuse before retrying.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	if lastErr != nil {
		return nil, fmt.Errorf("wise: retries exhausted: %w", lastErr)
	}

	return resp, nil
}

// backoff returns a jittered duration for the given attempt number (1-based).
func (t *retryTransport) backoff(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	maxDelay := time.Duration(exp) * t.baseDelay

	const maxBackoff = 30 * time.Second
	if maxDelay > maxBackoff {
		maxDelay = maxBackoff
	}

	// Full jitter: uniform random in [0, maxDelay).
	return time.Duration(cryptoRandInt63n(int64(maxDelay)))
}

// cryptoRandInt63n returns a cryptographically random non-negative int64 in [0, n).
// It is used for jitter in exponential backoff to satisfy the gosec G404 rule.
func cryptoRandInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}

	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is not recoverable; use a safe fallback of n/2.
		return n / 2 //nolint:mnd // safe static fallback
	}

	return int64(binary.BigEndian.Uint64(b[:]) >> 1 % uint64(n)) //nolint:gosec // mod reduces range, not a security issue
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	for _, kw := range []string{"connection reset", "EOF", "broken pipe", "no such host"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}

	return false
}

// cloneRequest creates a shallow clone of req with a re-readable body.
func cloneRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())

	if req.Body == nil {
		return clone, nil
	}

	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()

	if err != nil {
		return nil, fmt.Errorf("wise: clone request body: %w", err)
	}

	req.Body = io.NopCloser(strings.NewReader(string(data)))
	clone.Body = io.NopCloser(strings.NewReader(string(data)))

	return clone, nil
}

// -----------------------------------------------------------------------------
// Hook transport (middleware)
// -----------------------------------------------------------------------------

// hookTransport wraps an inner RoundTripper with request/response hooks.
type hookTransport struct {
	inner         http.RoundTripper
	requestHooks  []RequestHook
	responseHooks []ResponseHook
}

func (t *hookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	start := time.Now()

	for _, h := range t.requestHooks {
		if err := h(ctx, req); err != nil {
			return nil, fmt.Errorf("wise: request hook: %w", err)
		}
	}

	resp, err := t.inner.RoundTrip(req)

	latency := time.Since(start)
	code := 0

	if resp != nil {
		code = resp.StatusCode
	}

	for _, h := range t.responseHooks {
		h(ctx, req, code, latency, err)
	}

	return resp, err //nolint:wrapcheck // pass-through
}

// applyHooks wraps the base transport with hook middleware if any hooks are registered.
func applyHooks(cfg *config, base http.RoundTripper) http.RoundTripper {
	if len(cfg.requestHooks) == 0 && len(cfg.responseHooks) == 0 {
		return base
	}

	return &hookTransport{
		inner:         base,
		requestHooks:  cfg.requestHooks,
		responseHooks: cfg.responseHooks,
	}
}

// -----------------------------------------------------------------------------
// Circuit breaker
// -----------------------------------------------------------------------------

// CircuitState describes the current state of the circuit breaker.
type CircuitState int

const (
	// CircuitClosed is the healthy state — requests pass through normally.
	CircuitClosed CircuitState = iota

	// CircuitOpen rejects all requests immediately to protect against cascading failures.
	CircuitOpen

	// CircuitHalfOpen allows one probe request to test whether the service has recovered.
	CircuitHalfOpen
)

// String returns a human-readable state name.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig configures a [CircuitBreaker].
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures that open the breaker.
	// Default: 5.
	FailureThreshold int

	// SuccessThreshold is the consecutive successes in HALF_OPEN that close the breaker.
	// Default: 2.
	SuccessThreshold int

	// Timeout is how long the breaker stays OPEN before transitioning to HALF_OPEN.
	// Default: 30s.
	Timeout time.Duration

	// IsFailure optionally overrides what counts as a failure.
	// By default, all transport errors and HTTP 5xx responses count.
	IsFailure func(resp *http.Response, err error) bool
}

func (c *CircuitBreakerConfig) withDefaults() CircuitBreakerConfig {
	cfg := *c

	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5 //nolint:mnd // documented default
	}

	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2 //nolint:mnd // documented default
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.IsFailure == nil {
		cfg.IsFailure = defaultIsFailure
	}

	return cfg
}

func defaultIsFailure(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}

	return resp != nil && resp.StatusCode >= http.StatusInternalServerError
}

// CircuitBreaker implements the circuit breaker pattern as an [http.RoundTripper].
//
//	cb := wise.NewCircuitBreaker(wise.CircuitBreakerConfig{FailureThreshold: 5})
//	client, _ := wise.New(
//	    wise.WithPersonalToken("tok"),
//	    wise.WithCircuitBreaker(cb),
//	)
type CircuitBreaker struct {
	cfg CircuitBreakerConfig

	mu               sync.Mutex
	state            CircuitState
	consecutiveFails int
	consecutiveOK    int
	openedAt         time.Time
}

// NewCircuitBreaker creates a CircuitBreaker with the provided configuration.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		cfg:              cfg.withDefaults(),
		mu:               sync.Mutex{},
		state:            CircuitClosed,
		consecutiveFails: 0,
		consecutiveOK:    0,
		openedAt:         time.Time{},
	}
}

// State returns the current circuit state. Safe for concurrent use.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.resolveState()
}

// Reset manually resets the circuit breaker to CLOSED state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.consecutiveFails = 0
	cb.consecutiveOK = 0
}

// resolveState transitions OPEN → HALF_OPEN when the timeout has elapsed.
// Must be called with cb.mu held.
func (cb *CircuitBreaker) resolveState() CircuitState {
	if cb.state == CircuitOpen && time.Since(cb.openedAt) >= cb.cfg.Timeout {
		cb.state = CircuitHalfOpen
		cb.consecutiveOK = 0
	}

	return cb.state
}

// Wrap returns an [http.RoundTripper] that applies the circuit breaker logic.
func (cb *CircuitBreaker) Wrap(inner http.RoundTripper) http.RoundTripper {
	return &cbTransport{cb: cb, inner: inner}
}

type cbTransport struct {
	cb    *CircuitBreaker
	inner http.RoundTripper
}

func (t *cbTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.cb.mu.Lock()
	state := t.cb.resolveState()

	if state == CircuitOpen {
		remaining := t.cb.cfg.Timeout - time.Since(t.cb.openedAt)
		t.cb.mu.Unlock()

		return nil, fmt.Errorf("%w (retry after %s)", ErrCircuitOpen, remaining.Round(time.Second))
	}

	t.cb.mu.Unlock()

	resp, err := t.inner.RoundTrip(req) //nolint:wrapcheck // pass-through

	t.cb.mu.Lock()
	defer t.cb.mu.Unlock()

	if t.cb.cfg.IsFailure(resp, err) {
		t.cb.onFailure()
	} else {
		t.cb.onSuccess()
	}

	if err != nil {
		return resp, fmt.Errorf("wise: circuit breaker transport: %w", err)
	}

	return resp, nil
}

// onFailure records a failure. Must be called with cb.mu held.
func (cb *CircuitBreaker) onFailure() {
	cb.consecutiveOK = 0
	cb.consecutiveFails++

	if cb.state == CircuitHalfOpen || cb.consecutiveFails >= cb.cfg.FailureThreshold {
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
	}
}

// onSuccess records a success. Must be called with cb.mu held.
func (cb *CircuitBreaker) onSuccess() {
	cb.consecutiveFails = 0

	if cb.state == CircuitHalfOpen {
		cb.consecutiveOK++

		if cb.consecutiveOK >= cb.cfg.SuccessThreshold {
			cb.state = CircuitClosed
		}
	}
}

// -----------------------------------------------------------------------------
// Transport config (TCP/TLS tuning)
// -----------------------------------------------------------------------------

// TransportConfig holds low-level TCP/TLS connection pool parameters.
type TransportConfig struct {
	// MaxIdleConns is the maximum total idle (keep-alive) connections. Default: 100.
	MaxIdleConns int

	// MaxIdleConnsPerHost is the maximum idle connections per host. Default: 20.
	MaxIdleConnsPerHost int

	// MaxConnsPerHost caps total connections per host (0 = unlimited).
	MaxConnsPerHost int

	// IdleConnTimeout is how long an idle connection is kept before closing. Default: 90s.
	IdleConnTimeout time.Duration

	// TLSHandshakeTimeout is the maximum time allowed for a TLS handshake. Default: 10s.
	TLSHandshakeTimeout time.Duration

	// ExpectContinueTimeout is the wait time for a 100-Continue response. Default: 1s.
	ExpectContinueTimeout time.Duration

	// DialTimeout is the TCP dial deadline. Default: 10s.
	DialTimeout time.Duration

	// KeepAlive is the TCP keep-alive period. Default: 30s.
	KeepAlive time.Duration

	// DisableHTTP2 forces HTTP/1.1 and disables HTTP/2. Default: false.
	DisableHTTP2 bool

	// TLSConfig allows customizing TLS parameters (e.g. certificate pinning).
	TLSConfig *tls.Config
}

// DefaultTransportConfig returns a TransportConfig tuned for production use
// against the Wise API: connection pooling, TCP keep-alives, HTTP/2 preferred.
func DefaultTransportConfig() TransportConfig {
	const (
		defaultMaxIdle        = 100
		defaultMaxIdlePerHost = 20
		defaultIdleTimeout    = 90 * time.Second
		defaultTLSTimeout     = 10 * time.Second
		defaultExpectTimeout  = time.Second
		defaultDialTimeout    = 10 * time.Second
		defaultKeepAlive      = 30 * time.Second
	)

	return TransportConfig{
		MaxIdleConns:          defaultMaxIdle,
		MaxIdleConnsPerHost:   defaultMaxIdlePerHost,
		MaxConnsPerHost:       0, // unlimited
		IdleConnTimeout:       defaultIdleTimeout,
		TLSHandshakeTimeout:   defaultTLSTimeout,
		ExpectContinueTimeout: defaultExpectTimeout,
		DialTimeout:           defaultDialTimeout,
		KeepAlive:             defaultKeepAlive,
		DisableHTTP2:          false,
		TLSConfig:             nil,
	}
}

func buildTransport(cfg TransportConfig) *http.Transport {
	const (
		fallbackDialTimeout = 10 * time.Second
		fallbackKeepAlive   = 30 * time.Second
		fallbackMaxIdle     = 100
		fallbackMaxIdlePH   = 20
		fallbackIdleTimeout = 90 * time.Second
		fallbackTLSTimeout  = 10 * time.Second
	)

	dialTimeout := cfg.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = fallbackDialTimeout
	}

	keepAlive := cfg.KeepAlive
	if keepAlive == 0 {
		keepAlive = fallbackKeepAlive
	}

	dialer := &net.Dialer{ //nolint:exhaustruct // only relevant fields set; stdlib struct
		Timeout:   dialTimeout,
		KeepAlive: keepAlive,
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = fallbackMaxIdle
	}

	maxIdlePH := cfg.MaxIdleConnsPerHost
	if maxIdlePH == 0 {
		maxIdlePH = fallbackMaxIdlePH
	}

	idleTimeout := cfg.IdleConnTimeout
	if idleTimeout == 0 {
		idleTimeout = fallbackIdleTimeout
	}

	tlsTimeout := cfg.TLSHandshakeTimeout
	if tlsTimeout == 0 {
		tlsTimeout = fallbackTLSTimeout
	}

	return &http.Transport{ //nolint:exhaustruct // only relevant fields set; stdlib struct
		DialContext:           dialer.DialContext,
		MaxIdleConns:          maxIdle,
		MaxIdleConnsPerHost:   maxIdlePH,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   tlsTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
		TLSClientConfig:       cfg.TLSConfig,
		ForceAttemptHTTP2:     !cfg.DisableHTTP2,
	}
}

// -----------------------------------------------------------------------------
// OAuth client credentials token fetch
// -----------------------------------------------------------------------------

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// refreshClientCredentials fetches a client-credentials access token.
func (c *Client) refreshClientCredentials(ctx context.Context) error {
	creds := base64.StdEncoding.EncodeToString(
		[]byte(c.cfg.clientID + ":" + c.cfg.clientSecret),
	)

	body := strings.NewReader("grant_type=client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", body)
	if err != nil {
		return fmt.Errorf("wise: build token request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(headerUserAgent, c.cfg.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wise: token request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("wise: read token response: %w", err)
	}

	if !isSuccess(resp.StatusCode) {
		return parseAPIError(resp, raw)
	}

	var tok tokenResponse
	if err := json.Unmarshal(raw, &tok); err != nil {
		return fmt.Errorf("wise: decode token response: %w", err)
	}

	expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	c.setToken(tok.AccessToken, tok.RefreshToken, expiry)

	return nil
}
