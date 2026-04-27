package wise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// ProductionBaseURL is the Wise production API base URL.
	ProductionBaseURL = "https://api.wise.com"

	// SandboxBaseURL is the Wise sandbox API base URL for integration testing.
	SandboxBaseURL = "https://api.wise-sandbox.com"
)

// Client is the main Wise API client.
// All service methods are accessed through the typed service fields.
// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	httpClient *http.Client
	cfg        *config

	// token state — guarded by mu.
	mu           sync.RWMutex
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time

	// Services.
	Profiles             *ProfileService
	Quotes               *QuoteService
	Recipients           *RecipientService
	Transfers            *TransferService
	Balances             *BalanceService
	Rates                *RateService
	Cards                *CardService
	CardOrders           *CardOrderService
	CardTxns             *CardTransactionService
	Batches              *BatchGroupService
	Webhooks             *WebhookService
	Currencies           *CurrencyService
	Statements           *StatementService
	BankAccounts         *BankAccountService
	Activities           *ActivityService
	Comparisons          *ComparisonService
	Simulations          *SimulationService
	SpendLimits          *SpendLimitService
	SpendControls        *SpendControlService
	Disputes             *DisputeService
	Addresses            *AddressService
	DirectDebits         *DirectDebitAccountService
	KYC                  *KYCService
	KYCReview            *KYCReviewService
	OAuthSvc             *OAuthService
	Cases                *CasesService
	PushProvisioning     *PushProvisioningService
	KioskCollection      *KioskCollectionService
	MultiCurrencyAccount *MultiCurrencyAccountService
	Users                *UserService
	UserSecurity         *UserSecurityService
	SCA                  *SCAService
	Contacts             *ContactService
	FaceTec              *FaceTecService
	JOSE                 *JOSEService
	OTT                  *OTTService
	ThreeDS              *ThreeDSService
	ClaimAccount         *ClaimAccountService
}

// New constructs a Client with the supplied options.
//
//	client, err := wise.New(
//	    wise.WithPersonalToken(os.Getenv("WISE_API_TOKEN")),
//	    wise.WithEnvironment(wise.Sandbox),
//	)
func New(opts ...Option) (*Client, error) {
	cfg := defaultConfig()

	for _, o := range opts {
		if err := o(cfg); err != nil {
			return nil, fmt.Errorf("wise: option error: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	c := newClient(resolveBaseURL(cfg), buildHTTPClient(cfg), cfg)

	if cfg.authMode == AuthClientCredentials && cfg.personalToken == "" {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		defer cancel()

		if err := c.refreshClientCredentials(ctx); err != nil {
			return nil, fmt.Errorf("wise: initial token fetch: %w", err)
		}
	}

	c.registerServices()

	return c, nil
}

// resolveBaseURL returns the effective API base URL from the config.
func resolveBaseURL(cfg *config) string {
	if cfg.baseURL != "" {
		return strings.TrimRight(cfg.baseURL, "/")
	}

	if cfg.env == Sandbox {
		return SandboxBaseURL
	}

	return ProductionBaseURL
}

// buildHTTPClient assembles the layered http.Client from config.
// Transport stack (inner→outer): base → circuit-breaker → retry → hooks.
func buildHTTPClient(cfg *config) *http.Client {
	var base http.RoundTripper = http.DefaultTransport

	if cfg.httpClient != nil && cfg.httpClient.Transport != nil {
		base = cfg.httpClient.Transport
	}

	if cfg.cb != nil {
		base = cfg.cb.Wrap(base)
	}

	rt := newRetryTransport(cfg)
	rt.inner = base
	outer := applyHooks(cfg, rt)

	if cfg.httpClient != nil {
		cfg.httpClient.Transport = outer
		return cfg.httpClient
	}

	return &http.Client{ //nolint:exhaustruct // only relevant fields set; stdlib struct
		Timeout:   cfg.timeout,
		Transport: outer,
	}
}

// newClient allocates a Client with all service fields nil.
// Call registerServices() after construction to populate them.
func newClient(baseURL string, httpClient *http.Client, cfg *config) *Client {
	return &Client{
		baseURL:              baseURL,
		httpClient:           httpClient,
		cfg:                  cfg,
		mu:                   sync.RWMutex{},
		accessToken:          cfg.personalToken,
		refreshToken:         cfg.refreshToken,
		tokenExpiry:          time.Time{},
		Profiles:             nil,
		Quotes:               nil,
		Recipients:           nil,
		Transfers:            nil,
		Balances:             nil,
		Rates:                nil,
		Cards:                nil,
		CardOrders:           nil,
		CardTxns:             nil,
		Batches:              nil,
		Webhooks:             nil,
		Currencies:           nil,
		Statements:           nil,
		BankAccounts:         nil,
		Activities:           nil,
		Comparisons:          nil,
		Simulations:          nil,
		SpendLimits:          nil,
		SpendControls:        nil,
		Disputes:             nil,
		Addresses:            nil,
		DirectDebits:         nil,
		KYC:                  nil,
		KYCReview:            nil,
		OAuthSvc:             nil,
		Cases:                nil,
		PushProvisioning:     nil,
		KioskCollection:      nil,
		MultiCurrencyAccount: nil,
		Users:                nil,
		UserSecurity:         nil,
		SCA:                  nil,
		Contacts:             nil,
		FaceTec:              nil,
		JOSE:                 nil,
		OTT:                  nil,
		ThreeDS:              nil,
		ClaimAccount:         nil,
	}
}

func (c *Client) registerServices() {
	c.Profiles = &ProfileService{c: c}
	c.Quotes = &QuoteService{c: c}
	c.Recipients = &RecipientService{c: c}
	c.Transfers = &TransferService{c: c}
	c.Balances = &BalanceService{c: c}
	c.Rates = &RateService{c: c}
	c.Cards = &CardService{c: c}
	c.CardOrders = &CardOrderService{c: c}
	c.CardTxns = &CardTransactionService{c: c}
	c.Batches = &BatchGroupService{c: c}
	c.Webhooks = &WebhookService{c: c}
	c.Currencies = &CurrencyService{c: c}
	c.Statements = &StatementService{c: c}
	c.BankAccounts = &BankAccountService{c: c}
	c.Activities = &ActivityService{c: c}
	c.Comparisons = &ComparisonService{c: c}
	c.Simulations = &SimulationService{c: c}
	c.SpendLimits = &SpendLimitService{c: c}
	c.SpendControls = &SpendControlService{c: c}
	c.Disputes = &DisputeService{c: c}
	c.Addresses = &AddressService{c: c}
	c.DirectDebits = &DirectDebitAccountService{c: c}
	c.KYC = &KYCService{c: c}
	c.KYCReview = &KYCReviewService{c: c}
	c.OAuthSvc = &OAuthService{c: c}
	c.Cases = &CasesService{c: c}
	c.PushProvisioning = &PushProvisioningService{c: c}
	c.KioskCollection = &KioskCollectionService{c: c}
	c.MultiCurrencyAccount = &MultiCurrencyAccountService{c: c}
	c.Users = &UserService{c: c}
	c.UserSecurity = &UserSecurityService{c: c}
	c.SCA = &SCAService{c: c}
	c.Contacts = &ContactService{c: c}
	c.FaceTec = &FaceTecService{c: c}
	c.JOSE = &JOSEService{c: c}
	c.OTT = &OTTService{c: c}
	c.ThreeDS = &ThreeDSService{c: c}
	c.ClaimAccount = &ClaimAccountService{c: c}
}

// -----------------------------------------------------------------------------
// HTTP dispatch
// -----------------------------------------------------------------------------

// newRequest builds an authenticated *http.Request.
func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("wise: invalid URL %q: %w", path, err)
	}

	var bodyReader io.Reader

	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("wise: marshal request body: %w", marshalErr)
		}

		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("wise: build request: %w", err)
	}

	req.Header.Set(headerUserAgent, c.cfg.userAgent)

	if body != nil {
		req.Header.Set(headerContentType, contentTypeJSON)
	}

	applyContextHeaders(ctx, req)

	token, err := c.currentToken(ctx)
	if err != nil {
		return nil, err
	}

	req.Header.Set(headerAuthorization, "Bearer "+token)

	return req, nil
}

// do executes req and decodes the JSON response into dst.
// If dst is nil the response body is discarded.
func (c *Client) do(req *http.Request, dst any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wise: http: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("wise: read response body: %w", err)
	}

	if !isSuccess(resp.StatusCode) {
		return parseAPIError(resp, body)
	}

	if dst != nil && len(body) > 0 {
		if err := json.Unmarshal(body, dst); err != nil {
			return fmt.Errorf("wise: decode response (%.200s): %w", body, err)
		}
	}

	return nil
}

// get is a convenience wrapper for GET requests.
func (c *Client) get(ctx context.Context, path string, params url.Values, dst any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}

	if len(params) > 0 {
		req.URL.RawQuery = params.Encode()
	}

	return c.do(req, dst)
}

// post is a convenience wrapper for POST requests.
func (c *Client) post(ctx context.Context, path string, body, dst any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}

	return c.do(req, dst)
}

// put is a convenience wrapper for PUT requests.
func (c *Client) put(ctx context.Context, path string, body, dst any) error {
	req, err := c.newRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}

	return c.do(req, dst)
}

// patch is a convenience wrapper for PATCH requests.
func (c *Client) patch(ctx context.Context, path string, body, dst any) error {
	req, err := c.newRequest(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}

	return c.do(req, dst)
}

// delete is a convenience wrapper for DELETE requests.
func (c *Client) delete(ctx context.Context, path string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

// getRaw executes a GET and returns raw bytes (used for binary statement formats).
func (c *Client) getRaw(ctx context.Context, path string, params url.Values) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	if len(params) > 0 {
		req.URL.RawQuery = params.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wise: http: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	const maxRawBytes = 50 << 20 // 50 MiB for PDF/XLSX statements

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRawBytes))
	if err != nil {
		return nil, fmt.Errorf("wise: read raw response: %w", err)
	}

	if !isSuccess(resp.StatusCode) {
		return nil, parseAPIError(resp, body)
	}

	return body, nil
}

// isSuccess reports whether the HTTP status code indicates success.
func isSuccess(code int) bool { return code >= http.StatusOK && code < http.StatusMultipleChoices }

// -----------------------------------------------------------------------------
// Token management
// -----------------------------------------------------------------------------

// currentToken returns the current bearer token, refreshing if necessary.
func (c *Client) currentToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	tok := c.accessToken
	exp := c.tokenExpiry
	c.mu.RUnlock()

	// Token is valid if it expires more than 30s from now (or has no expiry).
	const tokenGrace = 30 * time.Second

	if tok != "" && (exp.IsZero() || time.Now().Before(exp.Add(-tokenGrace))) {
		return tok, nil
	}

	switch c.cfg.authMode {
	case AuthClientCredentials:
		if err := c.refreshClientCredentials(ctx); err != nil {
			return tok, err
		}

		c.mu.RLock()
		tok = c.accessToken
		c.mu.RUnlock()

	case AuthUserToken:
		if c.cfg.tokenRefreshFunc != nil {
			c.mu.RLock()
			oldRefresh := c.refreshToken
			c.mu.RUnlock()

			newTok, newRefresh, newExp, err := c.cfg.tokenRefreshFunc(ctx, oldRefresh)
			if err != nil {
				return "", fmt.Errorf("wise: token refresh: %w", err)
			}

			c.setToken(newTok, newRefresh, newExp)

			return newTok, nil
		}

	case AuthPersonalToken:
		// Personal tokens do not expire; return as-is.
	}

	return tok, nil
}

func (c *Client) setToken(access, refresh string, expiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accessToken = access
	c.refreshToken = refresh
	c.tokenExpiry = expiry
}

// -----------------------------------------------------------------------------
// Escape hatch
// -----------------------------------------------------------------------------

// RawRequest is the input for [Client.DoRaw].
type RawRequest struct {
	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE).
	Method string

	// Path is the API path starting with /v (e.g. "/v1/profiles").
	Path string

	// Body is the request body, marshaled to JSON if non-nil.
	Body any

	// Query holds URL query parameters.
	Query map[string]string

	// Headers holds additional headers; existing headers are preserved.
	Headers map[string]string
}

// RawResponse is returned by [Client.DoRaw].
type RawResponse struct {
	// StatusCode is the HTTP response status code.
	StatusCode int

	// Body is the raw response body.
	Body []byte

	// Headers are the response headers.
	Headers http.Header
}

// DoRaw executes an arbitrary Wise API request and returns the raw response.
// Use this for endpoints not yet covered by the typed service methods.
//
//	resp, err := client.DoRaw(ctx, wise.RawRequest{
//	    Method: "POST",
//	    Path:   "/v1/profiles/12345/some-new-endpoint",
//	    Body:   map[string]any{"key": "value"},
//	})
func (c *Client) DoRaw(ctx context.Context, r RawRequest) (*RawResponse, error) {
	req, err := c.newRequest(ctx, r.Method, r.Path, r.Body)
	if err != nil {
		return nil, fmt.Errorf("wise: DoRaw build request: %w", err)
	}

	if len(r.Query) > 0 {
		q := req.URL.Query()

		for k, v := range r.Query {
			q.Set(k, v)
		}

		req.URL.RawQuery = q.Encode()
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wise: DoRaw: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("wise: DoRaw read body: %w", err)
	}

	raw := &RawResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}

	if !isSuccess(resp.StatusCode) {
		return raw, parseAPIError(resp, body)
	}

	return raw, nil
}

// Ping verifies API connectivity by listing supported currencies.
// Returns nil on a successful response.
//
//	if err := client.Ping(ctx); err != nil {
//	    log.Fatal("Wise API unreachable:", err)
//	}
func (c *Client) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/currencies", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wise: ping: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("wise: ping returned HTTP %d", resp.StatusCode)
	}

	return nil
}
