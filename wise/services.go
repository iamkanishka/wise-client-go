package wise

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// =============================================================================
// Profile Service
// =============================================================================

// ProfileService manages Wise personal and business profiles.
type ProfileService struct{ c *Client }

// CreatePersonalRequest is the body for creating a personal profile.
type CreatePersonalRequest struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DateOfBirth string `json:"dateOfBirth"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
}

// CreateBusinessRequest is the body for creating a business profile.
type CreateBusinessRequest struct {
	Name                   string  `json:"name"`
	RegistrationNumber     string  `json:"registrationNumber"`
	Category               string  `json:"category"`
	SubCategory            string  `json:"subCategory"`
	BusinessType           string  `json:"businessType"`
	CompanyRole            string  `json:"companyRole"`
	DescriptionOfBusiness  string  `json:"descriptionOfBusiness"`
	Webpage                string  `json:"webpage,omitempty"`
	AverageMonthlyPayments float64 `json:"averageMonthlyPayments,omitempty"`
}

// UpdatePersonalRequest is the body for updating a personal profile.
type UpdatePersonalRequest struct {
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	DateOfBirth string `json:"dateOfBirth,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
}

// List returns all profiles for the authenticated user.
func (s *ProfileService) List(ctx context.Context) ([]Profile, error) {
	var profiles []Profile
	if err := s.c.get(ctx, "/v1/profiles", nil, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

// Get returns a single profile by ID.
func (s *ProfileService) Get(ctx context.Context, profileID int64) (*Profile, error) {
	var p Profile
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d", profileID), nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreatePersonal creates a new personal profile.
func (s *ProfileService) CreatePersonal(ctx context.Context, req CreatePersonalRequest) (*Profile, error) {
	body := map[string]any{"type": ProfileTypePersonal, "details": req}
	var p Profile
	if err := s.c.post(ctx, "/v1/profiles", body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateBusiness creates a new business profile.
func (s *ProfileService) CreateBusiness(ctx context.Context, req CreateBusinessRequest) (*Profile, error) {
	body := map[string]any{"type": ProfileTypeBusiness, "details": req}
	var p Profile
	if err := s.c.post(ctx, "/v1/profiles", body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePersonal updates a personal profile.
func (s *ProfileService) UpdatePersonal(ctx context.Context, profileID int64, req UpdatePersonalRequest) (*Profile, error) {
	body := map[string]any{"type": ProfileTypePersonal, "details": req}
	var p Profile
	if err := s.c.put(ctx, fmt.Sprintf("/v1/profiles/%d", profileID), body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateBusiness updates a business profile.
func (s *ProfileService) UpdateBusiness(ctx context.Context, profileID int64, req CreateBusinessRequest) (*Profile, error) {
	body := map[string]any{"type": ProfileTypeBusiness, "details": req}
	var p Profile
	if err := s.c.put(ctx, fmt.Sprintf("/v1/profiles/%d/business", profileID), body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// =============================================================================
// Quote Service
// =============================================================================

// QuoteService provides rate-locking and fee calculation for transfers.
type QuoteService struct{ c *Client }

// CreateQuoteRequest is the body for creating a quote.
type CreateQuoteRequest struct {
	SourceCurrency string        `json:"sourceCurrency"`
	TargetCurrency string        `json:"targetCurrency"`
	SourceAmount   *float64      `json:"sourceAmount,omitempty"`
	TargetAmount   *float64      `json:"targetAmount,omitempty"`
	PayIn          PaymentMethod `json:"payIn,omitempty"`
	PayOut         PaymentMethod `json:"payOut,omitempty"`
	PreferredPayIn PaymentMethod `json:"preferredPayIn,omitempty"`
	TargetAccount  int64         `json:"targetAccount,omitempty"`
}

// UpdateQuoteRequest patches an existing quote.
type UpdateQuoteRequest struct {
	TargetAccount int64         `json:"targetAccount,omitempty"`
	PayIn         PaymentMethod `json:"payIn,omitempty"`
	PayOut        PaymentMethod `json:"payOut,omitempty"`
}

// Quote is the Wise API quote response.
type Quote struct {
	ID                 string          `json:"id"`
	SourceCurrency     string          `json:"sourceCurrency"`
	TargetCurrency     string          `json:"targetCurrency"`
	SourceAmount       float64         `json:"sourceAmount"`
	TargetAmount       float64         `json:"targetAmount"`
	Rate               float64         `json:"rate"`
	CreatedTime        Time            `json:"createdTime"`
	ExpirationTime     Time            `json:"expirationTime"`
	Status             string          `json:"status"`
	ProvidedAmountType string          `json:"providedAmountType"`
	PaymentOptions     []PaymentOption `json:"paymentOptions"`
	RateType           string          `json:"rateType"`
	TargetAccount      int64           `json:"targetAccount,omitempty"`
}

// PaymentOption describes one pay-in/pay-out combination available for a quote.
type PaymentOption struct {
	Disabled                   bool          `json:"disabled"`
	EstimatedDelivery          Time          `json:"estimatedDelivery"`
	FormattedEstimatedDelivery string        `json:"formattedEstimatedDelivery"`
	Fee                        PaymentFee    `json:"fee"`
	Price                      PaymentPrice  `json:"price"`
	SourceAmount               float64       `json:"sourceAmount"`
	TargetAmount               float64       `json:"targetAmount"`
	SourceCurrency             string        `json:"sourceCurrency"`
	TargetCurrency             string        `json:"targetCurrency"`
	PayIn                      PaymentMethod `json:"payIn"`
	PayOut                     PaymentMethod `json:"payOut"`
}

// PaymentFee breaks down the fee components for a payment option.
type PaymentFee struct {
	Transferwise   float64 `json:"transferwise"`
	PayIn          float64 `json:"payIn"`
	Discount       float64 `json:"discount"`
	PriceSetAmount float64 `json:"priceSetAmount"`
	Partner        float64 `json:"partner"`
	Total          float64 `json:"total"`
}

// PaymentPrice holds the total all-in pricing for a payment option.
type PaymentPrice struct {
	PriceSetAmount Amount `json:"priceSetAmount"`
	Fee            Amount `json:"fee"`
	Total          Amount `json:"total"`
}

// Create creates an authenticated quote for the given profile.
func (s *QuoteService) Create(ctx context.Context, profileID int64, req CreateQuoteRequest) (*Quote, error) {
	var q Quote
	if err := s.c.post(ctx, fmt.Sprintf("/v3/profiles/%d/quotes", profileID), req, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// CreateAnonymous creates an unauthenticated quote (rate preview only).
func (s *QuoteService) CreateAnonymous(ctx context.Context, req CreateQuoteRequest) (*Quote, error) {
	var q Quote
	if err := s.c.post(ctx, "/v1/quotes", req, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// Get retrieves an existing quote by ID.
func (s *QuoteService) Get(ctx context.Context, profileID int64, quoteID string) (*Quote, error) {
	var q Quote
	if err := s.c.get(ctx, fmt.Sprintf("/v3/profiles/%d/quotes/%s", profileID, quoteID), nil, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// Update patches an existing quote (e.g. to attach a recipient account).
func (s *QuoteService) Update(ctx context.Context, profileID int64, quoteID string, req UpdateQuoteRequest) (*Quote, error) {
	var q Quote
	if err := s.c.patch(ctx, fmt.Sprintf("/v3/profiles/%d/quotes/%s", profileID, quoteID), req, &q); err != nil {
		return nil, err
	}
	return &q, nil
}

// AccountRequirements returns the recipient account fields required for the quote.
func (s *QuoteService) AccountRequirements(ctx context.Context, profileID int64, quoteID string) ([]RequirementField, error) {
	var fields []RequirementField
	path := fmt.Sprintf("/v3/profiles/%d/quotes/%s/account-requirements", profileID, quoteID)
	if err := s.c.get(ctx, path, nil, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// RefreshAccountRequirements returns updated requirements after a field value is selected.
func (s *QuoteService) RefreshAccountRequirements(ctx context.Context, profileID int64, quoteID string, details map[string]any) ([]RequirementField, error) {
	var fields []RequirementField
	path := fmt.Sprintf("/v3/profiles/%d/quotes/%s/account-requirements", profileID, quoteID)
	if err := s.c.post(ctx, path, details, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// =============================================================================
// Recipient Service
// =============================================================================

// RecipientService manages beneficiary accounts for transfers.
type RecipientService struct{ c *Client }

// RecipientAccount represents a Wise recipient (beneficiary) account.
type RecipientAccount struct {
	ID                int64          `json:"id"`
	Profile           int64          `json:"profile"`
	AccountHolderName string         `json:"accountHolderName"`
	Currency          string         `json:"currency"`
	Country           string         `json:"country"`
	Type              string         `json:"type"`
	Details           map[string]any `json:"details"`
	Active            bool           `json:"active"`
	OwnedByCustomer   bool           `json:"ownedByCustomer"`
	CreatedAt         Time           `json:"createdAt"`
	UpdatedAt         Time           `json:"updatedAt"`
}

// CreateRecipientRequest is the body for creating a recipient account.
type CreateRecipientRequest struct {
	Profile           int64          `json:"profile"`
	AccountHolderName string         `json:"accountHolderName"`
	Currency          string         `json:"currency"`
	Type              string         `json:"type"`
	Details           map[string]any `json:"details"`
	OwnedByCustomer   bool           `json:"ownedByCustomer,omitempty"`
}

// ListRecipientsParams configures a recipient list query.
type ListRecipientsParams struct {
	PageParams
	ProfileID int64
	Currency  string
}

// Create creates a new recipient account.
func (s *RecipientService) Create(ctx context.Context, req CreateRecipientRequest) (*RecipientAccount, error) {
	var r RecipientAccount
	if err := s.c.post(ctx, "/v1/accounts", req, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Get returns a recipient account by ID.
func (s *RecipientService) Get(ctx context.Context, accountID int64) (*RecipientAccount, error) {
	var r RecipientAccount
	if err := s.c.get(ctx, fmt.Sprintf("/v1/accounts/%d", accountID), nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Delete deactivates a recipient account.
func (s *RecipientService) Delete(ctx context.Context, accountID int64) error {
	return s.c.delete(ctx, fmt.Sprintf("/v1/accounts/%d", accountID))
}

// List returns recipient accounts matching the given parameters.
func (s *RecipientService) List(ctx context.Context, p ListRecipientsParams) ([]RecipientAccount, error) {
	params := p.Values()
	if p.ProfileID > 0 {
		params.Set("profile", strconv.FormatInt(p.ProfileID, 10))
	}
	if p.Currency != "" {
		params.Set("currency", p.Currency)
	}
	var accounts []RecipientAccount
	if err := s.c.get(ctx, "/v1/accounts", params, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// AccountRequirements returns the fields required to create a recipient for the route.
func (s *RecipientService) AccountRequirements(ctx context.Context, sourceCurrency, targetCurrency string, sourceAmount float64) ([]RequirementField, error) {
	params := url.Values{
		"source":       {sourceCurrency},
		"target":       {targetCurrency},
		"sourceAmount": {fmt.Sprintf("%.2f", sourceAmount)},
	}
	var fields []RequirementField
	if err := s.c.get(ctx, "/v1/account-requirements", params, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// RefreshAccountRequirements re-fetches recipient fields after a select-type field
// changes. Call this whenever a RequirementGroup with RefreshRequirementsOnChange:true
// is updated, passing the currently-filled details so Wise can return dependent fields.
//
// POST /v1/account-requirements?source={src}&target={tgt}&sourceAmount={amt}.
func (s *RecipientService) RefreshAccountRequirements(ctx context.Context, sourceCurrency, targetCurrency string, sourceAmount float64, details map[string]any) ([]RequirementField, error) {
	params := url.Values{
		"source":       {sourceCurrency},
		"target":       {targetCurrency},
		"sourceAmount": {fmt.Sprintf("%.2f", sourceAmount)},
	}
	path := "/v1/account-requirements?" + params.Encode()
	var fields []RequirementField
	if err := s.c.post(ctx, path, details, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// =============================================================================
// Transfer Service
// =============================================================================

// TransferService manages payment transfers.
type TransferService struct{ c *Client }

// Transfer represents a Wise payment transfer.
type Transfer struct {
	ID                    int64           `json:"id"`
	UserID                int64           `json:"user"`
	TargetAccount         int64           `json:"targetAccount"`
	Quote                 int64           `json:"quote"`
	QuoteUUID             string          `json:"quoteUuid"`
	Status                TransferStatus  `json:"status"`
	Reference             string          `json:"reference"`
	Rate                  float64         `json:"rate"`
	Created               Time            `json:"created"`
	Details               TransferDetails `json:"details"`
	HasActiveIssues       bool            `json:"hasActiveIssues"`
	SourceCurrency        string          `json:"sourceCurrency"`
	SourceValue           float64         `json:"sourceValue"`
	TargetCurrency        string          `json:"targetCurrency"`
	TargetValue           float64         `json:"targetValue"`
	CustomerTransactionID string          `json:"customerTransactionId"`
}

// TransferDetails holds reference and purpose metadata for a transfer.
type TransferDetails struct {
	Reference       string `json:"reference"`
	TransferPurpose string `json:"transferPurpose,omitempty"`
	SourceOfFunds   string `json:"sourceOfFunds,omitempty"`
}

// CreateTransferRequest is the body for creating a transfer.
type CreateTransferRequest struct {
	TargetAccount         int64           `json:"targetAccount"`
	QuoteUUID             string          `json:"quoteUuid"`
	CustomerTransactionID string          `json:"customerTransactionId"`
	Details               TransferDetails `json:"details,omitempty"`
}

// ListTransfersParams configures a transfer list query.
type ListTransfersParams struct {
	PageParams
	ProfileID int64
	Status    TransferStatus
	Currency  string
	StartDate time.Time
	EndDate   time.Time
}

// FundResponse is returned after funding a transfer from a balance.
type FundResponse struct {
	Type              string `json:"type"`
	Status            string `json:"status"`
	ErrorCode         string `json:"errorCode,omitempty"`
	ExternalReference string `json:"externalReference,omitempty"`
}

// Create creates a new transfer.
func (s *TransferService) Create(ctx context.Context, req CreateTransferRequest) (*Transfer, error) {
	var t Transfer
	if err := s.c.post(ctx, "/v1/transfers", req, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Get retrieves a transfer by ID.
func (s *TransferService) Get(ctx context.Context, transferID int64) (*Transfer, error) {
	var t Transfer
	if err := s.c.get(ctx, fmt.Sprintf("/v1/transfers/%d", transferID), nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns transfers matching the given parameters.
func (s *TransferService) List(ctx context.Context, p ListTransfersParams) ([]Transfer, error) {
	params := p.Values()
	if p.ProfileID > 0 {
		params.Set("profile", strconv.FormatInt(p.ProfileID, 10))
	}
	if p.Status != "" {
		params.Set("status", string(p.Status))
	}
	if p.Currency != "" {
		params.Set("currency", p.Currency)
	}
	if !p.StartDate.IsZero() {
		params.Set("createdDateStart", p.StartDate.Format(time.RFC3339))
	}
	if !p.EndDate.IsZero() {
		params.Set("createdDateEnd", p.EndDate.Format(time.RFC3339))
	}
	var transfers []Transfer
	if err := s.c.get(ctx, "/v1/transfers", params, &transfers); err != nil {
		return nil, err
	}
	return transfers, nil
}

// Fund funds a transfer from a balance account. SCA-protected in EU/UK.
func (s *TransferService) Fund(ctx context.Context, profileID, transferID int64) (*FundResponse, error) {
	body := map[string]string{"type": "BALANCE"}
	var resp FundResponse
	if err := s.c.post(ctx, fmt.Sprintf("/v3/profiles/%d/transfers/%d/payments", profileID, transferID), body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Cancel cancels an unfunded transfer.
func (s *TransferService) Cancel(ctx context.Context, transferID int64) (*Transfer, error) {
	var t Transfer
	if err := s.c.post(ctx, fmt.Sprintf("/v1/transfers/%d/cancel", transferID), nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// DeliveryEstimate returns the expected arrival time for a funded transfer.
func (s *TransferService) DeliveryEstimate(ctx context.Context, transferID int64) (*DeliveryEstimate, error) {
	var est DeliveryEstimate
	if err := s.c.get(ctx, fmt.Sprintf("/v1/delivery-estimates/%d", transferID), nil, &est); err != nil {
		return nil, err
	}
	return &est, nil
}

// PayinDepositDetails returns the bank transfer funding instructions for a transfer.
type PayinDepositDetail struct {
	AccountHolderName string            `json:"accountHolderName"`
	BankCode          string            `json:"bankCode"`
	AccountNumber     string            `json:"accountNumber"`
	BankName          string            `json:"bankName"`
	BankAddress       string            `json:"bankAddress"`
	Reference         string            `json:"reference"`
	Details           map[string]string `json:"details"`
}

// PayinDepositDetails returns bank transfer funding details for a transfer.
func (s *TransferService) PayinDepositDetails(ctx context.Context, profileID, transferID int64) (*PayinDepositDetail, error) {
	var d PayinDepositDetail
	path := fmt.Sprintf("/v1/profiles/%d/transfers/%d/deposit-details/bank-transfer", profileID, transferID)
	if err := s.c.get(ctx, path, nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Requirements returns additional required fields for a transfer.
func (s *TransferService) Requirements(ctx context.Context, transferID int64) ([]RequirementField, error) {
	var fields []RequirementField
	if err := s.c.get(ctx, fmt.Sprintf("/v1/transfers/%d/requirements", transferID), nil, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// =============================================================================
// Balance Service
// =============================================================================

// BalanceService manages multi-currency balance accounts.
type BalanceService struct{ c *Client }

// Balance represents a single currency balance within a multi-currency account.
type Balance struct {
	ID               int64               `json:"id"`
	Currency         string              `json:"currency"`
	Type             BalanceType         `json:"type"`
	Name             string              `json:"name,omitempty"`
	Amount           Amount              `json:"amount"`
	ReservedAmount   Amount              `json:"reservedAmount"`
	BankDetails      *BalanceBankDetails `json:"bankDetails,omitempty"`
	InvestmentState  string              `json:"investmentState"`
	CreationTime     Time                `json:"creationTime"`
	ModificationTime Time                `json:"modificationTime"`
	Visible          bool                `json:"visible"`
}

// BalanceBankDetails is a lightweight summary of bank details for a balance.
type BalanceBankDetails struct {
	ID       int64  `json:"id"`
	Currency string `json:"currency"`
}

// CreateBalanceRequest is the body for opening a new balance account.
type CreateBalanceRequest struct {
	Currency string      `json:"currency"`
	Type     BalanceType `json:"type"`
	Name     string      `json:"name,omitempty"`
}

// MoveMoneyRequest is the body for converting or moving funds between balances.
type MoveMoneyRequest struct {
	QuoteID         string  `json:"quoteId,omitempty"`
	Amount          *Amount `json:"amount,omitempty"`
	SourceBalanceID int64   `json:"sourceBalanceId"`
	TargetBalanceID int64   `json:"targetBalanceId"`
}

// BalanceMovement is the result of a convert/move-money operation.
type BalanceMovement struct {
	Type  string `json:"type"`
	State string `json:"state"`
	Quote string `json:"quote,omitempty"`
}

// TotalFunds is the aggregate balance overview returned by GetTotalFunds.
type TotalFunds struct {
	Currency       string `json:"currency"`
	TotalWorth     Amount `json:"totalWorth"`
	TotalAvailable Amount `json:"totalAvailable"`
	TotalCash      Amount `json:"totalCash"`
	OverdraftUsage Amount `json:"overdraftUsage"`
	OverdraftLimit Amount `json:"overdraftLimit"`
}

// DepositLimits holds the regulatory deposit limit for a currency.
type DepositLimits struct {
	Currency string   `json:"currency"`
	Max      *float64 `json:"max,omitempty"`
}

// Create opens a new balance account for the profile.
func (s *BalanceService) Create(ctx context.Context, profileID int64, req CreateBalanceRequest) (*Balance, error) {
	var b Balance
	if err := s.c.post(ctx, fmt.Sprintf("/v4/profiles/%d/balances", profileID), req, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// List returns all balance accounts for the profile matching the given types.
func (s *BalanceService) List(ctx context.Context, profileID int64, types ...BalanceType) ([]Balance, error) {
	params := url.Values{}
	for _, t := range types {
		params.Add("types", string(t))
	}
	if len(types) == 0 {
		params.Set("types", "STANDARD,SAVINGS")
	}
	var balances []Balance
	if err := s.c.get(ctx, fmt.Sprintf("/v4/profiles/%d/balances", profileID), params, &balances); err != nil {
		return nil, err
	}
	return balances, nil
}

// Get retrieves a single balance by ID.
func (s *BalanceService) Get(ctx context.Context, profileID, balanceID int64) (*Balance, error) {
	var b Balance
	if err := s.c.get(ctx, fmt.Sprintf("/v4/profiles/%d/balances/%d", profileID, balanceID), nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Close deactivates a zero-balance account.
func (s *BalanceService) Close(ctx context.Context, profileID, balanceID int64) error {
	return s.c.delete(ctx, fmt.Sprintf("/v4/profiles/%d/balances/%d", profileID, balanceID))
}

// MoveMoney converts or moves funds between balances.
func (s *BalanceService) MoveMoney(ctx context.Context, profileID int64, req MoveMoneyRequest) (*BalanceMovement, error) {
	var m BalanceMovement
	if err := s.c.post(ctx, fmt.Sprintf("/v2/profiles/%d/balance-movements", profileID), req, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetTotalFunds returns the aggregate balance valuation in the given currency.
func (s *BalanceService) GetTotalFunds(ctx context.Context, profileID int64, currency string) (*TotalFunds, error) {
	var tf TotalFunds
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/total-funds/%s", profileID, currency), nil, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

// GetDepositLimits returns the regulatory deposit limits for the profile.
// Relevant for Singapore and Malaysia accounts.
func (s *BalanceService) GetDepositLimits(ctx context.Context, profileID int64) ([]DepositLimits, error) {
	var limits []DepositLimits
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/balance-capacity", profileID), nil, &limits); err != nil {
		return nil, err
	}
	return limits, nil
}

// SetExcessMoneyAccount configures where excess funds are auto-transferred when
// the regulatory hold limit is exceeded (Singapore and Malaysia customers).
func (s *BalanceService) SetExcessMoneyAccount(ctx context.Context, profileID, recipientID int64) error {
	body := map[string]int64{"recipientId": recipientID}
	return s.c.post(ctx, fmt.Sprintf("/v1/profiles/%d/excess-money-account", profileID), body, nil)
}

// =============================================================================
// Rate Service
// =============================================================================

// RateService provides current and historical exchange rates.
type RateService struct{ c *Client }

// GetRateParams configures an exchange rate request.
type GetRateParams struct {
	Source string
	Target string
}

// Get returns the current exchange rate for a currency pair.
// Returns ErrNotFound if no rate exists for the pair.
func (s *RateService) Get(ctx context.Context, p GetRateParams) (*ExchangeRate, error) {
	params := url.Values{}
	if p.Source != "" {
		params.Set("source", p.Source)
	}
	if p.Target != "" {
		params.Set("target", p.Target)
	}
	var rates []ExchangeRate
	if err := s.c.get(ctx, "/v1/rates", params, &rates); err != nil {
		return nil, err
	}
	if len(rates) == 0 {
		return nil, ErrNotFound
	}
	return &rates[0], nil
}

// List returns all exchange rates, optionally filtered by currency pair.
func (s *RateService) List(ctx context.Context, p GetRateParams) ([]ExchangeRate, error) {
	params := url.Values{}
	if p.Source != "" {
		params.Set("source", p.Source)
	}
	if p.Target != "" {
		params.Set("target", p.Target)
	}
	var rates []ExchangeRate
	if err := s.c.get(ctx, "/v1/rates", params, &rates); err != nil {
		return nil, err
	}
	return rates, nil
}

// =============================================================================
// Currency Service
// =============================================================================

// CurrencyService lists currencies supported for transfers.
type CurrencyService struct{ c *Client }

// List returns all currencies supported for Wise transfers.
func (s *CurrencyService) List(ctx context.Context) ([]Currency, error) {
	var currencies []Currency
	if err := s.c.get(ctx, "/v1/currencies", nil, &currencies); err != nil {
		return nil, err
	}
	return currencies, nil
}

// =============================================================================
// Activity Service
// =============================================================================

// ActivityService provides the profile activity log.
type ActivityService struct{ c *Client }

// Activity represents a single performed action for a profile.
type Activity struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CreatedAt   Time   `json:"createdAt"`
	UpdatedAt   Time   `json:"updatedAt"`
	Description string `json:"description,omitempty"`
}

// ListActivitiesParams configures an activity list query.
type ListActivitiesParams struct {
	PageParams
	Type      string
	Status    string
	StartDate string
	EndDate   string
}

// ListActivitiesResponse wraps the paginated activity list response.
type ListActivitiesResponse struct {
	Activities []Activity `json:"activities"`
	Total      int        `json:"total"`
}

// List returns activities for the given profile.
func (s *ActivityService) List(ctx context.Context, profileID int64, p ListActivitiesParams) (*ListActivitiesResponse, error) {
	params := p.Values()
	if p.Type != "" {
		params.Set("type", p.Type)
	}
	if p.Status != "" {
		params.Set("status", p.Status)
	}
	if p.StartDate != "" {
		params.Set("intervalStart", p.StartDate)
	}
	if p.EndDate != "" {
		params.Set("intervalEnd", p.EndDate)
	}
	var resp ListActivitiesResponse
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/activities", profileID), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// =============================================================================
// Comparison Service
// =============================================================================

// ComparisonService compares prices and speeds across money transfer providers.
type ComparisonService struct{ c *Client }

// ProviderQuote holds a single provider's price and speed estimate for a route.
type ProviderQuote struct {
	Provider       string  `json:"provider"`
	SendAmount     float64 `json:"sendAmount"`
	ReceiveAmount  float64 `json:"receiveAmount"`
	SourceCurrency string  `json:"sourceCurrency"`
	TargetCurrency string  `json:"targetCurrency"`
	Fee            float64 `json:"fee"`
	Rate           float64 `json:"rate"`
	Speed          *Speed  `json:"speed,omitempty"`
	PayInMethod    string  `json:"payInMethod"`
	PayOutMethod   string  `json:"payOutMethod"`
}

// Speed holds estimated delivery time range.
type Speed struct {
	MinHours *float64 `json:"minHours"`
	MaxHours *float64 `json:"maxHours"`
}

// ComparisonParams configures a comparison request.
type ComparisonParams struct {
	SourceCurrency string
	TargetCurrency string
	SendAmount     float64
	ReceiveAmount  float64
}

// Compare returns provider comparison data for the requested route and amount.
func (s *ComparisonService) Compare(ctx context.Context, p ComparisonParams) ([]ProviderQuote, error) {
	params := url.Values{}
	params.Set("sourceCurrency", p.SourceCurrency)
	params.Set("targetCurrency", p.TargetCurrency)
	if p.SendAmount > 0 {
		params.Set("sourceAmount", fmt.Sprintf("%.2f", p.SendAmount))
	}
	if p.ReceiveAmount > 0 {
		params.Set("targetAmount", fmt.Sprintf("%.2f", p.ReceiveAmount))
	}
	var quotes []ProviderQuote
	if err := s.c.get(ctx, "/v4/comparisons", params, &quotes); err != nil {
		return nil, err
	}
	return quotes, nil
}

// =============================================================================
// Address Service
// =============================================================================

// AddressService manages physical addresses with country-specific fields.
type AddressService struct{ c *Client }

// CreateAddressRequest is the body for creating or updating an address.
type CreateAddressRequest struct {
	Profile    int64  `json:"profile"`
	Country    string `json:"country"`
	City       string `json:"city"`
	PostCode   string `json:"postCode"`
	FirstLine  string `json:"firstLine"`
	SecondLine string `json:"secondLine,omitempty"`
	State      string `json:"state,omitempty"`
}

// Create creates a new address record.
func (s *AddressService) Create(ctx context.Context, req CreateAddressRequest) (*Address, error) {
	var a Address
	if err := s.c.post(ctx, "/v1/addresses", req, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// List returns all addresses for the given profile.
func (s *AddressService) List(ctx context.Context, profileID int64) ([]Address, error) {
	params := url.Values{"profile": {strconv.FormatInt(profileID, 10)}}
	var addrs []Address
	if err := s.c.get(ctx, "/v1/addresses", params, &addrs); err != nil {
		return nil, err
	}
	return addrs, nil
}

// Get returns an address by ID.
func (s *AddressService) Get(ctx context.Context, addressID int64) (*Address, error) {
	var a Address
	if err := s.c.get(ctx, fmt.Sprintf("/v1/addresses/%d", addressID), nil, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Requirements returns the fields required to create a valid address for the given country.
func (s *AddressService) Requirements(ctx context.Context, country string) ([]RequirementField, error) {
	params := url.Values{}
	if country != "" {
		params.Set("country", country)
	}
	var fields []RequirementField
	if err := s.c.get(ctx, "/v1/address-requirements", params, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// =============================================================================
// Direct Debit Account Service
// =============================================================================

// DirectDebitAccountService manages external bank accounts for ACH/EFT batch funding.
type DirectDebitAccountService struct{ c *Client }

// DirectDebitAccount is an external bank account registered for direct debit.
type DirectDebitAccount struct {
	ID                string `json:"id"`
	AccountHolderName string `json:"accountHolderName"`
	Currency          string `json:"currency"`
	Country           string `json:"country"`
	AccountNumber     string `json:"accountNumber,omitempty"`
	RoutingNumber     string `json:"routingNumber,omitempty"`
	IBAN              string `json:"iban,omitempty"`
	Status            string `json:"status"`
}

// CreateDirectDebitRequest is the body for registering a direct debit account.
type CreateDirectDebitRequest struct {
	AccountHolderName string `json:"accountHolderName"`
	Currency          string `json:"currency"`
	Country           string `json:"country"`
	AccountNumber     string `json:"accountNumber,omitempty"`
	RoutingNumber     string `json:"routingNumber,omitempty"`
	IBAN              string `json:"iban,omitempty"`
}

// Create registers an external bank account for direct debit funding.
func (s *DirectDebitAccountService) Create(ctx context.Context, profileID int64, req CreateDirectDebitRequest) (*DirectDebitAccount, error) {
	var a DirectDebitAccount
	if err := s.c.post(ctx, fmt.Sprintf("/v1/profiles/%d/direct-debit-accounts", profileID), req, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// List returns all direct debit accounts for the profile.
func (s *DirectDebitAccountService) List(ctx context.Context, profileID int64) ([]DirectDebitAccount, error) {
	var accounts []DirectDebitAccount
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/direct-debit-accounts", profileID), nil, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// =============================================================================
// Batch Group Service
// =============================================================================

// BatchGroupService manages batch groups of up to 1,000 transfers.
type BatchGroupService struct{ c *Client }

// BatchGroup is a named collection of transfers funded together.
type BatchGroup struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	SourceCurrency string             `json:"sourceCurrency"`
	Status         BatchGroupStatus   `json:"status"`
	TotalValue     float64            `json:"totalValue"`
	TransferCount  int                `json:"transferCount"`
	Version        int                `json:"version"`
	CreatedAt      Time               `json:"createdAt"`
	UpdatedAt      Time               `json:"updatedAt"`
	PayInDetails   *BatchPayInDetails `json:"payInDetails,omitempty"`
}

// BatchPayInDetails holds the funding instructions for a completed batch group.
type BatchPayInDetails struct {
	Reference   string            `json:"reference"`
	TotalAmount Amount            `json:"totalAmount"`
	BankDetails map[string]string `json:"bankDetails"`
}

// CreateBatchGroupRequest is the body for creating a batch group.
type CreateBatchGroupRequest struct {
	Name           string `json:"name"`
	SourceCurrency string `json:"sourceCurrency"`
}

// PaymentInitiation is the response from initiating a direct debit batch payment.
type PaymentInitiation struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	EstimatedArrival Time   `json:"estimatedArrival"`
}

// Create creates a new batch group.
func (s *BatchGroupService) Create(ctx context.Context, profileID int64, req CreateBatchGroupRequest) (*BatchGroup, error) {
	var bg BatchGroup
	if err := s.c.post(ctx, fmt.Sprintf("/v3/profiles/%d/batch-groups", profileID), req, &bg); err != nil {
		return nil, err
	}
	return &bg, nil
}

// Get retrieves a batch group by ID.
func (s *BatchGroupService) Get(ctx context.Context, profileID int64, batchGroupID string) (*BatchGroup, error) {
	var bg BatchGroup
	if err := s.c.get(ctx, fmt.Sprintf("/v3/profiles/%d/batch-groups/%s", profileID, batchGroupID), nil, &bg); err != nil {
		return nil, err
	}
	return &bg, nil
}

// Complete marks the batch group as COMPLETED and populates pay-in funding instructions.
func (s *BatchGroupService) Complete(ctx context.Context, profileID int64, batchGroupID string, version int) (*BatchGroup, error) {
	body := map[string]any{"status": "COMPLETED", "version": version}
	var bg BatchGroup
	if err := s.c.patch(ctx, fmt.Sprintf("/v3/profiles/%d/batch-groups/%s", profileID, batchGroupID), body, &bg); err != nil {
		return nil, err
	}
	return &bg, nil
}

// Cancel cancels a batch group and all unfunded transfers within it.
func (s *BatchGroupService) Cancel(ctx context.Context, profileID int64, batchGroupID string, version int) (*BatchGroup, error) {
	body := map[string]any{"status": "CANCELED", "version": version}
	var bg BatchGroup
	if err := s.c.patch(ctx, fmt.Sprintf("/v3/profiles/%d/batch-groups/%s", profileID, batchGroupID), body, &bg); err != nil {
		return nil, err
	}
	return &bg, nil
}

// AddTransfer adds a transfer to the batch group.
func (s *BatchGroupService) AddTransfer(ctx context.Context, profileID int64, batchGroupID string, req CreateTransferRequest) (*Transfer, error) {
	var t Transfer
	path := fmt.Sprintf("/v3/profiles/%d/batch-groups/%s/transfers", profileID, batchGroupID)
	if err := s.c.post(ctx, path, req, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Fund funds all transfers in a completed batch group from a balance. SCA-protected.
func (s *BatchGroupService) Fund(ctx context.Context, profileID int64, batchGroupID string) error {
	path := fmt.Sprintf("/v3/profiles/%d/batch-payments/%s/payments", profileID, batchGroupID)
	return s.c.post(ctx, path, map[string]string{"type": "BALANCE"}, nil)
}

// FundViaDirectDebit funds a batch via ACH or EFT direct debit. SCA-protected.
func (s *BatchGroupService) FundViaDirectDebit(ctx context.Context, profileID int64, batchGroupID, directDebitAccountID string) (*PaymentInitiation, error) {
	body := map[string]string{"directDebitAccountId": directDebitAccountID}
	var pi PaymentInitiation
	path := fmt.Sprintf("/v1/profiles/%d/batch-groups/%s/payment-initiations", profileID, batchGroupID)
	if err := s.c.post(ctx, path, body, &pi); err != nil {
		return nil, err
	}
	return &pi, nil
}

// GetPaymentInitiation retrieves a payment initiation by ID.
func (s *BatchGroupService) GetPaymentInitiation(ctx context.Context, profileID int64, batchGroupID, paymentInitiationID string) (*PaymentInitiation, error) {
	var pi PaymentInitiation
	path := fmt.Sprintf("/v1/profiles/%d/batch-groups/%s/payment-initiations/%s", profileID, batchGroupID, paymentInitiationID)
	if err := s.c.get(ctx, path, nil, &pi); err != nil {
		return nil, err
	}
	return &pi, nil
}

// SendSettlementJournal submits a bulk settlement journal to Wise.
// Requires a client credentials token.
func (s *BatchGroupService) SendSettlementJournal(ctx context.Context, journal BulkSettlementJournal) error {
	return s.c.post(ctx, "/v1/settlements", journal, nil)
}

// BulkSettlementJournal is the body for a bulk settlement request.
type BulkSettlementJournal struct {
	Reference          string               `json:"reference"`
	SettlementCurrency string               `json:"settlementCurrency,omitempty"`
	Transfers          []SettlementTransfer `json:"transfers"`
}

// SettlementTransfer is a single transfer in a settlement journal.
type SettlementTransfer struct {
	ID           int64   `json:"id"`
	ExchangeRate float64 `json:"exchangeRate,omitempty"`
}

// =============================================================================
// Balance Statement Service
// =============================================================================

// StatementService provides balance statement downloads in multiple formats.
type StatementService struct{ c *Client }

// StatementParams configures a balance statement request.
type StatementParams struct {
	IntervalStart string
	IntervalEnd   string
	Type          string
}

// Statement is the JSON-format balance statement response.
type Statement struct {
	AccountHolder         StatementAccountHolder `json:"accountHolder"`
	Issuer                StatementIssuer        `json:"issuer"`
	Transactions          []StatementEntry       `json:"transactions"`
	EndOfStatementBalance Amount                 `json:"endOfStatementBalance"`
	Query                 StatementQuery         `json:"query"`
}

// StatementAccountHolder holds account owner details in a statement.
type StatementAccountHolder struct {
	Type    string  `json:"type"`
	Address Address `json:"address"`
}

// StatementIssuer is the Wise entity that issued the statement.
type StatementIssuer struct {
	Name     string `json:"name"`
	City     string `json:"city"`
	PostCode string `json:"postCode"`
	Country  string `json:"country"`
}

// StatementQuery reflects the parameters used to generate the statement.
type StatementQuery struct {
	IntervalStart Time   `json:"intervalStart"`
	IntervalEnd   Time   `json:"intervalEnd"`
	Currency      string `json:"currency"`
	AccountID     int64  `json:"accountId"`
}

// StatementEntry is a single transaction line in a JSON statement.
type StatementEntry struct {
	Type            string           `json:"type"`
	Date            Time             `json:"date"`
	Amount          Amount           `json:"amount"`
	TotalFees       Amount           `json:"totalFees"`
	Details         StatementDetails `json:"details"`
	RunningBalance  Amount           `json:"runningBalance"`
	ReferenceNumber string           `json:"referenceNumber"`
}

// StatementDetails provides context for a statement entry.
type StatementDetails struct {
	Type             string  `json:"type"`
	Description      string  `json:"description"`
	Amount           Amount  `json:"amount"`
	SourceAmount     *Amount `json:"sourceAmount,omitempty"`
	TargetAmount     *Amount `json:"targetAmount,omitempty"`
	Fee              *Amount `json:"fee,omitempty"`
	TransferID       *int64  `json:"transferId,omitempty"`
	PaymentReference string  `json:"paymentReference,omitempty"`
}

// GetJSON returns a structured JSON statement for a balance. SCA-protected in EU/UK.
func (s *StatementService) GetJSON(ctx context.Context, profileID, balanceID int64, p StatementParams) (*Statement, error) {
	params := url.Values{}
	if p.IntervalStart != "" {
		params.Set("intervalStart", p.IntervalStart)
	}
	if p.IntervalEnd != "" {
		params.Set("intervalEnd", p.IntervalEnd)
	}
	if p.Type != "" {
		params.Set("type", p.Type)
	}
	var stmt Statement
	path := fmt.Sprintf("/v1/profiles/%d/balance-statements/%d/statement.json", profileID, balanceID)
	if err := s.c.get(ctx, path, params, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

// GetRaw downloads a statement in a binary format (CSV, PDF, XLSX, etc.). SCA-protected.
func (s *StatementService) GetRaw(ctx context.Context, profileID, balanceID int64, format StatementFormat, p StatementParams) ([]byte, error) {
	params := url.Values{}
	if p.IntervalStart != "" {
		params.Set("intervalStart", p.IntervalStart)
	}
	if p.IntervalEnd != "" {
		params.Set("intervalEnd", p.IntervalEnd)
	}
	extMap := map[StatementFormat]string{
		StatementFormatCSV:   "csv",
		StatementFormatPDF:   "pdf",
		StatementFormatXLSX:  "xlsx",
		StatementFormatCAMT:  "xml",
		StatementFormatMT940: "mt940",
		StatementFormatQIF:   "qif",
	}
	ext, ok := extMap[format]
	if !ok {
		return nil, fmt.Errorf("wise: unsupported statement format %q", format)
	}
	path := fmt.Sprintf("/v1/profiles/%d/balance-statements/%d/statement.%s", profileID, balanceID, ext)
	return s.c.getRaw(ctx, path, params)
}

// =============================================================================
// Bank Account Details Service
// =============================================================================

// BankAccountService manages receive-money bank account details.
type BankAccountService struct{ c *Client }

// BankAccountDetail is a bank account detail entry for a balance.
type BankAccountDetail struct {
	ID        int64             `json:"id"`
	Currency  string            `json:"currency"`
	Country   string            `json:"country,omitempty"`
	Type      string            `json:"type"`
	IsExample bool              `json:"isExample"`
	Status    string            `json:"status"`
	Details   map[string]string `json:"details"`
}

// BankAccountDetailOrder is an order to issue bank account details.
type BankAccountDetailOrder struct {
	ID       string `json:"id"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

// CreateOrder creates an order to issue bank account details for a currency.
func (s *BankAccountService) CreateOrder(ctx context.Context, profileID int64, currency string) (*BankAccountDetailOrder, error) {
	body := map[string]string{"currency": currency}
	var order BankAccountDetailOrder
	if err := s.c.post(ctx, fmt.Sprintf("/v1/profiles/%d/account-details-orders", profileID), body, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// List returns all available bank account details for the profile.
func (s *BankAccountService) List(ctx context.Context, profileID int64) ([]BankAccountDetail, error) {
	var details []BankAccountDetail
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/account-details", profileID), nil, &details); err != nil {
		return nil, err
	}
	return details, nil
}

// ListOrders returns all bank account detail orders for the profile.
func (s *BankAccountService) ListOrders(ctx context.Context, profileID int64) ([]BankAccountDetailOrder, error) {
	var orders []BankAccountDetailOrder
	if err := s.c.get(ctx, fmt.Sprintf("/v3/profiles/%d/account-details-orders", profileID), nil, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

// CreatePaymentReturn creates a return for an inbound payment.
func (s *BankAccountService) CreatePaymentReturn(ctx context.Context, profileID int64, paymentID, reason string) error {
	body := map[string]string{"reason": reason}
	path := fmt.Sprintf("/v1/profiles/%d/account-details/payments/%s/returns", profileID, paymentID)
	return s.c.post(ctx, path, body, nil)
}

// =============================================================================
// Simulation Service (sandbox only)
// =============================================================================

// SimulationService provides sandbox-only endpoints for advancing transfer and
// card states without real funds or physical card issuance.
type SimulationService struct{ c *Client }

// SimulateTransferState is a valid target state for transfer simulation.
type SimulateTransferState string

const (
	// SimulateProcessing advances the transfer to the processing state.
	SimulateProcessing SimulateTransferState = "processing"

	// SimulateFundsConverted advances the transfer to funds_converted.
	SimulateFundsConverted SimulateTransferState = "funds_converted"

	// SimulateOutgoingPaymentSent advances the transfer to outgoing_payment_sent.
	SimulateOutgoingPaymentSent SimulateTransferState = "outgoing_payment_sent"

	// SimulateBouncedBack advances the transfer to bounced_back.
	SimulateBouncedBack SimulateTransferState = "bounced_back"

	// SimulateFundsRefunded advances the transfer to funds_refunded.
	SimulateFundsRefunded SimulateTransferState = "funds_refunded"
)

// CardProductionState is a target state for kiosk card production simulation.
type CardProductionState string

const (
	// SimulateCardProductionReady sets the card production status to READY.
	SimulateCardProductionReady CardProductionState = "READY"

	// SimulateCardProductionInProgress sets the status to IN_PROGRESS.
	SimulateCardProductionInProgress CardProductionState = "IN_PROGRESS"

	// SimulateCardProductionProduced sets the status to PRODUCED.
	SimulateCardProductionProduced CardProductionState = "PRODUCED"

	// SimulateCardProductionError sets the status to PRODUCTION_ERROR.
	SimulateCardProductionError CardProductionState = "PRODUCTION_ERROR"
)

// SimulateCardProductionRequest configures a card production simulation.
type SimulateCardProductionRequest struct {
	Status    CardProductionState `json:"status"`
	ErrorCode string              `json:"errorCode,omitempty"`
}

// AdvanceTransfer moves a sandbox transfer to the given simulation state.
func (s *SimulationService) AdvanceTransfer(ctx context.Context, transferID int64, state SimulateTransferState) (*Transfer, error) {
	var t Transfer
	path := fmt.Sprintf("/v1/simulation/transfers/%d/%s", transferID, state)
	if err := s.c.post(ctx, path, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// SimulateCardProduction advances a card's kiosk production status in the sandbox.
func (s *SimulationService) SimulateCardProduction(ctx context.Context, profileID int64, cardToken string, req SimulateCardProductionRequest) error {
	path := fmt.Sprintf("/v3/spend/profiles/%d/simulation/card-production/%s", profileID, cardToken)
	return s.c.post(ctx, path, req, nil)
}

// SimulateIncomingPayment credits a sandbox balance with simulated inbound funds.
func (s *SimulationService) SimulateIncomingPayment(ctx context.Context, profileID, balanceID int64, amount Amount) error {
	body := map[string]any{"profileId": profileID, "balanceId": balanceID, "amount": amount}
	return s.c.post(ctx, "/v1/simulation/balance/topup", body, nil)
}

// RefreshRequirements calls the POST /v1/address-requirements endpoint to
// discover additional required fields based on previously selected field values.
// Use when a field has refreshRequirementsOnChange: true.
//
//	// After user selects country=US, discover additional fields:
//	fields, err := client.Addresses.RefreshRequirements(ctx, map[string]any{
//	    "details": map[string]string{"country": "US"},
//	})
func (s *AddressService) RefreshRequirements(ctx context.Context, details map[string]any) ([]RequirementField, error) {
	var fields []RequirementField
	if err := s.c.post(ctx, "/v1/address-requirements", details, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// CreateMultipleBankDetails creates a pair of local and international bank
// account details linked to a specific balance.
// Contact Wise Support to obtain access to this endpoint.
//
//	POST /v3/profiles/{profileId}/bank-details
func (s *BankAccountService) CreateMultipleBankDetails(ctx context.Context, profileID, balanceID int64) ([]BankAccountDetail, error) {
	body := map[string]int64{"balanceId": balanceID}
	var details []BankAccountDetail
	if err := s.c.post(ctx, fmt.Sprintf("/v3/profiles/%d/bank-details", profileID), body, &details); err != nil {
		return nil, err
	}
	return details, nil
}
