package wise

import (
	"context"
	"fmt"
	"time"
)

// =============================================================================
// Card Service
// =============================================================================

// CardService manages card status, permissions, and sensitive card data.
type CardService struct{ c *Client }

// Card represents a Wise payment card.
type Card struct {
	CardToken      string     `json:"cardToken"`
	Status         CardStatus `json:"status"`
	Type           CardType   `json:"cardType"`
	CardNumber     string     `json:"cardNumber,omitempty"`
	ExpiryDate     string     `json:"expiryDate,omitempty"`
	CardHolderName string     `json:"cardHolderName"`
	PhoneNumber    string     `json:"phoneNumber,omitempty"`
	CardOrderID    string     `json:"cardOrderId,omitempty"`
	ProgramType    string     `json:"programType,omitempty"`
	Brand          string     `json:"brand,omitempty"`
}

// SpendingPermissions holds all payment-method permission flags for a card.
type SpendingPermissions struct {
	AllowTransactions       bool `json:"allowTransactions"`
	AllowCashWithdrawals    bool `json:"allowCashWithdrawals"`
	AllowOnlineTransactions bool `json:"allowOnlineTransactions"`
	AllowContactless        bool `json:"allowContactless"`
	AllowMobileWallets      bool `json:"allowMobileWallets"`
	AllowSwipeTransactions  bool `json:"allowSwipeTransactions"`
	AllowChipTransactions   bool `json:"allowChipTransactions"`
}

// EncryptionKey is the RSA public key for encrypting sensitive card data requests.
type EncryptionKey struct {
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
}

// SensitiveCardData holds decrypted PAN, CVV, and PIN details.
type SensitiveCardData struct {
	PAN            string `json:"pan"`
	CVV            string `json:"cvv"`
	ExpiryDate     string `json:"expiryDate"`
	CardHolderName string `json:"cardHolderName"`
}

// ListCards returns a paginated list of cards for the given profile.
func (s *CardService) ListCards(ctx context.Context, profileID int64, p PageParams) ([]Card, error) {
	var cards []Card
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/cards", profileID), p.Values(), &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

// GetCard retrieves a card by its token.
func (s *CardService) GetCard(ctx context.Context, profileID int64, cardToken string) (*Card, error) {
	var card Card
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/cards/%s", profileID, cardToken), nil, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// UpdateStatus changes the card status (ACTIVE, FROZEN, or BLOCKED).
func (s *CardService) UpdateStatus(ctx context.Context, profileID int64, cardToken string, status CardStatus) (*Card, error) {
	body := map[string]string{"status": string(status)}
	var card Card
	if err := s.c.put(ctx, fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/status", profileID, cardToken), body, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// ResetPINCount unlocks a card blocked due to too many incorrect PIN entries.
func (s *CardService) ResetPINCount(ctx context.Context, profileID int64, cardToken string) error {
	return s.c.post(ctx, fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/reset-pin-count", profileID, cardToken), nil, nil)
}

// GetSpendingPermissions returns the current spending permission flags for a card.
func (s *CardService) GetSpendingPermissions(ctx context.Context, profileID int64, cardToken string) (*SpendingPermissions, error) {
	var perms SpendingPermissions
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/spending-permissions", profileID, cardToken), nil, &perms); err != nil {
		return nil, err
	}
	return &perms, nil
}

// UpdateSinglePermission enables or disables a single spending permission on a card.
// For updating multiple permissions atomically, use UpdateSpendingPermissions (v4) instead.
//
// PATCH /v3/spend/profiles/{profileId}/cards/{cardToken}/spending-permissions.
func (s *CardService) UpdateSinglePermission(ctx context.Context, profileID int64, cardToken, permissionType string, allowed bool) (*SpendingPermissions, error) {
	body := map[string]any{"type": permissionType, "allowed": allowed}
	var updated SpendingPermissions
	if err := s.c.patch(ctx, fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/spending-permissions", profileID, cardToken), body, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// UpdateSpendingPermissions atomically updates multiple spending permission flags.
// This is the recommended endpoint for bulk permission updates.
//
// PATCH /v4/spend/profiles/{profileId}/cards/{cardToken}/spending-permissions.
func (s *CardService) UpdateSpendingPermissions(ctx context.Context, profileID int64, cardToken string, perms SpendingPermissions) (*SpendingPermissions, error) {
	var updated SpendingPermissions
	if err := s.c.patch(ctx, fmt.Sprintf("/v4/spend/profiles/%d/cards/%s/spending-permissions", profileID, cardToken), perms, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// GetEncryptionKey fetches the RSA public key for encrypting sensitive card data requests.
func (s *CardService) GetEncryptionKey(ctx context.Context) (*EncryptionKey, error) {
	var key EncryptionKey
	if err := s.c.get(ctx, "/twcard-data/v1/clientSideEncryption/fetchEncryptingKey", nil, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// GetSensitiveDetails retrieves PAN, CVV, and expiry. This endpoint is SCA-protected.
// EncryptedPayload must be a JWE-encrypted request body.
func (s *CardService) GetSensitiveDetails(ctx context.Context, encryptedPayload string) (*SensitiveCardData, error) {
	body := map[string]string{"encryptedPayload": encryptedPayload}
	var data SensitiveCardData
	if err := s.c.post(ctx, "/twcard-data/v1/sensitive-card-data/details", body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetPIN retrieves the card PIN. SCA-protected.
func (s *CardService) GetPIN(ctx context.Context, encryptedPayload string) (string, error) {
	body := map[string]string{"encryptedPayload": encryptedPayload}
	var resp struct {
		PIN string `json:"pin"`
	}
	if err := s.c.post(ctx, "/twcard-data/v1/sensitive-card-data/pin", body, &resp); err != nil {
		return "", err
	}
	return resp.PIN, nil
}

// =============================================================================
// Card Order Service
// =============================================================================

// CardOrderService manages ordering physical and virtual Wise cards.
type CardOrderService struct{ c *Client }

// CardOrder represents a card order and its lifecycle state.
type CardOrder struct {
	ID        string   `json:"id"`
	ProfileID int64    `json:"profileId"`
	CardToken string   `json:"cardToken,omitempty"`
	CardType  CardType `json:"cardType"`
	Status    string   `json:"status"`
	CreatedAt Time     `json:"createdAt"`
	UpdatedAt Time     `json:"updatedAt"`
}

// CardProgram describes an available card product configuration.
type CardProgram struct {
	ProgramID   string   `json:"programId"`
	CardType    CardType `json:"cardType"`
	Description string   `json:"description"`
}

// CardOrderRequirement is a single requirement for completing a card order.
type CardOrderRequirement struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// CreateCardOrderRequest is the body for ordering a new card.
type CreateCardOrderRequest struct {
	ProfileID      int64    `json:"profileId"`
	CardType       CardType `json:"cardType"`
	CardProgramID  string   `json:"cardProgramId,omitempty"`
	CardHolderName string   `json:"cardHolderName,omitempty"`
	PhoneNumber    string   `json:"phoneNumber,omitempty"`
	LifetimeLimit  *float64 `json:"lifetimeLimit,omitempty"`
}

// Create creates a card order. Use WithIdempotencyKey on the context.
func (s *CardOrderService) Create(ctx context.Context, profileID int64, req CreateCardOrderRequest) (*CardOrder, error) {
	var co CardOrder
	if err := s.c.post(ctx, fmt.Sprintf("/v3/spend/profiles/%d/card-orders", profileID), req, &co); err != nil {
		return nil, err
	}
	return &co, nil
}

// List returns all card orders for the given profile.
func (s *CardOrderService) List(ctx context.Context, profileID int64, p PageParams) ([]CardOrder, error) {
	var orders []CardOrder
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/card-orders", profileID), p.Values(), &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

// Get retrieves a card order by ID.
func (s *CardOrderService) Get(ctx context.Context, profileID int64, cardOrderID string) (*CardOrder, error) {
	var co CardOrder
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/card-orders/%s", profileID, cardOrderID), nil, &co); err != nil {
		return nil, err
	}
	return &co, nil
}

// ListPrograms returns available card programs for the profile.
func (s *CardOrderService) ListPrograms(ctx context.Context, profileID int64) ([]CardProgram, error) {
	var programs []CardProgram
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/card-orders/availability", profileID), nil, &programs); err != nil {
		return nil, err
	}
	return programs, nil
}

// GetRequirements retrieves all requirements and their statuses for a card order.
func (s *CardOrderService) GetRequirements(ctx context.Context, profileID int64, cardOrderID string) ([]CardOrderRequirement, error) {
	var reqs []CardOrderRequirement
	path := fmt.Sprintf("/v3/spend/profiles/%d/card-orders/%s/requirements", profileID, cardOrderID)
	if err := s.c.get(ctx, path, nil, &reqs); err != nil {
		return nil, err
	}
	return reqs, nil
}

// UpdateStatus updates the card order status (e.g. to CANCELED).
func (s *CardOrderService) UpdateStatus(ctx context.Context, profileID int64, cardOrderID, status string) (*CardOrder, error) {
	body := map[string]string{"status": status}
	var co CardOrder
	if err := s.c.put(ctx, fmt.Sprintf("/v3/spend/profiles/%d/card-orders/%s/status", profileID, cardOrderID), body, &co); err != nil {
		return nil, err
	}
	return &co, nil
}

// ValidateAddress validates a delivery or billing address for a card order.
func (s *CardOrderService) ValidateAddress(ctx context.Context, addr Address) error {
	return s.c.post(ctx, "/v3/spend/address/validate", addr, nil)
}

// SetPresetPIN sets a PIN during the card order flow for supported partners.
func (s *CardOrderService) SetPresetPIN(ctx context.Context, cardOrderID, encryptedPIN string) error {
	body := map[string]string{"cardOrderId": cardOrderID, "encryptedPin": encryptedPIN}
	req, err := s.c.newRequest(ctx, "POST", "/twcard-data/v1/sensitive-card-data/preset-pin", body)
	if err != nil {
		return err
	}
	req.Header.Set("X-card-order-id", cardOrderID)
	return s.c.do(req, nil)
}

// =============================================================================
// Card Transaction Service
// =============================================================================

// CardTransactionService retrieves transaction history for Wise cards.
type CardTransactionService struct{ c *Client }

// CardTransaction represents a transaction on a Wise card.
type CardTransaction struct {
	ID                    string    `json:"id"`
	Card                  CardRef   `json:"card"`
	Type                  string    `json:"type"`
	State                 string    `json:"state"`
	TransactionAmount     Amount    `json:"transactionAmount"`
	BillingAmount         Amount    `json:"billingAmount"`
	Merchant              *Merchant `json:"merchant,omitempty"`
	Date                  Time      `json:"date"`
	DeclineReason         string    `json:"declineReason,omitempty"`
	DetailedDeclineReason string    `json:"detailedDeclineReason,omitempty"`
	BalanceAfter          *Amount   `json:"balanceAfter,omitempty"`
	Reference             string    `json:"reference,omitempty"`
}

// CardRef is a lightweight card identifier embedded in a transaction.
type CardRef struct {
	Token string `json:"token"`
}

// Merchant holds merchant metadata for a card transaction.
type Merchant struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	CategoryCode string `json:"categoryCode"`
	City         string `json:"city,omitempty"`
	Country      string `json:"country,omitempty"`
}

// ListCardTransactionsParams configures a card transaction list query.
type ListCardTransactionsParams struct {
	PageParams
	StartDate time.Time
	EndDate   time.Time
	State     string
	Type      string
}

// List returns transactions for a specific card.
func (s *CardTransactionService) List(ctx context.Context, profileID int64, cardToken string, p ListCardTransactionsParams) ([]CardTransaction, error) {
	params := p.Values()
	if !p.StartDate.IsZero() {
		params.Set("intervalStart", p.StartDate.Format(time.RFC3339))
	}
	if !p.EndDate.IsZero() {
		params.Set("intervalEnd", p.EndDate.Format(time.RFC3339))
	}
	if p.State != "" {
		params.Set("state", p.State)
	}
	if p.Type != "" {
		params.Set("type", p.Type)
	}
	path := fmt.Sprintf("/v4/spend/profiles/%d/cards/%s/transactions", profileID, cardToken)
	var txns []CardTransaction
	if err := s.c.get(ctx, path, params, &txns); err != nil {
		return nil, err
	}
	return txns, nil
}

// Get retrieves a single card transaction by ID.
func (s *CardTransactionService) Get(ctx context.Context, profileID int64, transactionID string) (*CardTransaction, error) {
	var txn CardTransaction
	path := fmt.Sprintf("/v4/spend/profiles/%d/cards/transactions/%s", profileID, transactionID)
	if err := s.c.get(ctx, path, nil, &txn); err != nil {
		return nil, err
	}
	return &txn, nil
}

// =============================================================================
// Spend Limit Service
// =============================================================================

// SpendLimitService manages card and profile-level spending limits.
type SpendLimitService struct{ c *Client }

// SpendLimits holds the spending limit configuration.
type SpendLimits struct {
	Daily       *SpendLimit `json:"daily,omitempty"`
	Monthly     *SpendLimit `json:"monthly,omitempty"`
	Transaction *SpendLimit `json:"transaction,omitempty"`
	Lifetime    *SpendLimit `json:"lifetime,omitempty"`
}

// SpendLimit is a single spending limit definition.
type SpendLimit struct {
	Value    *float64 `json:"value"`
	Currency string   `json:"currency"`
	Used     *float64 `json:"used,omitempty"`
}

// GetProfileLimits returns spending limits at the profile level.
func (s *SpendLimitService) GetProfileLimits(ctx context.Context, profileID int64) (*SpendLimits, error) {
	var limits SpendLimits
	if err := s.c.get(ctx, fmt.Sprintf("/v1/spend/profiles/%d/limits", profileID), nil, &limits); err != nil {
		return nil, err
	}
	return &limits, nil
}

// UpdateProfileLimits sets spending limits at the profile level.
func (s *SpendLimitService) UpdateProfileLimits(ctx context.Context, profileID int64, limits SpendLimits) (*SpendLimits, error) {
	var updated SpendLimits
	if err := s.c.put(ctx, fmt.Sprintf("/v1/spend/profiles/%d/limits", profileID), limits, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// GetCardLimits returns spending limits for a specific card.
func (s *SpendLimitService) GetCardLimits(ctx context.Context, profileID int64, cardToken string) (*SpendLimits, error) {
	var limits SpendLimits
	path := fmt.Sprintf("/v1/spend/profiles/%d/cards/%s/limits", profileID, cardToken)
	if err := s.c.get(ctx, path, nil, &limits); err != nil {
		return nil, err
	}
	return &limits, nil
}

// UpdateCardLimits sets spending limits for a specific card.
func (s *SpendLimitService) UpdateCardLimits(ctx context.Context, profileID int64, cardToken string, limits SpendLimits) (*SpendLimits, error) {
	var updated SpendLimits
	path := fmt.Sprintf("/v1/spend/profiles/%d/cards/%s/limits", profileID, cardToken)
	if err := s.c.put(ctx, path, limits, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// =============================================================================
// Spend Control Service
// =============================================================================

// SpendControlService manages MCC and transaction-type restrictions on cards.
type SpendControlService struct{ c *Client }

// SpendControls defines merchant and transaction restrictions on a card.
type SpendControls struct {
	AllowedMCCs             []string `json:"allowedMccs,omitempty"`
	BlockedMCCs             []string `json:"blockedMccs,omitempty"`
	AllowedTransactionTypes []string `json:"allowedTransactionTypes,omitempty"`
}

// Get retrieves spend controls for a card.
func (s *SpendControlService) Get(ctx context.Context, profileID int64, cardToken string) (*SpendControls, error) {
	var sc SpendControls
	if err := s.c.get(ctx, fmt.Sprintf("/v1/spend/profiles/%d/cards/%s/spend-controls", profileID, cardToken), nil, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// Update sets spend controls for a card.
func (s *SpendControlService) Update(ctx context.Context, profileID int64, cardToken string, controls SpendControls) (*SpendControls, error) {
	var updated SpendControls
	if err := s.c.put(ctx, fmt.Sprintf("/v1/spend/profiles/%d/cards/%s/spend-controls", profileID, cardToken), controls, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

// =============================================================================
// Dispute Service
// =============================================================================

// DisputeService manages card transaction disputes.
type DisputeService struct{ c *Client }

// Dispute represents an active or resolved card transaction dispute.
type Dispute struct {
	ID            string           `json:"id"`
	TransactionID string           `json:"transactionId"`
	Status        string           `json:"status"`
	SubStatus     DisputeSubStatus `json:"subStatus"`
	Reason        string           `json:"reason"`
	Scheme        string           `json:"scheme"`
	CreatedAt     Time             `json:"createdAt"`
	UpdatedAt     Time             `json:"updatedAt"`
}

// DisputeReason holds metadata for a dispute reason option.
type DisputeReason struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Scheme      string `json:"scheme"`
}

// DisputeFile is returned when a file is uploaded for a dispute submission.
type DisputeFile struct {
	// FileID is the identifier to reference in the dispute submission's files object.
	FileID    string `json:"fileId"`
	ExpiresAt Time   `json:"expiresAt"`
}

// DynamicFlowEntry retrieves the JSON for initiating a dispute using Wise's
// Dynamic Flow framework. Pass the response into the Dynamic Flow JS library.
//
// POST /v3/spend/profiles/{profileId}/dispute-form/flows/step/{scheme}/{reason}.
func (s *DisputeService) DynamicFlowEntry(ctx context.Context, profileID int64, scheme, reason, transactionID string) (map[string]any, error) {
	body := map[string]string{"transactionId": transactionID}
	var resp map[string]any
	path := fmt.Sprintf("/v3/spend/profiles/%d/dispute-form/flows/step/%s/%s", profileID, scheme, reason)
	if err := s.c.post(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Submit submits a dispute for a card transaction directly via API.
// The request body structure varies per dispute reason code; see the
// Wise "Disputes via API" guide for per-reason field details.
//
// POST /v3/spend/profiles/{profileId}/dispute-form/flows/{scheme}/{reason}.
func (s *DisputeService) Submit(ctx context.Context, profileID int64, scheme, reason string, body map[string]any) (*Dispute, error) {
	var d Dispute
	path := fmt.Sprintf("/v3/spend/profiles/%d/dispute-form/flows/%s/%s", profileID, scheme, reason)
	if err := s.c.post(ctx, path, body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// UploadFile uploads a supporting file for a dispute submission.
// Use the returned FileID in the files object when calling Submit.
// The file must be submitted in a dispute within 2 hours or it expires.
//
// POST /v4/spend/profiles/{profileId}/dispute-form/file.
func (s *DisputeService) UploadFile(ctx context.Context, profileID int64, filename string, content []byte, mimeType string) (*DisputeFile, error) {
	body := map[string]any{
		"filename": filename,
		"content":  content,
		"mimeType": mimeType,
	}
	var f DisputeFile
	if err := s.c.post(ctx, fmt.Sprintf("/v4/spend/profiles/%d/dispute-form/file", profileID), body, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListReasons returns the available dispute reasons.
func (s *DisputeService) ListReasons(ctx context.Context, profileID int64) ([]DisputeReason, error) {
	var reasons []DisputeReason
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/dispute-form/reasons", profileID), nil, &reasons); err != nil {
		return nil, err
	}
	return reasons, nil
}

// List returns all disputes for the profile.
func (s *DisputeService) List(ctx context.Context, profileID int64, p PageParams) ([]Dispute, error) {
	var disputes []Dispute
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/disputes", profileID), p.Values(), &disputes); err != nil {
		return nil, err
	}
	return disputes, nil
}

// Get retrieves a single dispute by ID.
func (s *DisputeService) Get(ctx context.Context, profileID int64, disputeID string) (*Dispute, error) {
	var d Dispute
	if err := s.c.get(ctx, fmt.Sprintf("/v3/spend/profiles/%d/disputes/%s", profileID, disputeID), nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Withdraw cancels (withdraws) an active dispute.
func (s *DisputeService) Withdraw(ctx context.Context, profileID int64, disputeID string) error {
	body := map[string]string{"status": "WITHDRAWN"}
	return s.c.put(ctx, fmt.Sprintf("/v3/spend/profiles/%d/disputes/%s/status", profileID, disputeID), body, nil)
}

// =============================================================================
// Kiosk Collection Service
// =============================================================================

// KioskCollectionService provides on-site card printing via kiosk machines.
type KioskCollectionService struct{ c *Client }

// CardProductionStatus describes the lifecycle state of kiosk card production.
type CardProductionStatus struct {
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
	UpdatedAt Time   `json:"updatedAt"`
}

// ProduceCard sends the card to the kiosk machine for chip encryption and printing.
func (s *KioskCollectionService) ProduceCard(ctx context.Context, profileID int64, cardToken string) error {
	path := fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/production", profileID, cardToken)
	return s.c.put(ctx, path, map[string]string{}, nil)
}

// GetProductionStatus returns the current kiosk production status for a card.
func (s *KioskCollectionService) GetProductionStatus(ctx context.Context, profileID int64, cardToken string) (*CardProductionStatus, error) {
	var status CardProductionStatus
	path := fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/production", profileID, cardToken)
	if err := s.c.get(ctx, path, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// =============================================================================
// Push Provisioning Service
// =============================================================================

// PushProvisioningService manages Apple Pay / Google Pay wallet provisioning.
type PushProvisioningService struct{ c *Client }

// PushProvisioningSession is the result of initiating a wallet provisioning flow.
type PushProvisioningSession struct {
	SessionID          string `json:"sessionId"`
	OpaquePaymentData  string `json:"opaquePaymentData,omitempty"`
	ActivationData     string `json:"activationData,omitempty"`
	EphemeralPublicKey string `json:"ephemeralPublicKey,omitempty"`
	Nonce              string `json:"nonce,omitempty"`
	NonceSignature     string `json:"nonceSignature,omitempty"`
	Status             string `json:"status"`
}

// CreateSessionRequest is the body for starting a push-provisioning session.
type CreateSessionRequest struct {
	WalletType     string   `json:"walletType"`
	Certificates   []string `json:"certificates,omitempty"`
	Nonce          string   `json:"nonce,omitempty"`
	NonceSignature string   `json:"nonceSignature,omitempty"`
}

// CreateSession initiates a push-provisioning session for the given card.
func (s *PushProvisioningService) CreateSession(ctx context.Context, profileID int64, cardToken string, req CreateSessionRequest) (*PushProvisioningSession, error) {
	var session PushProvisioningSession
	path := fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/push-provision", profileID, cardToken)
	if err := s.c.post(ctx, path, req, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// GetStatus returns the current push-provisioning status for a card.
func (s *PushProvisioningService) GetStatus(ctx context.Context, profileID int64, cardToken string) (*PushProvisioningSession, error) {
	var session PushProvisioningSession
	path := fmt.Sprintf("/v3/spend/profiles/%d/cards/%s/push-provision", profileID, cardToken)
	if err := s.c.get(ctx, path, nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}
