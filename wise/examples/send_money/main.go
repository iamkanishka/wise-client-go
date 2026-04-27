// Package main demonstrates the canonical Wise transfer workflow:
// Profile → Quote → Recipient → Transfer → Fund → Simulate (sandbox).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iamkanishka/wise-client-go/wise"
)

func main() {
	// Build client from environment variables.
	client, err := wise.New(
		wise.WithPersonalToken(os.Getenv("WISE_API_TOKEN")),
		wise.WithEnvironment(wise.Sandbox),
		wise.WithTimeout(30*time.Second),
		wise.WithMaxRetries(3),
		wise.WithTransportConfig(wise.DefaultTransportConfig()),
	)
	if err != nil {
		log.Fatalf("wise.New: %v", err)
	}

	ctx := context.Background()

	// 1. Verify connectivity.
	if err := client.Ping(ctx); err != nil {
		log.Fatalf("ping failed: %v", err)
	}

	fmt.Println("✓ Connected to Wise Sandbox")

	// 2. Find personal profile.
	profiles, err := client.Profiles.List(ctx)
	if err != nil {
		log.Fatalf("profiles.List: %v", err)
	}

	var profileID int64

	for _, p := range profiles {
		if p.Type == wise.ProfileTypePersonal {
			profileID = p.ID
			break
		}
	}

	if profileID == 0 {
		log.Fatal("no personal profile found")
	}

	fmt.Printf("✓ Profile ID: %d\n", profileID)

	// 3. Create a USD → GBP quote.
	quote, err := client.Quotes.Create(ctx, profileID, wise.CreateQuoteRequest{
		SourceCurrency: "USD",
		TargetCurrency: "GBP",
		SourceAmount:   wise.Ptr(100.0),
		PayIn:          wise.PaymentMethodBalance,
		PayOut:         wise.PaymentMethodBankTransfer,
	})
	if err != nil {
		log.Fatalf("quotes.Create: %v", err)
	}

	fmt.Printf("✓ Quote: %s → rate %.6f → target %.2f GBP\n",
		quote.ID, quote.Rate, quote.TargetAmount)

	// 4. Create GBP recipient (sandbox sort code).
	recipient, err := client.Recipients.Create(ctx, wise.CreateRecipientRequest{
		Profile:           profileID,
		AccountHolderName: "Example Recipient",
		Currency:          "GBP",
		Type:              "sort_code",
		Details: map[string]any{
			"sortCode":      "040075",
			"accountNumber": "12345678",
		},
	})
	if err != nil {
		log.Fatalf("recipients.Create: %v", err)
	}

	fmt.Printf("✓ Recipient ID: %d\n", recipient.ID)

	// Clean up recipient when done.
	defer func() {
		if delErr := client.Recipients.Delete(ctx, recipient.ID); delErr != nil {
			log.Printf("warning: delete recipient: %v", delErr)
		}
	}()

	// 5. Create transfer with idempotency key.
	idemKey := wise.NewIdempotencyKey()
	ctx = wise.WithIdempotencyKey(ctx, idemKey)

	transfer, err := client.Transfers.Create(ctx, wise.CreateTransferRequest{
		TargetAccount:         recipient.ID,
		QuoteUUID:             quote.ID,
		CustomerTransactionID: idemKey,
		Details:               wise.TransferDetails{Reference: "Example payment"},
	})
	if err != nil {
		log.Fatalf("transfers.Create: %v", err)
	}

	fmt.Printf("✓ Transfer %d created, status: %s\n", transfer.ID, transfer.Status)

	// 6. Fund from balance.
	funded, err := client.Transfers.Fund(ctx, profileID, transfer.ID)
	if err != nil {
		if wise.IsSCARequired(err) {
			fmt.Println("⚠ SCA required — redirect user to Wise for 2FA.")
			return
		}

		log.Fatalf("transfers.Fund: %v", err)
	}

	fmt.Printf("✓ Funded: type=%s status=%s\n", funded.Type, funded.Status)

	// 7. Advance through sandbox states.
	states := []wise.SimulateTransferState{
		wise.SimulateProcessing,
		wise.SimulateFundsConverted,
		wise.SimulateOutgoingPaymentSent,
	}

	for _, state := range states {
		updated, simErr := client.Simulations.AdvanceTransfer(ctx, transfer.ID, state)
		if simErr != nil {
			log.Printf("simulate %s: %v", state, simErr)
			break
		}

		fmt.Printf("  → %s\n", updated.Status)
	}

	// 8. Delivery estimate.
	est, err := client.Transfers.DeliveryEstimate(ctx, transfer.ID)
	if err != nil {
		log.Printf("delivery estimate: %v", err)
	} else {
		fmt.Printf("✓ Estimated delivery: %s (guaranteed=%v)\n",
			est.EstimatedDeliveryDate.Format(time.RFC3339), est.Guaranteed)
	}
}
