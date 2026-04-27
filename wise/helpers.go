package wise

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

// -----------------------------------------------------------------------------
// Context helpers
// -----------------------------------------------------------------------------

type contextKey struct{ name string }

var (
	idempotencyKeyCtxKey = &contextKey{"idempotency-key"}
	requestIDCtxKey      = &contextKey{"request-id"}
)

// WithIdempotencyKey returns a new context carrying an idempotency key.
// The key is attached as the X-Idempotency-Key header on state-changing requests
// (POST transfers, fund batch, etc.) to prevent duplicate operations on retry.
//
//	ctx = wise.WithIdempotencyKey(ctx, wise.NewIdempotencyKey())
//	transfer, err := client.Transfers.Create(ctx, req)
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKeyCtxKey, key)
}

// WithRequestID returns a new context carrying a request ID.
// The ID is sent as X-Request-Id to aid Wise support escalation.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey, id)
}

// NewIdempotencyKey generates a cryptographically random 32-character hex key.
// Use one per logical operation to ensure exactly-once semantics on retries.
func NewIdempotencyKey() string {
	b := make([]byte, 16) //nolint:mnd // 16 bytes = 32 hex chars

	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is not recoverable in production.
		panic(fmt.Sprintf("wise: crypto/rand failed: %v", err))
	}

	return hex.EncodeToString(b)
}

// idempotencyKeyFromCtx extracts an idempotency key from ctx.
func idempotencyKeyFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(idempotencyKeyCtxKey).(string)
	return v
}

// requestIDFromCtx extracts a request ID from ctx.
func requestIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(requestIDCtxKey).(string)
	return v
}

// applyContextHeaders injects context-carried headers onto req.
func applyContextHeaders(ctx context.Context, req *http.Request) {
	if key := idempotencyKeyFromCtx(ctx); key != "" {
		req.Header.Set(headerIdempotencyKey, key)
	}

	if id := requestIDFromCtx(ctx); id != "" {
		req.Header.Set(headerRequestID, id)
	}
}

// -----------------------------------------------------------------------------
// Generic iterator
// -----------------------------------------------------------------------------

// Iter is a generic pull-iterator over paginated Wise API resources.
//
//	iter := wise.NewIter(func(p wise.PageParams) ([]wise.Transfer, bool, error) {
//	    list, err := client.Transfers.List(ctx, wise.ListTransfersParams{
//	        PageParams: p, ProfileID: profileID,
//	    })
//	    return list, len(list) == p.Limit, err
//	})
//	for iter.Next() {
//	    t := iter.Item()
//	    fmt.Println(t.ID)
//	}
//	if err := iter.Err(); err != nil { /* handle */ }
//
// Iter is safe to use from a single goroutine; it is not concurrency-safe.
type Iter[T any] struct {
	fetchFn func(PageParams) ([]T, bool, error)
	buf     []T
	pos     int
	page    PageParams
	err     error
	done    bool
}

// NewIter creates an Iter using fetchFn to page through results.
// FetchFn must return (items, hasMore, err).
// If Limit is zero it defaults to 100.
func NewIter[T any](fetchFn func(PageParams) ([]T, bool, error)) *Iter[T] {
	return &Iter[T]{
		fetchFn: fetchFn,
		buf:     nil,
		pos:     0,
		page:    PageParams{Limit: 100, Offset: 0, Cursor: ""}, //nolint:mnd // sensible default page size
		err:     nil,
		done:    false,
	}
}

// Next advances the iterator. Returns true while items are available.
func (it *Iter[T]) Next() bool {
	if it.err != nil {
		return false
	}

	// Advance within the current buffer first.
	if it.pos < len(it.buf) {
		it.pos++
		return true
	}

	// Buffer exhausted. If no more pages, stop.
	if it.done {
		return false
	}

	// Fetch the next page.
	items, hasMore, err := it.fetchFn(it.page)
	if err != nil {
		it.err = err
		return false
	}

	if len(items) == 0 {
		it.done = true
		return false
	}

	it.buf = items
	it.pos = 1
	it.page.Offset += len(items)

	if !hasMore {
		it.done = true // consume this buffer then stop
	}

	return true
}

// Item returns the current item. Must be called after Next returns true.
func (it *Iter[T]) Item() T { return it.buf[it.pos-1] }

// Err returns any error encountered during iteration.
func (it *Iter[T]) Err() error { return it.err }

// -----------------------------------------------------------------------------
// Generic helpers
// -----------------------------------------------------------------------------

// Ptr returns a pointer to v.
// Useful for setting optional numeric or string fields in request structs.
//
//	req := wise.CreateQuoteRequest{SourceAmount: wise.Ptr(1000.0)}
//
// The generic parameter T is inferred from v.
func Ptr[T any](v T) *T { return &v }
