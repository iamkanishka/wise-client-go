package wise

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// -----------------------------------------------------------------------------
// Environments
// -----------------------------------------------------------------------------

// Environment selects the Wise API environment.
type Environment string

const (
	// Production is the live Wise API environment.
	Production Environment = "production"

	// Sandbox is the Wise Sandbox environment for integration testing.
	Sandbox Environment = "sandbox"
)

// -----------------------------------------------------------------------------
// Authentication
// -----------------------------------------------------------------------------

// AuthMode describes how the client authenticates requests.
type AuthMode int

const (
	// AuthPersonalToken uses a Wise Personal API Token.
	AuthPersonalToken AuthMode = iota

	// AuthClientCredentials uses OAuth 2.0 client_credentials grant.
	AuthClientCredentials

	// AuthUserToken uses an OAuth 2.0 bearer token with optional auto-refresh.
	AuthUserToken
)

// -----------------------------------------------------------------------------
// Pagination
// -----------------------------------------------------------------------------

// PageParams configures a paginated list request.
type PageParams struct {
	// Limit is the maximum number of items per page (default varies by endpoint).
	Limit int

	// Offset is the zero-based start index for offset-based pagination.
	Offset int

	// Cursor is the cursor value for cursor-based pagination.
	Cursor string
}

// Values encodes PageParams into URL query parameters.
func (p PageParams) Values() url.Values {
	v := url.Values{}

	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}

	if p.Offset > 0 {
		v.Set("offset", strconv.Itoa(p.Offset))
	}

	if p.Cursor != "" {
		v.Set("cursor", p.Cursor)
	}

	return v
}

// -----------------------------------------------------------------------------
// Money
// -----------------------------------------------------------------------------

// Amount represents a monetary value with a currency code.
type Amount struct {
	// Value is the numeric amount.
	Value float64 `json:"value"`

	// Currency is the ISO 4217 three-letter currency code (e.g. "USD", "GBP").
	Currency string `json:"currency"`
}

// String implements fmt.Stringer. Example output: "USD 123.4560".
func (a Amount) String() string {
	return fmt.Sprintf("%s %.4f", a.Currency, a.Value)
}

// -----------------------------------------------------------------------------
// Time
// -----------------------------------------------------------------------------

// Time wraps time.Time to handle the several date-time formats the Wise API returns.
type Time struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler.
// It handles RFC 3339 with/without nanoseconds, the ".000Z" millisecond form,
// and plain date strings (YYYY-MM-DD).
func (t *Time) UnmarshalJSON(b []byte) error {
	s := ""
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("wise: unmarshal time: %w", err)
	}

	if s == "" || s == "null" {
		return nil
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, f := range formats {
		if parsed, err := time.Parse(f, s); err == nil {
			t.Time = parsed

			return nil
		}
	}

	return fmt.Errorf("wise: cannot parse time %q", s)
}

// MarshalJSON implements json.Marshaler.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}

	return json.Marshal(t.UTC().Format(time.RFC3339Nano)) //nolint:wrapcheck // stdlib
}

// -----------------------------------------------------------------------------
// Profile types
// -----------------------------------------------------------------------------

// ProfileType distinguishes personal from business profiles.
type ProfileType string

const (
	// ProfileTypePersonal is a personal (individual) Wise profile.
	ProfileTypePersonal ProfileType = "personal"

	// ProfileTypeBusiness is a business Wise profile.
	ProfileTypeBusiness ProfileType = "business"
)

// Profile represents a Wise user profile.
type Profile struct {
	// ID is the unique numeric profile identifier.
	ID int64 `json:"id"`

	// Type is "personal" or "business".
	Type ProfileType `json:"type"`

	// Details holds personal profile fields (when Type == ProfileTypePersonal).
	Details *PersonalDetails `json:"details,omitempty"`

	// Business holds business profile fields (when Type == ProfileTypeBusiness).
	Business *BusinessDetails `json:"fullName,omitempty"`
}

// PersonalDetails holds personal profile data.
type PersonalDetails struct {
	// FirstName is the profile holder's given name.
	FirstName string `json:"firstName"`

	// LastName is the profile holder's family name.
	LastName string `json:"lastName"`

	// DateOfBirth is formatted YYYY-MM-DD.
	DateOfBirth string `json:"dateOfBirth"`

	// PhoneNumber is the E.164-formatted phone number.
	PhoneNumber string `json:"phoneNumber"`

	// Avatar is the URL of the profile avatar image.
	Avatar string `json:"avatar"`

	// Occupation is the profile holder's occupation code.
	Occupation string `json:"occupation"`

	// PrimaryAddress is the ID of the primary address record.
	PrimaryAddress int64 `json:"primaryAddress"`
}

// BusinessDetails holds business profile data.
type BusinessDetails struct {
	// Name is the registered business name.
	Name string `json:"name"`

	// RegistrationNumber is the official company registration number.
	RegistrationNumber string `json:"registrationNumber"`

	// Category is the broad business category.
	Category string `json:"category"`

	// SubCategory is the business sub-category.
	SubCategory string `json:"subCategory"`

	// BusinessType is the legal entity type (e.g. "LIMITED").
	BusinessType string `json:"businessType"`

	// CompanyRole is the role of the submitter in the company.
	CompanyRole string `json:"companyRole"`

	// DescriptionOfBusiness is a short description of business activities.
	DescriptionOfBusiness string `json:"descriptionOfBusiness"`

	// Webpage is the company website URL.
	Webpage string `json:"webpage"`

	// PrimaryAddress is the ID of the primary business address record.
	PrimaryAddress int64 `json:"primaryAddress"`

	// AverageMonthlyPayments is the expected monthly payment volume in USD.
	AverageMonthlyPayments float64 `json:"averageMonthlyPayments"`

	// Directors lists the company directors.
	Directors []Director `json:"directors,omitempty"`

	// Shareholders lists the company shareholders.
	Shareholders []Shareholder `json:"shareholders,omitempty"`
}

// Director represents a company director.
type Director struct {
	// FirstName is the director's given name.
	FirstName string `json:"firstName"`

	// LastName is the director's family name.
	LastName string `json:"lastName"`

	// DateOfBirth is formatted YYYY-MM-DD.
	DateOfBirth string `json:"dateOfBirth"`
}

// Shareholder represents a company shareholder.
type Shareholder struct {
	// FirstName is the shareholder's given name.
	FirstName string `json:"firstName"`

	// LastName is the shareholder's family name.
	LastName string `json:"lastName"`

	// DateOfBirth is formatted YYYY-MM-DD.
	DateOfBirth string `json:"dateOfBirth"`

	// SharesPercentage is the percentage of shares held (0–100).
	SharesPercentage float64 `json:"sharesPercentage"`
}

// -----------------------------------------------------------------------------
// Transfer
// -----------------------------------------------------------------------------

// TransferStatus describes the lifecycle state of a transfer.
type TransferStatus string

const (
	// TransferStatusDraft means the transfer has been created but not funded.
	TransferStatusDraft TransferStatus = "draft"

	// TransferStatusPendingCustomerInput means customer action is required.
	TransferStatusPendingCustomerInput TransferStatus = "pending_customer_input"

	// TransferStatusProcessing means the transfer is being processed.
	TransferStatusProcessing TransferStatus = "processing"

	// TransferStatusFundsConverted means the source funds have been converted.
	TransferStatusFundsConverted TransferStatus = "funds_converted"

	// TransferStatusOutgoingPaymentSent means the transfer has been sent.
	TransferStatusOutgoingPaymentSent TransferStatus = "outgoing_payment_sent"

	// TransferStatusCanceled means the transfer has been canceled.
	TransferStatusCanceled TransferStatus = "canceled"

	// TransferStatusFundsRefunded means the funds have been refunded.
	TransferStatusFundsRefunded TransferStatus = "funds_refunded"

	// TransferStatusBouncedBack means the transfer was bounced back by the recipient bank.
	TransferStatusBouncedBack TransferStatus = "bounced_back"

	// TransferStatusChargedBack means the transfer was charged back.
	TransferStatusChargedBack TransferStatus = "charged_back"
)

// -----------------------------------------------------------------------------
// Balance
// -----------------------------------------------------------------------------

// BalanceType distinguishes standard from savings (Jar) balances.
type BalanceType string

const (
	// BalanceTypeStandard is a regular currency balance (one per currency).
	BalanceTypeStandard BalanceType = "STANDARD"

	// BalanceSavings is a savings Jar balance (multiple per currency allowed).
	BalanceSavings BalanceType = "SAVINGS"
)

// -----------------------------------------------------------------------------
// Card
// -----------------------------------------------------------------------------

// CardStatus is the lifecycle state of a Wise card.
type CardStatus string

const (
	// CardStatusActive means the card is active and usable.
	CardStatusActive CardStatus = "ACTIVE"

	// CardStatusInactive means the card has not been activated yet.
	CardStatusInactive CardStatus = "INACTIVE"

	// CardStatusFrozen means the card is temporarily frozen.
	CardStatusFrozen CardStatus = "FROZEN"

	// CardStatusBlocked means the card is permanently blocked.
	CardStatusBlocked CardStatus = "BLOCKED"
)

// CardType distinguishes physical from virtual cards.
type CardType string

const (
	// CardTypePhysical is a physical payment card.
	CardTypePhysical CardType = "PHYSICAL"

	// CardTypeVirtual is a virtual (digital-only) card.
	CardTypeVirtual CardType = "VIRTUAL"
)

// -----------------------------------------------------------------------------
// Batch Group
// -----------------------------------------------------------------------------

// BatchGroupStatus describes the state of a batch group.
type BatchGroupStatus string

const (
	// BatchGroupNew means the batch group was just created.
	BatchGroupNew BatchGroupStatus = "NEW"

	// BatchGroupCompleted means the batch is closed for modifications.
	BatchGroupCompleted BatchGroupStatus = "COMPLETED"

	// BatchGroupFunded means all transfers in the batch have been funded.
	BatchGroupFunded BatchGroupStatus = "FUNDED"

	// BatchGroupCanceled means the batch and all unfunded transfers were canceled.
	BatchGroupCanceled BatchGroupStatus = "CANCELED"
)

// -----------------------------------------------------------------------------
// Statement
// -----------------------------------------------------------------------------

// StatementFormat is the file format for a balance statement download.
type StatementFormat string

const (
	// StatementFormatJSON returns a structured JSON statement.
	StatementFormatJSON StatementFormat = "json"

	// StatementFormatCSV returns a CSV spreadsheet.
	StatementFormatCSV StatementFormat = "csv"

	// StatementFormatPDF returns a branded PDF statement.
	StatementFormatPDF StatementFormat = "pdf"

	// StatementFormatXLSX returns an Excel spreadsheet.
	StatementFormatXLSX StatementFormat = "xlsx"

	// StatementFormatCAMT returns a CAMT.053 XML statement.
	StatementFormatCAMT StatementFormat = "xml"

	// StatementFormatMT940 returns an MT940 bank statement.
	StatementFormatMT940 StatementFormat = "mt940"

	// StatementFormatQIF returns a QIF (Quicken Interchange Format) statement.
	StatementFormatQIF StatementFormat = "qif"
)

// -----------------------------------------------------------------------------
// Webhook
// -----------------------------------------------------------------------------

// WebhookEventType identifies the type of event delivered to a webhook.
type WebhookEventType string

const (
	// EventTransferStateChange fires when a transfer changes status.
	EventTransferStateChange WebhookEventType = "transfers#state-change"

	// EventTransferActiveDays fires for active transfer cases.
	EventTransferActiveDays WebhookEventType = "transfers#active-cases"

	// EventBalanceCredit fires when a balance receives funds.
	EventBalanceCredit WebhookEventType = "balances#credit"

	// EventBalanceDebit fires when funds leave a balance.
	EventBalanceDebit WebhookEventType = "balances#debit"

	// EventCardTransactionCreated fires when a card transaction is created.
	EventCardTransactionCreated WebhookEventType = "cards#transaction-created"

	// EventCardTransactionUpdated fires when a card transaction is updated.
	EventCardTransactionUpdated WebhookEventType = "cards#transaction-updated"

	// EventCardProductionStatus fires on kiosk card production status changes.
	EventCardProductionStatus WebhookEventType = "cards#card-production-status-change"

	// EventProfileVerification fires on profile verification state changes.
	EventProfileVerification WebhookEventType = "profiles#verification-state-change"
)

// -----------------------------------------------------------------------------
// Dispute
// -----------------------------------------------------------------------------

// DisputeSubStatus represents the lifecycle state of a card transaction dispute.
type DisputeSubStatus string

const (
	// DisputeSubmitted is the initial dispute state.
	DisputeSubmitted DisputeSubStatus = "SUBMITTED"

	// DisputeInReview means the dispute is under review.
	DisputeInReview DisputeSubStatus = "IN_REVIEW"

	// DisputeRefunded means the refund has been processed.
	DisputeRefunded DisputeSubStatus = "REFUNDED"

	// DisputeRejected means the dispute was found to be invalid.
	DisputeRejected DisputeSubStatus = "REJECTED"

	// DisputeWithdrawn means the customer withdrew the dispute.
	DisputeWithdrawn DisputeSubStatus = "WITHDRAWN"

	// DisputeConfirmed means the dispute was reviewed but a refund is not applicable.
	DisputeConfirmed DisputeSubStatus = "CONFIRMED"

	// DisputeRefundInProgress means a refund is currently being processed.
	DisputeRefundInProgress DisputeSubStatus = "REFUND_IN_PROGRESS"

	// DisputeAttemptingRecovery means a chargeback request has been submitted.
	DisputeAttemptingRecovery DisputeSubStatus = "ATTEMPTING_RECOVERY"

	// DisputeRecoveryUnsuccessful means the chargeback attempt was unsuccessful.
	DisputeRecoveryUnsuccessful DisputeSubStatus = "RECOVERY_UNSUCCESSFUL"
)

// -----------------------------------------------------------------------------
// Shared supporting types
// -----------------------------------------------------------------------------

// Currency holds details of a supported currency.
type Currency struct {
	// Code is the ISO 4217 three-letter currency code.
	Code string `json:"code"`

	// Name is the human-readable currency name.
	Name string `json:"name"`

	// Symbol is the optional currency symbol (e.g. "$").
	Symbol string `json:"symbol,omitempty"`
}

// DeliveryEstimate is the expected arrival time for a transfer.
type DeliveryEstimate struct {
	// EstimatedDeliveryDate is when funds are expected to arrive.
	EstimatedDeliveryDate Time `json:"estimatedDeliveryDate"`

	// Guaranteed indicates whether the delivery date is guaranteed.
	Guaranteed bool `json:"guaranteed"`

	// Source describes how the estimate was calculated.
	Source string `json:"source"`
}

// Address is a physical postal address.
type Address struct {
	// ID is the unique address identifier (zero for new addresses).
	ID int64 `json:"id,omitempty"`

	// Country is the ISO 3166-1 alpha-2 country code.
	Country string `json:"country"`

	// City is the city or locality name.
	City string `json:"city"`

	// PostCode is the postal/ZIP code.
	PostCode string `json:"postCode"`

	// FirstLine is the first line of the street address.
	FirstLine string `json:"firstLine"`

	// SecondLine is the optional second address line.
	SecondLine string `json:"secondLine,omitempty"`

	// State is the state/province code (required for US, CA, BR, AU).
	State string `json:"state,omitempty"`
}

// RequirementField describes a dynamic form field returned by requirements APIs.
type RequirementField struct {
	// Name is the field group name.
	Name string `json:"name"`

	// Group holds the individual input fields within this group.
	Group []RequirementGroup `json:"group"`
}

// RequirementGroup holds a group of related form fields.
type RequirementGroup struct {
	// Key is the field identifier used when submitting the form.
	Key string `json:"key"`

	// Type is the input type ("text", "select", etc.).
	Type string `json:"type"`

	// RefreshRequirementsOnChange indicates that selecting a value triggers
	// a POST to the requirements endpoint to discover dependent fields.
	RefreshRequirementsOnChange bool `json:"refreshRequirementsOnChange"`

	// Required indicates that this field must be provided.
	Required bool `json:"required"`

	// DisplayFormat is an optional formatting hint for the UI.
	DisplayFormat string `json:"displayFormat,omitempty"`

	// Example is a sample valid value.
	Example string `json:"example,omitempty"`

	// MinLength is the minimum allowed string length.
	MinLength int `json:"minLength,omitempty"`

	// MaxLength is the maximum allowed string length.
	MaxLength int `json:"maxLength,omitempty"`

	// ValidationRegexp is a regular expression the value must match.
	ValidationRegexp string `json:"validationRegexp,omitempty"`

	// ValidateAsync indicates that the value must be validated server-side.
	ValidateAsync bool `json:"validateAsync"`

	// ValuesAllowed is the list of valid options for select-type fields.
	ValuesAllowed []AllowedValue `json:"valuesAllowed,omitempty"`
}

// AllowedValue is a valid option for a select / enum field.
type AllowedValue struct {
	// Key is the value to submit when this option is selected.
	Key string `json:"key"`

	// Name is the human-readable display label.
	Name string `json:"name"`
}

// PaymentMethod describes the pay-in or pay-out method for a quote or transfer.
type PaymentMethod string

const (
	// PaymentMethodBalance funds the transfer from a Wise balance.
	PaymentMethodBalance PaymentMethod = "BALANCE"

	// PaymentMethodBankTransfer uses a bank transfer for funding/payout.
	PaymentMethodBankTransfer PaymentMethod = "BANK_TRANSFER"

	// PaymentMethodDebitCard uses a debit card for funding.
	PaymentMethodDebitCard PaymentMethod = "DEBIT_CARD"

	// PaymentMethodCreditCard uses a credit card for funding.
	PaymentMethodCreditCard PaymentMethod = "CREDIT_CARD"

	// PaymentMethodSwift uses SWIFT for payout.
	PaymentMethodSwift PaymentMethod = "SWIFT"

	// PaymentMethodPIX uses PIX (Brazil) for payout.
	PaymentMethodPIX PaymentMethod = "PIX"
)

// ExchangeRate represents a currency pair exchange rate.
type ExchangeRate struct {
	// Source is the source currency code.
	Source string `json:"source"`

	// Target is the target currency code.
	Target string `json:"target"`

	// Rate is the mid-market exchange rate.
	Rate float64 `json:"rate"`

	// Time is when the rate was recorded.
	Time Time `json:"time"`
}
