// Package wise is the root of the Wise Platform Go SDK.
//
// The SDK implementation lives in the wise sub-package:
//
//	import "github.com/iamkanishka/wise/wise"
//
// # Quick Start
//
//	client, err := wise.New(
//	    wise.WithPersonalToken(os.Getenv("WISE_API_TOKEN")),
//	    wise.WithEnvironment(wise.Sandbox),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	profiles, err := client.Profiles.List(context.Background())
//
// # Structure
//
// All implementation files live inside the wise/ sub-package:
//
//	wise/
//	├── auth.go          OAuth, User, SCA, OTT, 3DS, KYC, Webhook, Contact, JOSE, …
//	├── cards.go         Card, CardOrder, CardTransaction, SpendLimit, Dispute, …
//	├── client.go        Client, New(), HTTP dispatch, DoRaw, Ping
//	├── errors.go        APIError, sentinel errors, FieldErrors, mock helpers
//	├── helpers.go       Iter[T] paginator, Ptr[T], idempotency key helpers
//	├── kyc_review.go    KYCReview (hosted and API-based verification workflows)
//	├── options.go       Functional options (WithPersonalToken, WithEnvironment, …)
//	├── services.go      Profile, Quote, Recipient, Transfer, Balance, Rate, …
//	├── transport.go     Retry, circuit breaker, hook middleware, TransportConfig
//	├── types.go         Domain types: Profile, Transfer, Balance, Card, Quote, …
//	├── internal/
//	│   └── ratelimit/   Token-bucket rate limiter (zero external dependencies)
//	└── examples/
//	    ├── send_money/  End-to-end transfer workflow
//	    └── webhooks/    HTTP webhook server with HMAC verification
package wise
