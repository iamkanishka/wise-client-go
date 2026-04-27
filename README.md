# wise-client-go

A production-grade Go client for the [Wise Platform API](https://docs.wise.com/api-reference).

[![Go Version](https://img.shields.io/badge/go-1.25%2B-blue)](https://go.dev)
[![Module](https://img.shields.io/badge/module-github.com%2Fiamkanishka%2Fwise-blue)](https://github.com/iamkanishka/wise-client-go)

## Features

- **All 42 Wise API groups** — Profile, Quote, Transfer, Balance, Card, Recipient, Webhook, KYC, and more
- **Three auth modes** — Personal Token, OAuth 2.0 Client Credentials, OAuth 2.0 User Token
- **Zero external dependencies** — pure Go standard library
- **Production-ready transport** — retry with exponential back-off, token-bucket rate limiter, circuit breaker
- **Middleware hooks** — structured logging (`log/slog`), metrics (Prometheus-compatible)
- **Generic iterator** — `Iter[T]` for seamless pagination
- **Webhook helpers** — `EventRouter` HTTP handler with HMAC-SHA256 signature verification
- **Full test coverage** — 133 unit tests, race-detector clean

---

## Installation

```bash
go get github.com/iamkanishka/wise-client-go/wise
```

Requires **Go 1.25+**.

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/iamkanishka/wise-client-go/wise"
)

func main() {
    client, err := wise.New(
        wise.WithPersonalToken(os.Getenv("WISE_API_TOKEN")),
        wise.WithEnvironment(wise.Sandbox), // or wise.Production
    )
    if err != nil {
        log.Fatal(err)
    }

    profiles, err := client.Profiles.List(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    for _, p := range profiles {
        fmt.Printf("Profile %d (%s)\n", p.ID, p.Type)
    }
}
```

---

## Authentication

### Personal API Token

```go
client, _ := wise.New(wise.WithPersonalToken(os.Getenv("WISE_API_TOKEN")))
```

### OAuth 2.0 — Client Credentials (auto-refresh)

```go
client, _ := wise.New(wise.WithClientCredentials(clientID, clientSecret))
```

### OAuth 2.0 — User Token with custom refresh

```go
client, _ := wise.New(wise.WithUserToken(
    accessToken, refreshToken, expiry,
    func(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
        // your token refresh logic
    },
))
```

---

## Environments

```go
wise.WithEnvironment(wise.Production) // https://api.wise.com  (default)
wise.WithEnvironment(wise.Sandbox)    // https://api.wise-sandbox.com
```

---

## Sending Money

```go
// 1. Create a quote
quote, _ := client.Quotes.Create(ctx, profileID, wise.CreateQuoteRequest{
    SourceCurrency: "USD",
    TargetCurrency: "GBP",
    SourceAmount:   wise.Ptr(1000.0),
    PayIn:          wise.PaymentMethodBalance,
})

// 2. Create a recipient
recipient, _ := client.Recipients.Create(ctx, wise.CreateRecipientRequest{
    Profile:           profileID,
    AccountHolderName: "Alice Smith",
    Currency:          "GBP",
    Type:              "sort_code",
    Details:           map[string]any{"sortCode": "040075", "accountNumber": "12345678"},
})

// 3. Create a transfer (with idempotency key)
idemKey := wise.NewIdempotencyKey()
ctx = wise.WithIdempotencyKey(ctx, idemKey)
transfer, _ := client.Transfers.Create(ctx, wise.CreateTransferRequest{
    TargetAccount:         recipient.ID,
    QuoteUUID:             quote.ID,
    CustomerTransactionID: idemKey,
    Details:               wise.TransferDetails{Reference: "Invoice #42"},
})

// 4. Fund from balance
funded, err := client.Transfers.Fund(ctx, profileID, transfer.ID)
if wise.IsSCARequired(err) {
    // redirect user to Wise for 2FA
}
```

---

## Error Handling

```go
_, err := client.Transfers.Fund(ctx, profileID, transferID)
switch {
case wise.IsNotFound(err):    // 404
case wise.IsSCARequired(err): // 403 SCA_REQUIRED
case wise.IsRateLimited(err): // 429
case wise.IsServerError(err): // 5xx
}

// Field-level validation errors (422)
for _, fe := range wise.FieldErrors(err) {
    fmt.Printf("field=%s code=%s msg=%s\n", fe.Field, fe.Code, fe.Message)
}
```

---

## Pagination

```go
iter := wise.NewIter(func(p wise.PageParams) ([]wise.Transfer, bool, error) {
    list, err := client.Transfers.List(ctx, wise.ListTransfersParams{
        PageParams: p,
        ProfileID:  profileID,
    })
    return list, len(list) == p.Limit, err
})
for iter.Next() {
    t := iter.Item()
    fmt.Println(t.ID, t.Status)
}
if err := iter.Err(); err != nil {
    log.Fatal(err)
}
```

---

## Webhooks

```go
router := wise.NewEventRouter(os.Getenv("WISE_WEBHOOK_SECRET"))

router.On(wise.EventTransferStateChange, func(e *wise.WebhookEvent) error {
    var ev wise.TransferStateChangeEvent
    e.UnmarshalData(&ev)
    fmt.Printf("transfer %d: %s → %s\n", ev.Resource.ID, ev.PreviousState, ev.CurrentState)
    return nil
})

http.Handle("/webhooks/wise", router)
http.ListenAndServe(":8080", nil)
```

---

## Production Configuration

```go
client, _ := wise.New(
    wise.WithPersonalToken(token),
    wise.WithEnvironment(wise.Production),
    wise.WithTimeout(30*time.Second),
    wise.WithMaxRetries(3),
    wise.WithTransportConfig(wise.DefaultTransportConfig()),
    wise.WithCircuitBreaker(wise.NewCircuitBreaker(wise.CircuitBreakerConfig{
        FailureThreshold: 5,
        Timeout:          30 * time.Second,
    })),
    wise.WithResponseHook(wise.SlogLoggingHook(slog.Default())),
    wise.WithResponseHook(wise.MetricsHook(func(method, path string, code int, d time.Duration, failed bool) {
        apiRequests.WithLabelValues(method, path, fmt.Sprint(code)).Inc()
    })),
)
```

---

## All 42 API Groups

| Service | Field | Description |
|---|---|---|
| Profile | `client.Profiles` | Personal & business profiles |
| Quote | `client.Quotes` | Rate locking & fee calculation |
| Recipient Account | `client.Recipients` | Beneficiary account management |
| Transfer | `client.Transfers` | Payment creation & funding |
| Balance | `client.Balances` | Multi-currency balances |
| Balance Statement | `client.Statements` | JSON / CSV / PDF / XLSX statements |
| Bank Account Details | `client.BankAccounts` | Receive-money bank details |
| Batch Group | `client.Batches` | Batch payments (up to 1,000) |
| Bulk Settlement | `client.Batches` | Settlement journal submission |
| Card | `client.Cards` | Card status, PIN, sensitive data |
| Card Kiosk Collection | `client.KioskCollection` | On-site card printing |
| Card Order | `client.CardOrders` | Order physical & virtual cards |
| Card Transaction | `client.CardTxns` | Card transaction history |
| Claim Account | `client.ClaimAccount` | Account claim code generation |
| Client Credentials Token | `client.OAuthSvc` | OAuth token exchange |
| Comparison | `client.Comparisons` | Multi-provider price comparison |
| Contact | `client.Contacts` | Find profiles by Wisetag/email/phone |
| Currencies | `client.Currencies` | Supported currencies list |
| Delivery Estimate | `client.Transfers` | Transfer delivery time |
| Direct Debit Account | `client.DirectDebits` | ACH/EFT funding accounts |
| Disputes | `client.Disputes` | Card transaction disputes |
| FaceTec | `client.FaceTec` | Biometric public key |
| JOSE | `client.JOSE` | JWS/JWE key management |
| KYC Review | `client.KYCReview` | Hosted & API verification flows |
| Multi Currency Account | `client.MultiCurrencyAccount` | MCA configuration |
| One Time Token | `client.OTT` | Legacy SCA (deprecated) |
| Partner Cases | `client.Cases` | Support case management |
| Payin Deposit Detail | `client.Transfers` | Funding instructions |
| Additional Customer Verification | `client.KYC` | Evidence upload |
| Push Provisioning | `client.PushProvisioning` | Apple/Google Pay provisioning |
| Rate | `client.Rates` | Exchange rates |
| Simulation | `client.Simulations` | Sandbox state simulation |
| Spend Controls | `client.SpendControls` | MCC & transaction restrictions |
| Spend Limits | `client.SpendLimits` | Per-card & per-profile limits |
| Strong Customer Authentication | `client.SCA` | Modern SCA framework |
| Transfer Requirements | `client.Transfers` | Required transfer fields |
| User | `client.Users` | User account management |
| User Security | `client.UserSecurity` | PIN, FaceMap, phone, device |
| User Tokens | `client.OAuthSvc` | Token refresh |
| Webhook | `client.Webhooks` | Subscription management |
| 3D Secure | `client.ThreeDS` | 3DS challenge results |
| Address | `client.Addresses` | Address management |

---

## Package Structure

```
github.com/iamkanishka/wise-client-go/
├── wise.go                    Package documentation
├── go.mod
├── .golangci.yml
└── wise/                      Implementation (import this)
    ├── client.go
    ├── options.go
    ├── errors.go
    ├── types.go
    ├── helpers.go
    ├── transport.go
    ├── services.go
    ├── cards.go
    ├── auth.go
    ├── kyc_review.go
    ├── wise_test.go
    ├── internal/ratelimit/
    └── examples/
        ├── send_money/
        └── webhooks/
```

---

## License

MIT
