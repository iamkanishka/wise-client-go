package wise_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iamkanishka/wise-go/wise"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestServer starts an httptest.Server and returns a pre-configured client.
func newTestServer(t *testing.T, h http.Handler) *wise.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := wise.New(
		wise.WithPersonalToken("test-token"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
		wise.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("wise.New: %v", err)
	}

	return c
}

// jsonResp writes v as JSON with status code.
func jsonResp(t *testing.T, code int, v any) http.Handler {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		if err := json.NewEncoder(w).Encode(v); err != nil {
			t.Errorf("jsonResp encode: %v", err)
		}
	})
}

// router is a simple path+method multiplexer for tests.
type router struct {
	routes map[string]http.Handler
}

func newRouter() *router { return &router{routes: make(map[string]http.Handler)} }

func (r *router) Handle(method, path string, h http.Handler) {
	r.routes[method+" "+path] = h
}

func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	key := req.Method + " " + req.URL.Path

	if h, ok := r.routes[key]; ok {
		h.ServeHTTP(w, req)
		return
	}

	for k, h := range r.routes {
		if strings.HasPrefix(key, k) {
			h.ServeHTTP(w, req)
			return
		}
	}

	http.NotFound(w, req)
}

// hmacSHA256 computes the HMAC-SHA256 used by VerifyWebhookSignature.
func hmacSHA256(secret string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(secret))
	h.Write(body)

	return hex.EncodeToString(h.Sum(nil))
}

// ---------------------------------------------------------------------------
// Client construction
// ---------------------------------------------------------------------------

func TestNew_PersonalToken(t *testing.T) {
	c, err := wise.New(wise.WithPersonalToken("tok"))
	if err != nil || c == nil {
		t.Fatalf("unexpected: err=%v c=%v", err, c)
	}
}

func TestNew_NoAuth_Error(t *testing.T) {
	_, err := wise.New()
	if err == nil {
		t.Fatal("expected error with no auth configured")
	}
}

func TestNew_InvalidTimeout(t *testing.T) {
	_, err := wise.New(wise.WithPersonalToken("tok"), wise.WithTimeout(-1))
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
}

func TestNew_InvalidMaxRetries(t *testing.T) {
	_, err := wise.New(wise.WithPersonalToken("tok"), wise.WithMaxRetries(-1))
	if err == nil {
		t.Fatal("expected error for negative maxRetries")
	}
}

func TestNew_AllServicesNonNil(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	checks := []struct {
		name string
		ok   bool
	}{
		{"Profiles", c.Profiles != nil},
		{"Quotes", c.Quotes != nil},
		{"Transfers", c.Transfers != nil},
		{"Balances", c.Balances != nil},
		{"Recipients", c.Recipients != nil},
		{"Rates", c.Rates != nil},
		{"Cards", c.Cards != nil},
		{"CardOrders", c.CardOrders != nil},
		{"CardTxns", c.CardTxns != nil},
		{"Batches", c.Batches != nil},
		{"Webhooks", c.Webhooks != nil},
		{"Currencies", c.Currencies != nil},
		{"Statements", c.Statements != nil},
		{"BankAccounts", c.BankAccounts != nil},
		{"Activities", c.Activities != nil},
		{"Comparisons", c.Comparisons != nil},
		{"Simulations", c.Simulations != nil},
		{"SpendLimits", c.SpendLimits != nil},
		{"SpendControls", c.SpendControls != nil},
		{"Disputes", c.Disputes != nil},
		{"Addresses", c.Addresses != nil},
		{"DirectDebits", c.DirectDebits != nil},
		{"KYC", c.KYC != nil},
		{"KYCReview", c.KYCReview != nil},
		{"OAuthSvc", c.OAuthSvc != nil},
		{"Cases", c.Cases != nil},
		{"PushProvisioning", c.PushProvisioning != nil},
		{"KioskCollection", c.KioskCollection != nil},
		{"MultiCurrencyAccount", c.MultiCurrencyAccount != nil},
		{"Users", c.Users != nil},
		{"UserSecurity", c.UserSecurity != nil},
		{"SCA", c.SCA != nil},
		{"Contacts", c.Contacts != nil},
		{"FaceTec", c.FaceTec != nil},
		{"JOSE", c.JOSE != nil},
		{"OTT", c.OTT != nil},
		{"ThreeDS", c.ThreeDS != nil},
		{"ClaimAccount", c.ClaimAccount != nil},
	}

	for _, tc := range checks {
		if !tc.ok {
			t.Errorf("service %s is nil", tc.name)
		}
	}
}

// ---------------------------------------------------------------------------
// Auth header
// ---------------------------------------------------------------------------

func TestClient_BearerHeader(t *testing.T) {
	var gotAuth string

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	})

	c := newTestServer(t, h)
	_, _ = c.Currencies.List(context.Background())

	if gotAuth != "Bearer test-token" {
		t.Errorf("want 'Bearer test-token', got %q", gotAuth)
	}
}

func TestClient_UserAgentHeader(t *testing.T) {
	var gotUA string

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	})

	c := newTestServer(t, h)
	_, _ = c.Currencies.List(context.Background())

	if !strings.Contains(gotUA, "wise-go") {
		t.Errorf("want wise-go in User-Agent, got %q", gotUA)
	}
}

// ---------------------------------------------------------------------------
// Profile service
// ---------------------------------------------------------------------------

func TestProfiles_List(t *testing.T) {
	want := []wise.Profile{
		{ID: 1, Type: wise.ProfileTypePersonal},
		{ID: 2, Type: wise.ProfileTypeBusiness},
	}
	c := newTestServer(t, jsonResp(t, 200, want))

	got, err := c.Profiles.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("want 2 profiles, got %d", len(got))
	}
}

func TestProfiles_Get(t *testing.T) {
	want := wise.Profile{ID: 42, Type: wise.ProfileTypePersonal}

	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/42", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Profiles.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != 42 {
		t.Errorf("want 42, got %d", got.ID)
	}
}

func TestProfiles_UpdatePersonal(t *testing.T) {
	want := wise.Profile{ID: 1, Type: wise.ProfileTypePersonal}

	rt := newRouter()
	rt.Handle("PUT", "/v1/profiles/1", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Profiles.UpdatePersonal(context.Background(), 1, wise.UpdatePersonalRequest{FirstName: "Alice"})
	if err != nil {
		t.Fatalf("UpdatePersonal: %v", err)
	}

	if got.ID != 1 {
		t.Errorf("want 1, got %d", got.ID)
	}
}

func TestProfiles_UpdateBusiness(t *testing.T) {
	want := wise.Profile{ID: 2, Type: wise.ProfileTypeBusiness}

	rt := newRouter()
	rt.Handle("PUT", "/v1/profiles/2/business", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Profiles.UpdateBusiness(context.Background(), 2, wise.CreateBusinessRequest{Name: "Acme"})
	if err != nil {
		t.Fatalf("UpdateBusiness: %v", err)
	}

	if got.Type != wise.ProfileTypeBusiness {
		t.Errorf("want business, got %s", got.Type)
	}
}

// ---------------------------------------------------------------------------
// Quote service
// ---------------------------------------------------------------------------

func TestQuotes_Create(t *testing.T) {
	want := wise.Quote{ID: "q-1", SourceCurrency: "USD", TargetCurrency: "GBP", Rate: 0.79}

	rt := newRouter()
	rt.Handle("POST", "/v3/profiles/1/quotes", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Quotes.Create(context.Background(), 1, wise.CreateQuoteRequest{
		SourceCurrency: "USD",
		TargetCurrency: "GBP",
		SourceAmount:   wise.Ptr(1000.0),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != "q-1" {
		t.Errorf("want q-1, got %s", got.ID)
	}

	if got.Rate != 0.79 {
		t.Errorf("want rate 0.79, got %.2f", got.Rate)
	}
}

func TestQuotes_Get(t *testing.T) {
	want := wise.Quote{ID: "q-abc", SourceCurrency: "EUR"}

	rt := newRouter()
	rt.Handle("GET", "/v3/profiles/5/quotes/q-abc", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Quotes.Get(context.Background(), 5, "q-abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.SourceCurrency != "EUR" {
		t.Errorf("want EUR, got %s", got.SourceCurrency)
	}
}

func TestQuotes_Update(t *testing.T) {
	want := wise.Quote{ID: "q-1", TargetAccount: 99}

	rt := newRouter()
	rt.Handle("PATCH", "/v3/profiles/1/quotes/q-1", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Quotes.Update(context.Background(), 1, "q-1", wise.UpdateQuoteRequest{TargetAccount: 99})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got.TargetAccount != 99 {
		t.Errorf("want 99, got %d", got.TargetAccount)
	}
}

// ---------------------------------------------------------------------------
// Transfer service
// ---------------------------------------------------------------------------

func TestTransfers_Create(t *testing.T) {
	want := wise.Transfer{ID: 9001, Status: wise.TransferStatusDraft, CustomerTransactionID: "idem-1"}

	rt := newRouter()
	rt.Handle("POST", "/v1/transfers", jsonResp(t, 201, want))

	c := newTestServer(t, rt)

	got, err := c.Transfers.Create(context.Background(), wise.CreateTransferRequest{
		TargetAccount:         123,
		QuoteUUID:             "q-uuid",
		CustomerTransactionID: "idem-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != 9001 {
		t.Errorf("want 9001, got %d", got.ID)
	}
}

func TestTransfers_Fund(t *testing.T) {
	want := wise.FundResponse{Type: "BALANCE", Status: "COMPLETED"}

	rt := newRouter()
	rt.Handle("POST", "/v3/profiles/1/transfers/42/payments", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Transfers.Fund(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("Fund: %v", err)
	}

	if got.Status != "COMPLETED" {
		t.Errorf("want COMPLETED, got %s", got.Status)
	}
}

func TestTransfers_Cancel(t *testing.T) {
	want := wise.Transfer{ID: 7, Status: wise.TransferStatusCanceled}

	rt := newRouter()
	rt.Handle("POST", "/v1/transfers/7/cancel", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Transfers.Cancel(context.Background(), 7)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if got.Status != wise.TransferStatusCanceled {
		t.Errorf("want canceled, got %s", got.Status)
	}
}

func TestTransfers_List(t *testing.T) {
	want := []wise.Transfer{{ID: 1}, {ID: 2}, {ID: 3}}

	rt := newRouter()
	rt.Handle("GET", "/v1/transfers", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Transfers.List(context.Background(), wise.ListTransfersParams{ProfileID: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("want 3, got %d", len(got))
	}
}

func TestTransfers_DeliveryEstimate(t *testing.T) {
	want := wise.DeliveryEstimate{Guaranteed: true, Source: "TRACKER"}

	rt := newRouter()
	rt.Handle("GET", "/v1/delivery-estimates/55", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Transfers.DeliveryEstimate(context.Background(), 55)
	if err != nil {
		t.Fatalf("DeliveryEstimate: %v", err)
	}

	if !got.Guaranteed {
		t.Error("want Guaranteed=true")
	}
}

// ---------------------------------------------------------------------------
// Balance service
// ---------------------------------------------------------------------------

func TestBalances_Create(t *testing.T) {
	want := wise.Balance{ID: 100, Currency: "GBP", Type: wise.BalanceTypeStandard}

	rt := newRouter()
	rt.Handle("POST", "/v4/profiles/1/balances", jsonResp(t, 201, want))

	c := newTestServer(t, rt)

	got, err := c.Balances.Create(context.Background(), 1, wise.CreateBalanceRequest{
		Currency: "GBP",
		Type:     wise.BalanceTypeStandard,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.Currency != "GBP" {
		t.Errorf("want GBP, got %s", got.Currency)
	}
}

func TestBalances_List(t *testing.T) {
	want := []wise.Balance{{ID: 1, Currency: "USD"}, {ID: 2, Currency: "EUR"}}

	rt := newRouter()
	rt.Handle("GET", "/v4/profiles/1/balances", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Balances.List(context.Background(), 1, wise.BalanceTypeStandard)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("want 2, got %d", len(got))
	}
}

func TestBalances_Close(t *testing.T) {
	rt := newRouter()
	rt.Handle("DELETE", "/v4/profiles/1/balances/99", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	c := newTestServer(t, rt)

	if err := c.Balances.Close(context.Background(), 1, 99); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestBalances_GetTotalFunds(t *testing.T) {
	want := wise.TotalFunds{Currency: "USD", TotalWorth: wise.Amount{Value: 5000, Currency: "USD"}}

	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/1/total-funds/USD", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Balances.GetTotalFunds(context.Background(), 1, "USD")
	if err != nil {
		t.Fatalf("GetTotalFunds: %v", err)
	}

	if got.TotalWorth.Value != 5000 {
		t.Errorf("want 5000, got %f", got.TotalWorth.Value)
	}
}

func TestBalances_GetDepositLimits(t *testing.T) {
	maxLimit := 5000.0
	want := []wise.DepositLimits{{Currency: "SGD", Max: &maxLimit}}

	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/1/balance-capacity", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Balances.GetDepositLimits(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetDepositLimits: %v", err)
	}

	if len(got) != 1 || got[0].Currency != "SGD" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestBalances_SetExcessMoneyAccount(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v1/profiles/1/excess-money-account", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]int64
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		if body["recipientId"] != 999 {
			http.Error(w, "wrong recipient", 400)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	c := newTestServer(t, rt)

	if err := c.Balances.SetExcessMoneyAccount(context.Background(), 1, 999); err != nil {
		t.Fatalf("SetExcessMoneyAccount: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Currency service
// ---------------------------------------------------------------------------

func TestCurrencies_List(t *testing.T) {
	want := []wise.Currency{{Code: "USD", Name: "US Dollar"}, {Code: "GBP", Name: "British Pound"}}
	c := newTestServer(t, jsonResp(t, 200, want))

	got, err := c.Currencies.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("want 2, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Recipient service
// ---------------------------------------------------------------------------

func TestRecipients_Create(t *testing.T) {
	want := wise.RecipientAccount{ID: 55, Currency: "EUR", Active: true}

	rt := newRouter()
	rt.Handle("POST", "/v1/accounts", jsonResp(t, 201, want))

	c := newTestServer(t, rt)

	got, err := c.Recipients.Create(context.Background(), wise.CreateRecipientRequest{
		Profile:           1,
		AccountHolderName: "Alice",
		Currency:          "EUR",
		Type:              "iban",
		Details:           map[string]any{"iban": "DE89370400440532013000"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != 55 {
		t.Errorf("want 55, got %d", got.ID)
	}
}

func TestRecipients_Delete(t *testing.T) {
	rt := newRouter()
	rt.Handle("DELETE", "/v1/accounts/99", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))

	c := newTestServer(t, rt)

	if err := c.Recipients.Delete(context.Background(), 99); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Rate service
// ---------------------------------------------------------------------------

func TestRates_List(t *testing.T) {
	want := []wise.ExchangeRate{{Source: "USD", Target: "GBP", Rate: 0.79}}
	c := newTestServer(t, jsonResp(t, 200, want))

	got, err := c.Rates.List(context.Background(), wise.GetRateParams{Source: "USD", Target: "GBP"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) == 0 || got[0].Rate != 0.79 {
		t.Errorf("unexpected rates: %+v", got)
	}
}

func TestRates_Get_NotFound(t *testing.T) {
	c := newTestServer(t, jsonResp(t, 200, []wise.ExchangeRate{}))

	_, err := c.Rates.Get(context.Background(), wise.GetRateParams{Source: "USD", Target: "XYZ"})
	if !wise.IsNotFound(err) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Card service
// ---------------------------------------------------------------------------

func TestCards_ListCards(t *testing.T) {
	want := []wise.Card{
		{CardToken: "tok-1", Status: wise.CardStatusActive, Type: wise.CardTypePhysical},
		{CardToken: "tok-2", Status: wise.CardStatusFrozen, Type: wise.CardTypeVirtual},
	}

	rt := newRouter()
	rt.Handle("GET", "/v3/spend/profiles/1/cards", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Cards.ListCards(context.Background(), 1, wise.PageParams{Limit: 20})
	if err != nil {
		t.Fatalf("ListCards: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("want 2, got %d", len(got))
	}
}

func TestCards_UpdateStatus_Frozen(t *testing.T) {
	want := wise.Card{CardToken: "tok-x", Status: wise.CardStatusFrozen}

	rt := newRouter()
	rt.Handle("PUT", "/v3/spend/profiles/1/cards/tok-x/status", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Cards.UpdateStatus(context.Background(), 1, "tok-x", wise.CardStatusFrozen)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	if got.Status != wise.CardStatusFrozen {
		t.Errorf("want FROZEN, got %s", got.Status)
	}
}

func TestCards_UpdateStatus_Blocked(t *testing.T) {
	want := wise.Card{CardToken: "tok-y", Status: wise.CardStatusBlocked}

	rt := newRouter()
	rt.Handle("PUT", "/v3/spend/profiles/1/cards/tok-y/status", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Cards.UpdateStatus(context.Background(), 1, "tok-y", wise.CardStatusBlocked)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	if got.Status != wise.CardStatusBlocked {
		t.Errorf("want BLOCKED, got %s", got.Status)
	}
}

func TestCards_ResetPINCount(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v3/spend/profiles/1/cards/tok/reset-pin-count", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	c := newTestServer(t, rt)

	if err := c.Cards.ResetPINCount(context.Background(), 1, "tok"); err != nil {
		t.Fatalf("ResetPINCount: %v", err)
	}
}

func TestCards_GetSpendingPermissions(t *testing.T) {
	want := wise.SpendingPermissions{AllowTransactions: true, AllowOnlineTransactions: true}

	rt := newRouter()
	rt.Handle("GET", "/v3/spend/profiles/1/cards/tok-p/spending-permissions", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Cards.GetSpendingPermissions(context.Background(), 1, "tok-p")
	if err != nil {
		t.Fatalf("GetSpendingPermissions: %v", err)
	}

	if !got.AllowTransactions {
		t.Error("want AllowTransactions=true")
	}
}

func TestCards_UpdateSpendingPermissions(t *testing.T) {
	want := wise.SpendingPermissions{AllowOnlineTransactions: false}

	rt := newRouter()
	rt.Handle("PATCH", "/v4/spend/profiles/1/cards/tok-p2/spending-permissions", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Cards.UpdateSpendingPermissions(context.Background(), 1, "tok-p2", wise.SpendingPermissions{
		AllowOnlineTransactions: false,
	})
	if err != nil {
		t.Fatalf("UpdateSpendingPermissions: %v", err)
	}

	if got.AllowOnlineTransactions {
		t.Error("want AllowOnlineTransactions=false")
	}
}

// ---------------------------------------------------------------------------
// CardOrder service
// ---------------------------------------------------------------------------

func TestCardOrders_Create(t *testing.T) {
	want := wise.CardOrder{ID: "co-001", CardType: wise.CardTypeVirtual}

	rt := newRouter()
	rt.Handle("POST", "/v3/spend/profiles/1/card-orders", jsonResp(t, 201, want))

	c := newTestServer(t, rt)

	got, err := c.CardOrders.Create(context.Background(), 1, wise.CreateCardOrderRequest{
		ProfileID: 1,
		CardType:  wise.CardTypeVirtual,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != "co-001" {
		t.Errorf("want co-001, got %s", got.ID)
	}
}

func TestCardOrders_GetRequirements(t *testing.T) {
	want := []wise.CardOrderRequirement{
		{Type: "VERIFICATION", Status: "COMPLETED"},
		{Type: "ADDRESS", Status: "NEEDS_ACTION"},
	}

	rt := newRouter()
	rt.Handle("GET", "/v3/spend/profiles/1/card-orders/co-001/requirements", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.CardOrders.GetRequirements(context.Background(), 1, "co-001")
	if err != nil {
		t.Fatalf("GetRequirements: %v", err)
	}

	if len(got) != 2 || got[0].Type != "VERIFICATION" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestCardOrders_UpdateStatus(t *testing.T) {
	want := wise.CardOrder{ID: "co-002", Status: "CANCELED"}

	rt := newRouter()
	rt.Handle("PUT", "/v3/spend/profiles/1/card-orders/co-002/status", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.CardOrders.UpdateStatus(context.Background(), 1, "co-002", "CANCELED")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	if got.Status != "CANCELED" {
		t.Errorf("want CANCELED, got %s", got.Status)
	}
}

func TestCardOrders_ValidateAddress(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v3/spend/address/validate", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	c := newTestServer(t, rt)

	err := c.CardOrders.ValidateAddress(context.Background(), wise.Address{
		Country: "GB", City: "London", PostCode: "EC1A 1BB", FirstLine: "1 Main St",
	})
	if err != nil {
		t.Fatalf("ValidateAddress: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Spend limits
// ---------------------------------------------------------------------------

func TestSpendLimits_GetAndUpdateProfile(t *testing.T) {
	limit := 100.0
	want := wise.SpendLimits{Daily: &wise.SpendLimit{Value: &limit, Currency: "GBP"}}

	rt := newRouter()
	rt.Handle("GET", "/v1/spend/profiles/1/limits", jsonResp(t, 200, want))
	rt.Handle("PUT", "/v1/spend/profiles/1/limits", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.SpendLimits.GetProfileLimits(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetProfileLimits: %v", err)
	}

	if got.Daily == nil || *got.Daily.Value != 100.0 {
		t.Errorf("unexpected limits: %+v", got)
	}

	updated, err := c.SpendLimits.UpdateProfileLimits(context.Background(), 1, want)
	if err != nil {
		t.Fatalf("UpdateProfileLimits: %v", err)
	}

	if *updated.Daily.Value != 100.0 {
		t.Errorf("want 100, got %v", *updated.Daily.Value)
	}
}

// ---------------------------------------------------------------------------
// Disputes service
// ---------------------------------------------------------------------------

func TestDisputes_List(t *testing.T) {
	want := []wise.Dispute{
		{ID: "d1", SubStatus: wise.DisputeSubmitted},
		{ID: "d2", SubStatus: wise.DisputeInReview},
	}

	rt := newRouter()
	rt.Handle("GET", "/v3/spend/profiles/1/disputes", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Disputes.List(context.Background(), 1, wise.PageParams{Limit: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("want 2, got %d", len(got))
	}
}

func TestDisputes_Withdraw(t *testing.T) {
	rt := newRouter()
	rt.Handle("PUT", "/v3/spend/profiles/1/disputes/d4/status", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	c := newTestServer(t, rt)

	if err := c.Disputes.Withdraw(context.Background(), 1, "d4"); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Batch Group service
// ---------------------------------------------------------------------------

func TestBatches_Create(t *testing.T) {
	want := wise.BatchGroup{ID: "bg-001", Name: "Payroll", Status: wise.BatchGroupNew}

	rt := newRouter()
	rt.Handle("POST", "/v3/profiles/1/batch-groups", jsonResp(t, 201, want))

	c := newTestServer(t, rt)

	got, err := c.Batches.Create(context.Background(), 1, wise.CreateBatchGroupRequest{
		Name:           "Payroll",
		SourceCurrency: "GBP",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != "bg-001" {
		t.Errorf("want bg-001, got %s", got.ID)
	}
}

func TestBatches_Complete(t *testing.T) {
	want := wise.BatchGroup{ID: "bg-002", Status: wise.BatchGroupCompleted, Version: 2}

	rt := newRouter()
	rt.Handle("PATCH", "/v3/profiles/1/batch-groups/bg-002", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Batches.Complete(context.Background(), 1, "bg-002", 1)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if got.Status != wise.BatchGroupCompleted {
		t.Errorf("want COMPLETED, got %s", got.Status)
	}
}

func TestBatches_GetPaymentInitiation(t *testing.T) {
	want := wise.PaymentInitiation{ID: "pi-001", Status: "COMPLETED"}

	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/1/batch-groups/bg-001/payment-initiations/pi-001", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Batches.GetPaymentInitiation(context.Background(), 1, "bg-001", "pi-001")
	if err != nil {
		t.Fatalf("GetPaymentInitiation: %v", err)
	}

	if got.ID != "pi-001" {
		t.Errorf("want pi-001, got %s", got.ID)
	}
}

// ---------------------------------------------------------------------------
// Webhook service
// ---------------------------------------------------------------------------

func TestWebhooks_Create(t *testing.T) {
	want := wise.WebhookSubscription{ID: "sub-1", TriggerOn: wise.EventTransferStateChange}

	rt := newRouter()
	rt.Handle("POST", "/v3/profiles/1/subscriptions", jsonResp(t, 201, want))

	c := newTestServer(t, rt)

	got, err := c.Webhooks.Create(context.Background(), wise.CreateWebhookRequest{
		Name:      "hook",
		TriggerOn: wise.EventTransferStateChange,
		URL:       "https://example.com/hook",
		ProfileID: 1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.ID != "sub-1" {
		t.Errorf("want sub-1, got %s", got.ID)
	}
}

func TestWebhooks_Delete(t *testing.T) {
	rt := newRouter()
	rt.Handle("DELETE", "/v3/profiles/1/subscriptions/sub-abc", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))

	c := newTestServer(t, rt)

	if err := c.Webhooks.Delete(context.Background(), 1, "sub-abc"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Webhook parsing & signature verification
// ---------------------------------------------------------------------------

func TestParseEvent_Valid(t *testing.T) {
	payload := `{"data":{"resource":{"id":123}},"subscriptionId":"s","eventType":"transfers#state-change","schemaVersion":"2.0.0","sentAt":"2024-01-15T10:30:00Z"}`

	ev, err := wise.ParseEvent([]byte(payload))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	if ev.EventType != wise.EventTransferStateChange {
		t.Errorf("wrong event type: %s", ev.EventType)
	}

	if ev.SubscriptionID != "s" {
		t.Errorf("wrong subscription ID: %s", ev.SubscriptionID)
	}
}

func TestParseEvent_UnmarshalData(t *testing.T) {
	payload := `{"eventType":"transfers#state-change","subscriptionId":"s","schemaVersion":"2.0.0","sentAt":"2024-01-15T10:30:00Z","data":{"currentState":"outgoing_payment_sent","resource":{"id":77}}}`

	ev, err := wise.ParseEvent([]byte(payload))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}

	var tsc wise.TransferStateChangeEvent
	if err := ev.UnmarshalData(&tsc); err != nil {
		t.Fatalf("UnmarshalData: %v", err)
	}

	if tsc.CurrentState != "outgoing_payment_sent" {
		t.Errorf("want outgoing_payment_sent, got %s", tsc.CurrentState)
	}

	if tsc.Resource.ID != 77 {
		t.Errorf("want 77, got %d", tsc.Resource.ID)
	}
}

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	secret := "my-secret"
	body := []byte(`{"eventType":"transfers#state-change"}`)
	sig := hmacSHA256(secret, body)

	if err := wise.VerifyWebhookSignature(body, sig, secret); err != nil {
		t.Errorf("valid signature rejected: %v", err)
	}
}

func TestVerifyWebhookSignature_WithSHA256Prefix(t *testing.T) {
	secret := "s"
	body := []byte(`{}`)
	sig := "sha256=" + hmacSHA256(secret, body)

	if err := wise.VerifyWebhookSignature(body, sig, secret); err != nil {
		t.Errorf("signature with sha256= prefix rejected: %v", err)
	}
}

func TestVerifyWebhookSignature_Invalid(t *testing.T) {
	err := wise.VerifyWebhookSignature([]byte(`{}`), "sha256=badhex", "secret")
	if !errors.Is(err, wise.ErrInvalidWebhookSignature) {
		t.Errorf("want ErrInvalidWebhookSignature, got %v", err)
	}
}

func TestVerifyWebhookSignature_EmptyHeader(t *testing.T) {
	err := wise.VerifyWebhookSignature([]byte(`{}`), "", "secret")
	if !errors.Is(err, wise.ErrInvalidWebhookSignature) {
		t.Errorf("want ErrInvalidWebhookSignature for empty header, got %v", err)
	}
}

func TestEventRouter_Dispatch(t *testing.T) {
	var (
		mu     sync.Mutex
		called bool
	)

	router := wise.NewEventRouter("")
	router.On(wise.EventTransferStateChange, func(e *wise.WebhookEvent) error {
		mu.Lock()
		called = true
		mu.Unlock()

		return nil
	})

	payload := `{"eventType":"transfers#state-change","subscriptionId":"x","schemaVersion":"2.0.0","sentAt":"2024-01-15T10:30:00Z","data":{}}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(payload))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	mu.Lock()
	defer mu.Unlock()

	if !called {
		t.Error("handler was not called")
	}
}

func TestEventRouter_ValidSignature(t *testing.T) {
	secret := "real-secret"
	payload := []byte(`{"eventType":"transfers#state-change","subscriptionId":"x","schemaVersion":"2.0.0","sentAt":"2024-01-15T10:30:00Z","data":{}}`)
	sig := "sha256=" + hmacSHA256(secret, payload)

	called := false
	router := wise.NewEventRouter(secret)
	router.On(wise.EventTransferStateChange, func(_ *wise.WebhookEvent) error {
		called = true
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(string(payload)))
	req.Header.Set("X-Signature-SHA256", sig)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	if !called {
		t.Error("handler not called with valid signature")
	}
}

func TestEventRouter_BadSignature(t *testing.T) {
	router := wise.NewEventRouter("real-secret")
	payload := `{"eventType":"transfers#state-change","subscriptionId":"x","schemaVersion":"2.0.0","sentAt":"2024-01-15T10:30:00Z","data":{}}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(payload))
	req.Header.Set("X-Signature-SHA256", "sha256=badsig")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestEventRouter_MethodNotAllowed(t *testing.T) {
	router := wise.NewEventRouter("")
	req := httptest.NewRequest(http.MethodGet, "/webhooks", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestAPIError_404_IsNotFound(t *testing.T) {
	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/999", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"}) //nolint:errcheck
	}))

	c := newTestServer(t, rt)

	_, err := c.Profiles.Get(context.Background(), 999)
	if !wise.IsNotFound(err) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestAPIError_401_IsUnauthorized(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"}) //nolint:errcheck
	})

	c := newTestServer(t, h)

	_, err := c.Profiles.List(context.Background())
	if !wise.IsUnauthorized(err) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAPIError_403_SCA_IsSCARequired(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"code": "SCA_REQUIRED", "message": "sca"}) //nolint:errcheck
	})

	c := newTestServer(t, h)

	_, err := c.Profiles.List(context.Background())
	if !wise.IsSCARequired(err) {
		t.Errorf("want IsSCARequired, got %v", err)
	}
}

func TestAPIError_429_IsRateLimited(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]string{"message": "slow down"}) //nolint:errcheck
	})

	c := newTestServer(t, h)

	_, err := c.Currencies.List(context.Background())
	if !wise.IsRateLimited(err) {
		t.Errorf("want ErrRateLimited, got %v", err)
	}
}

func TestAPIError_500_IsServerError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"message": "oops"}) //nolint:errcheck
	})

	c := newTestServer(t, h)

	_, err := c.Currencies.List(context.Background())
	if !wise.IsServerError(err) {
		t.Errorf("want ErrServerError, got %v", err)
	}
}

func TestAPIError_422_FieldErrors(t *testing.T) {
	body := map[string]any{
		"message": "validation failed",
		"errors":  []map[string]string{{"field": "sourceCurrency", "code": "INVALID", "message": "bad"}},
	}

	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(body) //nolint:errcheck
	})

	c := newTestServer(t, h)

	_, err := c.Currencies.List(context.Background())

	fields := wise.FieldErrors(err)
	if len(fields) != 1 || fields[0].Field != "sourceCurrency" {
		t.Errorf("unexpected field errors: %+v", fields)
	}
}

func TestAPIError_MessageInError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"message": "something went wrong"}) //nolint:errcheck
	})

	c := newTestServer(t, h)

	_, err := c.Currencies.List(context.Background())
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("want message in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Mock error helpers
// ---------------------------------------------------------------------------

func TestMockAPIError_SCA(t *testing.T) {
	err := wise.MockAPIError(403, "SCA_REQUIRED", "auth required")
	if !wise.IsSCARequired(err) {
		t.Errorf("want IsSCARequired, got %v", err)
	}
}

func TestMockNotFoundError(t *testing.T) {
	err := wise.MockNotFoundError("profile not found")
	if !wise.IsNotFound(err) {
		t.Errorf("want IsNotFound, got %v", err)
	}
}

func TestMockValidationError(t *testing.T) {
	err := wise.MockValidationError(
		wise.MockFieldError("currency", "INVALID", "bad"),
		wise.MockFieldError("amount", "POSITIVE", "must be >0"),
	)
	fields := wise.FieldErrors(err)

	if len(fields) != 2 || fields[0].Field != "currency" {
		t.Errorf("unexpected field errors: %+v", fields)
	}
}

// ---------------------------------------------------------------------------
// Retry transport
// ---------------------------------------------------------------------------

func TestRetry_RetriesOn429(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(429)
			return
		}

		json.NewEncoder(w).Encode([]wise.Currency{{Code: "USD"}}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(3),
		wise.WithTimeout(10*time.Second),
	)

	got, err := c.Currencies.List(context.Background())
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("want 1, got %d", len(got))
	}

	if atomic.LoadInt32(&calls) < 3 {
		t.Errorf("expected >=3 calls, got %d", atomic.LoadInt32(&calls))
	}
}

func TestRetry_NoRetryOn400(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "bad"}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(3),
	)

	_, _ = c.Currencies.List(context.Background())

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("want 1 call for 400, got %d", atomic.LoadInt32(&calls))
	}
}

func TestRetry_RetriesOn503(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 2 {
			w.WriteHeader(503)
			return
		}

		json.NewEncoder(w).Encode([]wise.Profile{{ID: 1}}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(3),
		wise.WithTimeout(10*time.Second),
	)

	_, err := c.Profiles.List(context.Background())
	if err != nil {
		t.Fatalf("expected success after 503 retry: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Circuit breaker
// ---------------------------------------------------------------------------

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	var serverCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&serverCalls, 1)
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"message": "error"}) //nolint:errcheck
	}))
	defer srv.Close()

	cb := wise.NewCircuitBreaker(wise.CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          100 * time.Millisecond,
	})

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
		wise.WithCircuitBreaker(cb),
	)

	for range 3 {
		_, _ = c.Currencies.List(context.Background())
	}

	if cb.State() != wise.CircuitOpen {
		t.Errorf("want CircuitOpen, got %s", cb.State())
	}

	// Fourth call should be rejected by the circuit breaker.
	_, err := c.Currencies.List(context.Background())
	if err == nil {
		t.Fatal("expected error from open circuit")
	}

	// Server should NOT have been called a 4th time.
	if atomic.LoadInt32(&serverCalls) != 3 {
		t.Errorf("want exactly 3 server calls, got %d", atomic.LoadInt32(&serverCalls))
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"message": "error"}) //nolint:errcheck

			return
		}

		json.NewEncoder(w).Encode([]wise.Currency{{Code: "USD"}}) //nolint:errcheck
	}))
	defer srv.Close()

	cb := wise.NewCircuitBreaker(wise.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
		wise.WithCircuitBreaker(cb),
	)

	_, _ = c.Currencies.List(context.Background())
	_, _ = c.Currencies.List(context.Background())

	if cb.State() != wise.CircuitOpen {
		t.Fatal("want CircuitOpen")
	}

	time.Sleep(60 * time.Millisecond)

	if cb.State() != wise.CircuitHalfOpen {
		t.Errorf("want CircuitHalfOpen after timeout, got %s", cb.State())
	}

	_, err := c.Currencies.List(context.Background())
	if err != nil {
		t.Fatalf("expected success in HALF_OPEN: %v", err)
	}

	if cb.State() != wise.CircuitClosed {
		t.Errorf("want CircuitClosed after success, got %s", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	cb := wise.NewCircuitBreaker(wise.CircuitBreakerConfig{FailureThreshold: 1})

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
		wise.WithCircuitBreaker(cb),
	)

	_, _ = c.Currencies.List(context.Background())

	if cb.State() != wise.CircuitOpen {
		t.Fatal("want CircuitOpen")
	}

	cb.Reset()

	if cb.State() != wise.CircuitClosed {
		t.Errorf("want CircuitClosed after Reset, got %s", cb.State())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	cases := []struct {
		s    wise.CircuitState
		want string
	}{
		{wise.CircuitClosed, "CLOSED"},
		{wise.CircuitOpen, "OPEN"},
		{wise.CircuitHalfOpen, "HALF_OPEN"},
	}

	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("want %s, got %s", tc.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Middleware hooks
// ---------------------------------------------------------------------------

func TestRequestHook_Called(t *testing.T) {
	var called int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithRequestHook(func(_ context.Context, _ *http.Request) error {
			atomic.AddInt32(&called, 1)
			return nil
		}),
	)

	_, _ = c.Currencies.List(context.Background())

	if atomic.LoadInt32(&called) == 0 {
		t.Error("request hook was not called")
	}
}

func TestRequestHook_Abort(t *testing.T) {
	var serverCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&serverCalls, 1)
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
		wise.WithRequestHook(func(_ context.Context, _ *http.Request) error {
			return fmt.Errorf("blocked by hook")
		}),
	)

	_, err := c.Currencies.List(context.Background())
	if err == nil {
		t.Fatal("expected error from hook")
	}

	if atomic.LoadInt32(&serverCalls) != 0 {
		t.Error("server should not have been called when hook aborts")
	}
}

func TestResponseHook_ReceivesStatusCode(t *testing.T) {
	var lastCode int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithResponseHook(func(_ context.Context, _ *http.Request, code int, _ time.Duration, _ error) {
			atomic.StoreInt32(&lastCode, int32(code))
		}),
	)

	_, _ = c.Currencies.List(context.Background())

	if atomic.LoadInt32(&lastCode) != 200 {
		t.Errorf("want 200, got %d", atomic.LoadInt32(&lastCode))
	}
}

func TestMultipleRequestHooks_CalledInOrder(t *testing.T) {
	var order []int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithRequestHook(func(_ context.Context, _ *http.Request) error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()

			return nil
		}),
		wise.WithRequestHook(func(_ context.Context, _ *http.Request) error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()

			return nil
		}),
		wise.WithRequestHook(func(_ context.Context, _ *http.Request) error {
			mu.Lock()
			order = append(order, 3)
			mu.Unlock()

			return nil
		}),
	)

	_, _ = c.Currencies.List(context.Background())

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("hooks not called in order: %v", order)
	}
}

// ---------------------------------------------------------------------------
// Idempotency & context helpers
// ---------------------------------------------------------------------------

func TestNewIdempotencyKey_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)

	for range 1000 {
		k := wise.NewIdempotencyKey()

		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate idempotency key")
		}

		seen[k] = struct{}{}

		if len(k) != 32 {
			t.Errorf("want 32-char key, got %d: %s", len(k), k)
		}
	}
}

func TestWithIdempotencyKey_AttachedAsHeader(t *testing.T) {
	var gotHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, _ := wise.New(wise.WithPersonalToken("tok"), wise.WithBaseURL(srv.URL))

	ctx := wise.WithIdempotencyKey(context.Background(), "my-idem-key")
	_ = c.Balances.Close(ctx, 1, 1)

	if gotHeader != "my-idem-key" {
		t.Errorf("want my-idem-key, got %q", gotHeader)
	}
}

func TestWithRequestID_AttachedAsHeader(t *testing.T) {
	var gotHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Request-Id")
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(wise.WithPersonalToken("tok"), wise.WithBaseURL(srv.URL))

	ctx := wise.WithRequestID(context.Background(), "req-abc-123")
	_, _ = c.Currencies.List(ctx)

	if gotHeader != "req-abc-123" {
		t.Errorf("want req-abc-123, got %q", gotHeader)
	}
}

// ---------------------------------------------------------------------------
// Generic iterator
// ---------------------------------------------------------------------------

func TestIter_PagesThrough(t *testing.T) {
	all := []string{"a", "b", "c", "d", "e"}

	iter := wise.NewIter(func(p wise.PageParams) ([]string, bool, error) {
		start := p.Offset
		if start >= len(all) {
			return nil, false, nil
		}

		end := start + 2
		if end > len(all) {
			end = len(all)
		}

		return all[start:end], end < len(all), nil
	})

	var got []string

	for iter.Next() {
		got = append(got, iter.Item())
	}

	if err := iter.Err(); err != nil {
		t.Fatalf("unexpected iter error: %v", err)
	}

	if len(got) != 5 {
		t.Errorf("want 5, got %d: %v", len(got), got)
	}
}

func TestIter_StopsOnError(t *testing.T) {
	calls := 0

	iter := wise.NewIter(func(_ wise.PageParams) ([]string, bool, error) {
		calls++
		if calls == 2 {
			return nil, false, fmt.Errorf("page 2 failed")
		}

		return []string{"x", "y"}, true, nil
	})

	var got []string

	for iter.Next() {
		got = append(got, iter.Item())
	}

	if iter.Err() == nil {
		t.Fatal("expected error from iterator")
	}

	if len(got) != 2 {
		t.Errorf("want 2 items before error, got %d", len(got))
	}
}

func TestIter_EmptyFirstPage(t *testing.T) {
	iter := wise.NewIter(func(_ wise.PageParams) ([]string, bool, error) {
		return nil, false, nil
	})

	if iter.Next() {
		t.Error("expected false for empty first page")
	}

	if iter.Err() != nil {
		t.Errorf("unexpected error: %v", iter.Err())
	}
}

// ---------------------------------------------------------------------------
// Ptr helper
// ---------------------------------------------------------------------------

func TestPtr_Float64(t *testing.T) {
	p := wise.Ptr(42.5)
	if p == nil || *p != 42.5 {
		t.Errorf("unexpected: %v", p)
	}
}

func TestPtr_String(t *testing.T) {
	p := wise.Ptr("hello")
	if *p != "hello" {
		t.Errorf("want hello, got %s", *p)
	}
}

func TestPtr_Int(t *testing.T) {
	p := wise.Ptr(99)
	if *p != 99 {
		t.Errorf("want 99, got %d", *p)
	}
}

// ---------------------------------------------------------------------------
// Time marshaling.
// ---------------------------------------------------------------------------

func TestTime_UnmarshalFormats(t *testing.T) {
	cases := []struct {
		input    string
		wantZero bool
	}{
		{`"2024-01-15T10:30:00Z"`, false},
		{`"2024-01-15T10:30:00.000Z"`, false},
		{`"2024-01-15"`, false},
		{`""`, true},
		{`"null"`, true},
	}

	for _, tc := range cases {
		var ts wise.Time

		if err := json.Unmarshal([]byte(tc.input), &ts); err != nil {
			t.Errorf("input %s: %v", tc.input, err)
			continue
		}

		if tc.wantZero && !ts.IsZero() {
			t.Errorf("input %s: expected zero time", tc.input)
		}

		if !tc.wantZero && ts.IsZero() {
			t.Errorf("input %s: expected non-zero time", tc.input)
		}
	}
}

func TestTime_MarshalZeroIsNull(t *testing.T) {
	var ts wise.Time

	b, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if string(b) != "null" {
		t.Errorf("want null, got %s", b)
	}
}

// ---------------------------------------------------------------------------
// Amount stringer
// ---------------------------------------------------------------------------

func TestAmount_String(t *testing.T) {
	a := wise.Amount{Value: 123.456, Currency: "USD"}
	s := a.String()

	if !strings.Contains(s, "USD") || !strings.Contains(s, "123.4560") {
		t.Errorf("unexpected Amount.String(): %s", s)
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety
// ---------------------------------------------------------------------------

func TestClient_ConcurrentSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]wise.Profile{{ID: 1}}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(wise.WithPersonalToken("tok"), wise.WithBaseURL(srv.URL))

	const goroutines = 50

	var wg sync.WaitGroup

	errs := make(chan error, goroutines)

	for range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if _, err := c.Profiles.List(context.Background()); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestClient_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode([]wise.Currency{}) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.Currencies.List(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

// ---------------------------------------------------------------------------
// SCA service
// ---------------------------------------------------------------------------

func TestSCA_IsPassed_AllPassed(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	status := &wise.SCAStatus{Challenges: []wise.SCAChallenge{
		{Required: true, Passed: true},
		{Required: false, Passed: false}, // optional — should be ignored
	}}

	if !c.SCA.IsPassed(status) {
		t.Error("want IsPassed=true when all required are passed")
	}
}

func TestSCA_IsPassed_NotAllPassed(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	status := &wise.SCAStatus{Challenges: []wise.SCAChallenge{
		{Required: true, Passed: true},
		{Required: true, Passed: false},
	}}

	if c.SCA.IsPassed(status) {
		t.Error("want IsPassed=false when required challenge is pending")
	}
}

func TestSCA_IsPassed_NilStatus(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	if c.SCA.IsPassed(nil) {
		t.Error("want IsPassed=false for nil status")
	}
}

func TestSCA_PendingChallenges(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	status := &wise.SCAStatus{Challenges: []wise.SCAChallenge{
		{Type: "PIN", Required: true, Passed: true},
		{Type: "SMS", Required: true, Passed: false},
		{Type: "FACE_MAP", Required: false, Passed: false},
	}}

	pending := c.SCA.PendingChallenges(status)
	if len(pending) != 1 || pending[0].Type != "SMS" {
		t.Errorf("unexpected pending challenges: %+v", pending)
	}
}

// ---------------------------------------------------------------------------
// OTT service
// ---------------------------------------------------------------------------

func TestOTT_IsPassed(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	cases := []struct {
		name string
		s    *wise.OTTStatus
		want bool
	}{
		{"nil", nil, false},
		{"all passed", &wise.OTTStatus{Challenges: []wise.OTTChallenge{{Required: true, Passed: true}}}, true},
		{"pending", &wise.OTTStatus{Challenges: []wise.OTTChallenge{{Required: true, Passed: false}}}, false},
	}

	for _, tc := range cases {
		if got := c.OTT.IsPassed(tc.s); got != tc.want {
			t.Errorf("%s: want %v, got %v", tc.name, tc.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// 3DS service
// ---------------------------------------------------------------------------

func TestThreeDS_InformChallengeResult_Accepted(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v3/spend/profiles/1/3dsecure/challenge-result", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body wise.InformChallengeResultRequest
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

		if body.Result != wise.ThreeDSChallengeAccepted {
			http.Error(w, "wrong result", 400)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	c := newTestServer(t, rt)

	err := c.ThreeDS.InformChallengeResult(context.Background(), 1, wise.InformChallengeResultRequest{
		ChallengeID: "ch-1",
		Result:      wise.ThreeDSChallengeAccepted,
	})
	if err != nil {
		t.Fatalf("InformChallengeResult: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Contact service
// ---------------------------------------------------------------------------

func TestContacts_Find_ByWiseTag(t *testing.T) {
	want := wise.Contact{ID: 77, Name: "Alice Smith", WiseTag: "alice"}

	rt := newRouter()
	rt.Handle("POST", "/v2/profiles/1/contacts", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.Contacts.Find(context.Background(), 1, wise.FindContactRequest{WiseTag: "alice"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if got.ID != 77 {
		t.Errorf("want 77, got %d", got.ID)
	}
}

func TestContacts_Find_NotFound(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v2/profiles/1/contacts", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"}) //nolint:errcheck
	}))

	c := newTestServer(t, rt)

	_, err := c.Contacts.Find(context.Background(), 1, wise.FindContactRequest{WiseTag: "nobody"})
	if !wise.IsNotFound(err) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FaceTec service
// ---------------------------------------------------------------------------

func TestFaceTec_GetPublicKey(t *testing.T) {
	want := wise.FaceTecPublicKey{Key: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"}

	rt := newRouter()
	rt.Handle("GET", "/v1/facetec/public-key", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.FaceTec.GetPublicKey(context.Background())
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}

	if !strings.Contains(got.Key, "BEGIN PUBLIC KEY") {
		t.Errorf("unexpected key: %s", got.Key)
	}
}

// ---------------------------------------------------------------------------
// JOSE service
// ---------------------------------------------------------------------------

func TestJOSE_GetResponsePublicKeys(t *testing.T) {
	want := wise.JOSEPublicKeySet{Keys: []wise.JOSEPublicKey{
		{KID: "key-1", Kty: "RSA", Use: "sig"},
	}}

	rt := newRouter()
	rt.Handle("GET", "/v1/auth/jose/response/public-keys", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.JOSE.GetResponsePublicKeys(context.Background())
	if err != nil {
		t.Fatalf("GetResponsePublicKeys: %v", err)
	}

	if len(got.Keys) != 1 || got.Keys[0].KID != "key-1" {
		t.Errorf("unexpected keys: %+v", got)
	}
}

func TestJOSE_PlaygroundVerifyJWS(t *testing.T) {
	want := wise.JOSEPlaygroundResult{Verified: true}

	rt := newRouter()
	rt.Handle("POST", "/v1/auth/jose/playground/jws", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.JOSE.PlaygroundVerifyJWS(context.Background(), "eyJhbGc...")
	if err != nil {
		t.Fatalf("PlaygroundVerifyJWS: %v", err)
	}

	if !got.Verified {
		t.Error("want Verified=true")
	}
}

// ---------------------------------------------------------------------------
// ClaimAccount service
// ---------------------------------------------------------------------------

func TestClaimAccount_GenerateCode(t *testing.T) {
	want := wise.ClaimAccountCode{Code: "claim-abc-123"}

	rt := newRouter()
	rt.Handle("POST", "/v1/user/claim-account", jsonResp(t, 200, want))

	c := newTestServer(t, rt)

	got, err := c.ClaimAccount.GenerateCode(context.Background(), 42)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	if got.Code != "claim-abc-123" {
		t.Errorf("want claim-abc-123, got %s", got.Code)
	}
}

// ---------------------------------------------------------------------------
// DoRaw escape hatch
// ---------------------------------------------------------------------------

func TestDoRaw_GetCurrencies(t *testing.T) {
	rt := newRouter()
	rt.Handle("GET", "/v1/currencies", jsonResp(t, 200, []wise.Currency{{Code: "USD"}}))

	c := newTestServer(t, rt)

	resp, err := c.DoRaw(context.Background(), wise.RawRequest{
		Method: "GET",
		Path:   "/v1/currencies",
	})
	if err != nil {
		t.Fatalf("DoRaw: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	if len(resp.Body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestDoRaw_ErrorPassthrough(t *testing.T) {
	rt := newRouter()
	rt.Handle("GET", "/v1/missing", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"}) //nolint:errcheck
	}))

	c := newTestServer(t, rt)

	resp, err := c.DoRaw(context.Background(), wise.RawRequest{Method: "GET", Path: "/v1/missing"})
	if err == nil {
		t.Fatal("expected error")
	}

	if resp == nil {
		t.Fatal("expected raw response even on error")
		return //nolint:govet // unreachable — satisfies staticcheck SA5011 nil-check analysis
	}

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Ping
// ---------------------------------------------------------------------------

func TestPing_Success(t *testing.T) {
	c := newTestServer(t, jsonResp(t, 200, []wise.Currency{{Code: "USD"}}))

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPing_Failure(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	})

	c := newTestServer(t, h)

	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected Ping to fail on 503")
	}
}

// ---------------------------------------------------------------------------
// API completeness: all 42 Wise API groups must have a service
// ---------------------------------------------------------------------------

func TestAPICompleteness_AllGroupsCovered(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))

	groups := map[string]bool{
		"3D Secure":                        c.ThreeDS != nil,
		"Activity":                         c.Activities != nil,
		"Additional Customer Verification": c.KYC != nil,
		"Address":                          c.Addresses != nil,
		"Balance":                          c.Balances != nil,
		"Balance Statement":                c.Statements != nil,
		"Bank Account Details":             c.BankAccounts != nil,
		"Batch Group":                      c.Batches != nil,
		"Bulk Settlement":                  c.Batches != nil,
		"Card":                             c.Cards != nil,
		"Card Kiosk Collection":            c.KioskCollection != nil,
		"Card Order":                       c.CardOrders != nil,
		"Card Transaction":                 c.CardTxns != nil,
		"Claim Account":                    c.ClaimAccount != nil,
		"Client Credentials Token":         c.OAuthSvc != nil,
		"Comparison":                       c.Comparisons != nil,
		"Contact":                          c.Contacts != nil,
		"Currencies":                       c.Currencies != nil,
		"Delivery Estimate":                c.Transfers != nil,
		"Direct Debit Account":             c.DirectDebits != nil,
		"Disputes":                         c.Disputes != nil,
		"FaceTec":                          c.FaceTec != nil,
		"JOSE":                             c.JOSE != nil,
		"Multi Currency Account":           c.MultiCurrencyAccount != nil,
		"One Time Token":                   c.OTT != nil,
		"Partner Cases":                    c.Cases != nil,
		"Payin Deposit Detail":             c.Transfers != nil,
		"KYC Review":                       c.KYCReview != nil,
		"Profile":                          c.Profiles != nil,
		"Push Provisioning":                c.PushProvisioning != nil,
		"Quote":                            c.Quotes != nil,
		"Rate":                             c.Rates != nil,
		"Recipient Account":                c.Recipients != nil,
		"Simulation":                       c.Simulations != nil,
		"Spend Controls":                   c.SpendControls != nil,
		"Spend Limits":                     c.SpendLimits != nil,
		"Strong Customer Authentication":   c.SCA != nil,
		"Transfer":                         c.Transfers != nil,
		"User":                             c.Users != nil,
		"User Security":                    c.UserSecurity != nil,
		"User Tokens":                      c.OAuthSvc != nil,
		"Webhook":                          c.Webhooks != nil,
	}

	for group, covered := range groups {
		if !covered {
			t.Errorf("API group %q is not covered by any service", group)
		}
	}
}

// ---------------------------------------------------------------------------
// New endpoints added in gap-analysis pass
// ---------------------------------------------------------------------------

// Address: POST /v1/address-requirements (refresh with context).
func TestAddresses_RefreshRequirements(t *testing.T) {
	want := []wise.RequirementField{{Name: "state"}}
	rt := newRouter()
	rt.Handle("POST", "/v1/address-requirements", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.Addresses.RefreshRequirements(context.Background(), map[string]any{
		"details": map[string]string{"country": "US"},
	})
	if err != nil {
		t.Fatalf("RefreshRequirements: %v", err)
	}
	if len(got) != 1 || got[0].Name != "state" {
		t.Errorf("unexpected: %+v", got)
	}
}

// BankAccount: POST /v3/profiles/{id}/bank-details.
func TestBankAccounts_CreateMultipleBankDetails(t *testing.T) {
	want := []wise.BankAccountDetail{{Currency: "GBP", Type: "SORT_CODE"}}
	rt := newRouter()
	rt.Handle("POST", "/v3/profiles/1/bank-details", jsonResp(t, 201, want))
	c := newTestServer(t, rt)
	got, err := c.BankAccounts.CreateMultipleBankDetails(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("CreateMultipleBankDetails: %v", err)
	}
	if len(got) != 1 || got[0].Currency != "GBP" {
		t.Errorf("unexpected: %+v", got)
	}
}

// KYC: POST /v3/.../verification-status/upload-document.
func TestKYC_UploadDocument(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v3/profiles/1/verification-status/upload-document",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
	c := newTestServer(t, rt)
	err := c.KYC.UploadDocument(context.Background(), 1, []map[string]any{
		{"type": "PASSPORT", "content": "base64data"},
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
}

// Card: PATCH /v3/.../spending-permissions (single permission v3).
func TestCards_UpdateSinglePermission(t *testing.T) {
	want := wise.SpendingPermissions{AllowCashWithdrawals: false}
	rt := newRouter()
	rt.Handle("PATCH", "/v3/spend/profiles/1/cards/tok/spending-permissions", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.Cards.UpdateSinglePermission(context.Background(), 1, "tok", "CASH_WITHDRAWALS", false)
	if err != nil {
		t.Fatalf("UpdateSinglePermission: %v", err)
	}
	if got.AllowCashWithdrawals {
		t.Error("want AllowCashWithdrawals=false")
	}
}

// Dispute: DynamicFlowEntry POST /v3/.../dispute-form/flows/step/{scheme}/{reason}.
func TestDisputes_DynamicFlowEntry(t *testing.T) {
	want := map[string]any{"type": "form", "fields": []any{}}
	rt := newRouter()
	rt.Handle("POST", "/v3/spend/profiles/1/dispute-form/flows/step/VISA/FRAUD", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.Disputes.DynamicFlowEntry(context.Background(), 1, "VISA", "FRAUD", "txn-123")
	if err != nil {
		t.Fatalf("DynamicFlowEntry: %v", err)
	}
	if got["type"] != "form" {
		t.Errorf("want type=form, got %v", got["type"])
	}
}

// Dispute: Submit POST /v3/.../dispute-form/flows/{scheme}/{reason}.
func TestDisputes_Submit(t *testing.T) {
	want := wise.Dispute{ID: "d-submit-001", SubStatus: wise.DisputeSubmitted}
	rt := newRouter()
	rt.Handle("POST", "/v3/spend/profiles/1/dispute-form/flows/VISA/FRAUD", jsonResp(t, 201, want))
	c := newTestServer(t, rt)
	got, err := c.Disputes.Submit(context.Background(), 1, "VISA", "FRAUD", map[string]any{
		"transactionId": "txn-abc",
		"form":          map[string]string{"description": "I did not make this purchase"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.ID != "d-submit-001" {
		t.Errorf("want d-submit-001, got %s", got.ID)
	}
	if got.SubStatus != wise.DisputeSubmitted {
		t.Errorf("want SUBMITTED, got %s", got.SubStatus)
	}
}

// Dispute: UploadFile POST /v4/.../dispute-form/file.
func TestDisputes_UploadFile(t *testing.T) {
	want := wise.DisputeFile{FileID: "file-xyz"}
	rt := newRouter()
	rt.Handle("POST", "/v4/spend/profiles/1/dispute-form/file", jsonResp(t, 201, want))
	c := newTestServer(t, rt)
	got, err := c.Disputes.UploadFile(context.Background(), 1, "receipt.pdf", []byte("%PDF-1.4"), "application/pdf")
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if got.FileID != "file-xyz" {
		t.Errorf("want file-xyz, got %s", got.FileID)
	}
}

// ---------------------------------------------------------------------------
// KYC Review service — all 6 endpoints
// ---------------------------------------------------------------------------

func TestKYCReview_Create(t *testing.T) {
	want := wise.KYCReview{
		ID:        "kr-001",
		ProfileID: 1,
		Status:    wise.KYCReviewWaitingCustomerInput,
	}
	rt := newRouter()
	rt.Handle("POST", "/v1/profiles/1/kyc-reviews", jsonResp(t, 201, want))
	c := newTestServer(t, rt)
	got, err := c.KYCReview.Create(context.Background(), 1, wise.CreateKYCReviewRequest{ProfileID: 1, Action: "ONBOARDING"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "kr-001" {
		t.Errorf("want kr-001, got %s", got.ID)
	}
	if got.Status != wise.KYCReviewWaitingCustomerInput {
		t.Errorf("want WAITING_CUSTOMER_INPUT, got %s", got.Status)
	}
}

func TestKYCReview_List(t *testing.T) {
	want := []wise.KYCReview{
		{ID: "kr-001", Status: wise.KYCReviewWaitingCustomerInput},
		{ID: "kr-002", Status: wise.KYCReviewApproved},
	}
	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/1/kyc-reviews", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.KYCReview.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2, got %d", len(got))
	}
}

func TestKYCReview_UpdateRedirectURL(t *testing.T) {
	want := wise.KYCReview{
		ID:     "kr-001",
		Status: wise.KYCReviewWaitingCustomerInput,
		Link:   "https://wise.com/hosted-kyc?token=abc123",
	}
	rt := newRouter()
	rt.Handle("PATCH", "/v1/profiles/1/kyc-reviews/kr-001", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.KYCReview.UpdateRedirectURL(context.Background(), 1, "kr-001", "https://yourapp.com/kyc-complete")
	if err != nil {
		t.Fatalf("UpdateRedirectURL: %v", err)
	}
	if got.Link == "" {
		t.Error("expected non-empty Link")
	}
	if got.Link != "https://wise.com/hosted-kyc?token=abc123" {
		t.Errorf("unexpected link: %s", got.Link)
	}
}

func TestKYCReview_GetByID(t *testing.T) {
	want := wise.KYCReview{
		ID:     "kr-002",
		Status: wise.KYCReviewApproved,
		Requirements: []wise.KYCRequirement{
			{Key: "PROOF_OF_IDENTITY", State: wise.KYCRequirementVerified, APICollectionSupported: false},
		},
	}
	rt := newRouter()
	rt.Handle("GET", "/v2/profiles/1/kyc-reviews/kr-002", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.KYCReview.GetByID(context.Background(), 1, "kr-002")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != "kr-002" {
		t.Errorf("want kr-002, got %s", got.ID)
	}
	if got.Status != wise.KYCReviewApproved {
		t.Errorf("want APPROVED, got %s", got.Status)
	}
	if len(got.Requirements) != 1 || got.Requirements[0].State != wise.KYCRequirementVerified {
		t.Errorf("unexpected requirements: %+v", got.Requirements)
	}
}

func TestKYCReview_GetByIDV1_Deprecated(t *testing.T) {
	want := wise.KYCReview{ID: "kr-003", Status: wise.KYCReviewPending}
	rt := newRouter()
	rt.Handle("GET", "/v1/profiles/1/kyc-reviews/kr-003", jsonResp(t, 200, want))
	c := newTestServer(t, rt)
	got, err := c.KYCReview.GetByIDV1(context.Background(), 1, "kr-003")
	if err != nil {
		t.Fatalf("GetByIDV1: %v", err)
	}
	if got.ID != "kr-003" {
		t.Errorf("want kr-003, got %s", got.ID)
	}
}

func TestKYCReview_SubmitRequirement(t *testing.T) {
	rt := newRouter()
	rt.Handle("POST", "/v2/profiles/1/kyc-requirements/PROOF_OF_IDENTITY",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
	c := newTestServer(t, rt)
	err := c.KYCReview.SubmitRequirement(context.Background(), 1, "PROOF_OF_IDENTITY", map[string]any{
		"documentType":   "PASSPORT",
		"documentNumber": "AB123456",
		"expiryDate":     "2030-01-01",
	})
	if err != nil {
		t.Fatalf("SubmitRequirement: %v", err)
	}
}

// Test all KYCReview status constants are properly typed.
func TestKYCReview_StatusConstants(t *testing.T) {
	statuses := []wise.KYCReviewStatus{
		wise.KYCReviewWaitingCustomerInput,
		wise.KYCReviewPending,
		wise.KYCReviewApproved,
		wise.KYCReviewRejected,
		wise.KYCReviewExpired,
	}
	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("empty status constant: %v", s)
		}
	}

	reqStates := []wise.KYCRequirementState{
		wise.KYCRequirementNotProvided,
		wise.KYCRequirementProvided,
		wise.KYCRequirementVerified,
		wise.KYCRequirementRejected,
		wise.KYCRequirementExpired,
	}
	for _, s := range reqStates {
		if string(s) == "" {
			t.Errorf("empty requirement state constant: %v", s)
		}
	}
}

// KYCReview is now part of API completeness — verify explicitly.
func TestKYCReview_ServiceNonNil(t *testing.T) {
	c, _ := wise.New(wise.WithPersonalToken("tok"))
	if c.KYCReview == nil {
		t.Fatal("KYCReview service is nil")
	}
}

// ---------------------------------------------------------------------------
// Recipient: POST /v1/account-requirements (context refresh)
// ---------------------------------------------------------------------------

func TestRecipients_RefreshAccountRequirements(t *testing.T) {
	want := []wise.RequirementField{{Name: "sortCode"}, {Name: "accountNumber"}}
	rt := newRouter()
	rt.Handle("POST", "/v1/account-requirements", jsonResp(t, 200, want))
	c := newTestServer(t, rt)

	got, err := c.Recipients.RefreshAccountRequirements(
		context.Background(),
		"GBP", "GBP", 100.0,
		map[string]any{"details": map[string]string{"accountType": "checking"}},
	)
	if err != nil {
		t.Fatalf("RefreshAccountRequirements: %v", err)
	}
	if len(got) != 2 || got[0].Name != "sortCode" {
		t.Errorf("unexpected fields: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Webhook Test: POST /v3/subscriptions/{id}/test
// ---------------------------------------------------------------------------

func TestWebhooks_Test_UsesPost(t *testing.T) {
	var method string
	rt := newRouter()
	rt.Handle("POST", "/v3/subscriptions/sub-xyz/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	c := newTestServer(t, rt)

	if err := c.Webhooks.Test(context.Background(), "sub-xyz"); err != nil {
		t.Fatalf("Test: %v", err)
	}
	if method != http.MethodPost {
		t.Errorf("want POST, got %s", method)
	}
}

// ---------------------------------------------------------------------------
// Rate: GET /v1/rates (was incorrectly using /v3/comparisons/rates)
// ---------------------------------------------------------------------------

func TestRates_Get_UsesCorrectPath(t *testing.T) {
	want := []wise.ExchangeRate{{Source: "USD", Target: "GBP", Rate: 0.79}}
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		json.NewEncoder(w).Encode(want) //nolint:errcheck
	}))
	defer srv.Close()

	c, _ := wise.New(
		wise.WithPersonalToken("tok"),
		wise.WithBaseURL(srv.URL),
		wise.WithMaxRetries(0),
	)
	_, _ = c.Rates.Get(context.Background(), wise.GetRateParams{Source: "USD", Target: "GBP"})

	if path != "/v1/rates" {
		t.Errorf("want /v1/rates, got %s", path)
	}
}
