// Package main demonstrates mounting the Wise EventRouter as an HTTP handler,
// with HMAC-SHA256 signature verification and typed event decoding.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/iamkanishka/wise-go/wise"
)

func main() {
	secret := os.Getenv("WISE_WEBHOOK_SECRET")
	token := os.Getenv("WISE_API_TOKEN")

	// Build Wise client for registering and managing subscriptions.
	client, err := wise.New(
		wise.WithPersonalToken(token),
		wise.WithEnvironment(wise.Sandbox),
		wise.WithLogger(slog.Default()),
		wise.WithResponseHook(wise.SlogLoggingHook(slog.Default())),
	)
	if err != nil {
		log.Fatalf("wise.New: %v", err)
	}

	// Register a webhook subscription (one-time setup; idempotent in sandbox).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profiles, err := client.Profiles.List(ctx)
	if err != nil {
		log.Fatalf("profiles.List: %v", err)
	}

	if len(profiles) > 0 {
		_, err = client.Webhooks.Create(ctx, wise.CreateWebhookRequest{
			Name:      "local-dev-handler",
			TriggerOn: wise.EventTransferStateChange,
			URL:       "https://your-domain.example.com/webhooks/wise",
			ProfileID: profiles[0].ID,
		})
		if err != nil {
			log.Printf("webhooks.Create (may already exist): %v", err)
		}
	}

	// Build the event router.
	router := wise.NewEventRouter(secret)

	// Handle transfer state changes.
	router.On(wise.EventTransferStateChange, func(e *wise.WebhookEvent) error {
		var ev wise.TransferStateChangeEvent
		if err := e.UnmarshalData(&ev); err != nil {
			return fmt.Errorf("unmarshal transfer event: %w", err)
		}

		slog.Info("transfer state changed",
			"transfer_id", ev.Resource.ID,
			"profile_id", ev.Resource.Profile,
			"from", ev.PreviousState,
			"to", ev.CurrentState,
			"occurred_at", ev.OccurredAt.Format(time.RFC3339),
		)

		return nil
	})

	// Handle balance credit events.
	router.On(wise.EventBalanceCredit, func(e *wise.WebhookEvent) error {
		var ev wise.BalanceCreditEvent
		if err := e.UnmarshalData(&ev); err != nil {
			return fmt.Errorf("unmarshal balance event: %w", err)
		}

		slog.Info("balance credited",
			"balance_id", ev.Resource.ID,
			"amount", ev.Amount.String(),
		)

		return nil
	})

	// Mount on /webhooks/wise and start serving.
	mux := http.NewServeMux()
	mux.Handle("/webhooks/wise", router)

	// Health check endpoint.
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	addr := ":8080"
	slog.Info("webhook server listening", "addr", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
