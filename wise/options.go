package wise

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/iamkanishka/wise/wise/internal/ratelimit"
)

// TokenRefreshFunc is called when the current OAuth access token expires.
// It must return a new access token, refresh token, and expiry time.
type TokenRefreshFunc func(
	ctx context.Context,
	refreshToken string,
) (accessToken, newRefreshToken string, expiry time.Time, err error)

// RequestHook is called before every HTTP request is dispatched.
// The hook may mutate the request (e.g. add trace headers).
// Return a non-nil error to abort the call.
type RequestHook func(ctx context.Context, req *http.Request) error

// ResponseHook is called after every HTTP response is received.
// StatusCode is 0 if a transport-level error occurred (err will be non-nil).
type ResponseHook func(
	ctx context.Context,
	req *http.Request,
	statusCode int,
	latency time.Duration,
	err error,
)

// config holds the resolved client configuration.
type config struct {
	env         Environment
	baseURL     string
	timeout     time.Duration
	maxRetries  int
	retryDelay  time.Duration
	userAgent   string
	logger      *slog.Logger
	httpClient  *http.Client
	rateLimiter ratelimit.Limiter
	cb          *CircuitBreaker

	requestHooks  []RequestHook
	responseHooks []ResponseHook

	// Auth fields.
	authMode         AuthMode
	personalToken    string
	clientID         string
	clientSecret     string
	refreshToken     string
	tokenRefreshFunc TokenRefreshFunc
}

func defaultConfig() *config {
	return &config{
		env:              Sandbox,
		baseURL:          "",
		timeout:          defaultTimeout,
		maxRetries:       defaultMaxRetries,
		retryDelay:       500 * time.Millisecond, //nolint:mnd // well-known default
		userAgent:        defaultUserAgent,
		logger:           nil,
		httpClient:       nil,
		rateLimiter:      nil,
		cb:               nil,
		requestHooks:     nil,
		responseHooks:    nil,
		authMode:         AuthPersonalToken,
		personalToken:    "",
		clientID:         "",
		clientSecret:     "",
		refreshToken:     "",
		tokenRefreshFunc: nil,
	}
}

func (c *config) validate() error {
	switch c.authMode {
	case AuthPersonalToken:
		if c.personalToken == "" {
			return fmt.Errorf("wise: personal token is required for AuthPersonalToken")
		}

	case AuthClientCredentials:
		if c.clientID == "" || c.clientSecret == "" {
			return fmt.Errorf("wise: clientID and clientSecret are required for AuthClientCredentials")
		}

	case AuthUserToken:
		if c.personalToken == "" && c.tokenRefreshFunc == nil {
			return fmt.Errorf("wise: an access token or TokenRefreshFunc is required for AuthUserToken")
		}

	default:
		return fmt.Errorf("wise: unknown auth mode %d", c.authMode)
	}

	return nil
}

// Option is a functional option for [New].
type Option func(*config) error

// WithEnvironment selects Production or Sandbox (default: Sandbox).
func WithEnvironment(env Environment) Option {
	return func(c *config) error {
		c.env = env
		return nil
	}
}

// WithBaseURL overrides the base URL entirely (useful for proxies or local mocks).
func WithBaseURL(u string) Option {
	return func(c *config) error {
		c.baseURL = u
		return nil
	}
}

// WithPersonalToken configures authentication via a Wise Personal API Token.
// Generate tokens at: Wise.com → Settings → Integrations and Tools → API tokens.
func WithPersonalToken(token string) Option {
	return func(c *config) error {
		c.personalToken = token
		c.authMode = AuthPersonalToken
		return nil
	}
}

// WithClientCredentials configures OAuth 2.0 client_credentials authentication.
// An access token is fetched automatically on startup and refreshed when expired.
func WithClientCredentials(clientID, clientSecret string) Option {
	return func(c *config) error {
		c.clientID = clientID
		c.clientSecret = clientSecret
		c.authMode = AuthClientCredentials
		return nil
	}
}

// WithUserToken configures authentication with an existing OAuth access token.
// Provide a [TokenRefreshFunc] to enable automatic token renewal on expiry.
func WithUserToken(
	accessToken, refreshToken string,
	expiry time.Time,
	refresh TokenRefreshFunc,
) Option {
	return func(c *config) error {
		c.personalToken = accessToken
		c.refreshToken = refreshToken
		c.authMode = AuthUserToken
		c.tokenRefreshFunc = refresh
		_ = expiry // stored on the client after construction

		return nil
	}
}

// WithTimeout sets the per-request HTTP timeout (default: 30s).
func WithTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("wise: timeout must be positive, got %v", d)
		}

		c.timeout = d

		return nil
	}
}

// WithMaxRetries sets the maximum number of retry attempts on 429 / 5xx responses.
// Set to 0 to disable retries entirely. Default: 3.
func WithMaxRetries(n int) Option {
	return func(c *config) error {
		if n < 0 {
			return fmt.Errorf("wise: maxRetries must be >= 0, got %d", n)
		}

		c.maxRetries = n

		return nil
	}
}

// WithLogger sets the structured [*slog.Logger] used for request/response logging.
// Defaults to a stderr handler at WARN level.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) error {
		c.logger = l
		return nil
	}
}

// WithHTTPClient replaces the default [http.Client]. Its transport will be
// wrapped by the retry and rate-limiting layers.
func WithHTTPClient(h *http.Client) Option {
	return func(c *config) error {
		c.httpClient = h
		return nil
	}
}

// WithRateLimiter plugs in a custom [ratelimit.Limiter].
// The default is a token bucket permitting ~10 req/s with a burst of 20.
func WithRateLimiter(rl ratelimit.Limiter) Option {
	return func(c *config) error {
		c.rateLimiter = rl
		return nil
	}
}

// WithCircuitBreaker adds a [CircuitBreaker] to the transport chain.
// The breaker sits between the retry layer and the underlying HTTP transport.
func WithCircuitBreaker(cb *CircuitBreaker) Option {
	return func(c *config) error {
		c.cb = cb
		return nil
	}
}

// WithRequestHook registers a hook that runs before every HTTP request.
// Multiple hooks are applied in registration order.
func WithRequestHook(h RequestHook) Option {
	return func(c *config) error {
		c.requestHooks = append(c.requestHooks, h)
		return nil
	}
}

// WithResponseHook registers a hook that runs after every HTTP response.
// Use it for metrics, distributed tracing, or structured logging.
func WithResponseHook(h ResponseHook) Option {
	return func(c *config) error {
		c.responseHooks = append(c.responseHooks, h)
		return nil
	}
}

// WithUserAgent prepends a token to the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *config) error {
		c.userAgent = ua + " " + defaultUserAgent
		return nil
	}
}

// WithTransportConfig applies TCP/TLS transport tuning.
// Use [DefaultTransportConfig] as a starting point.
func WithTransportConfig(cfg TransportConfig) Option {
	return func(c *config) error {
		t := buildTransport(cfg)
		if c.httpClient == nil {
			c.httpClient = &http.Client{Transport: t} //nolint:exhaustruct // only relevant fields set; stdlib struct
		} else {
			c.httpClient.Transport = t
		}

		return nil
	}
}

// SlogLoggingHook returns a [ResponseHook] that logs every Wise API call
// using the provided [*slog.Logger].
// Successful calls are logged at DEBUG level; failures at WARN level.
func SlogLoggingHook(logger *slog.Logger) ResponseHook {
	return func(ctx context.Context, req *http.Request, code int, latency time.Duration, err error) {
		attrs := []any{
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.Int("status", code),
			slog.Duration("latency", latency),
		}

		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			logger.WarnContext(ctx, "wise api call failed", attrs...)

			return
		}

		logger.DebugContext(ctx, "wise api call", attrs...)
	}
}

// MetricsHook returns a [ResponseHook] adapter for Prometheus-style counters.
//
//	wise.WithResponseHook(wise.MetricsHook(func(method, path string, code int, d time.Duration, failed bool) {
//	    apiTotal.WithLabelValues(method, path, fmt.Sprint(code)).Inc()
//	    apiDuration.WithLabelValues(method, path).Observe(d.Seconds())
//	}))
func MetricsHook(fn func(method, path string, statusCode int, latency time.Duration, failed bool)) ResponseHook {
	return func(_ context.Context, req *http.Request, code int, latency time.Duration, err error) {
		fn(req.Method, req.URL.Path, code, latency, err != nil || code >= http.StatusBadRequest)
	}
}
