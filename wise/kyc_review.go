package wise

import (
	"context"
	"fmt"
)

// =============================================================================
// KYC Review Service
// =============================================================================

// KYCReviewService provides access to the Wise KYC Review API.
//
// A KYC Review is a structured workflow for collecting and validating identity
// information from a customer. There are two integration patterns:
//
//   - Hosted KYC — redirect the customer to a Wise-hosted UI via the link
//     returned by UpdateRedirectURL.
//   - API submission — submit requirements directly using SubmitRequirement
//     when apiCollectionSupported is true on the requirement.
//
// Typical flow:
//  1. Call Create to open a KYC review for a profile action.
//  2. Call UpdateRedirectURL with your redirect URL to get the hosted KYC link.
//  3. Redirect the customer; Wise appends ?status=success|failed|closed on return.
//  4. Poll GetByID or listen to the verification webhook to track completion.
type KYCReviewService struct{ c *Client }

// KYCReviewStatus is the lifecycle state of a KYC review.
type KYCReviewStatus string

const (
	// KYCReviewWaitingCustomerInput means the customer must complete actions.
	KYCReviewWaitingCustomerInput KYCReviewStatus = "WAITING_CUSTOMER_INPUT"

	// KYCReviewPending means the review is under evaluation by Wise.
	KYCReviewPending KYCReviewStatus = "PENDING"

	// KYCReviewApproved means the KYC review passed.
	KYCReviewApproved KYCReviewStatus = "APPROVED"

	// KYCReviewRejected means the KYC review was not approved.
	KYCReviewRejected KYCReviewStatus = "REJECTED"

	// KYCReviewExpired means the KYC review expired without completion.
	KYCReviewExpired KYCReviewStatus = "EXPIRED"
)

// KYCRequirementState describes the collection status of a single KYC requirement.
type KYCRequirementState string

const (
	// KYCRequirementNotProvided means the data has not been submitted yet.
	KYCRequirementNotProvided KYCRequirementState = "NOT_PROVIDED"

	// KYCRequirementProvided means the data has been submitted.
	KYCRequirementProvided KYCRequirementState = "PROVIDED"

	// KYCRequirementVerified means the submitted data was accepted.
	KYCRequirementVerified KYCRequirementState = "VERIFIED"

	// KYCRequirementRejected means the submitted data was rejected.
	KYCRequirementRejected KYCRequirementState = "REJECTED"

	// KYCRequirementExpired means the submitted data expired.
	KYCRequirementExpired KYCRequirementState = "EXPIRED"
)

// KYCReview is an identity verification workflow for a Wise profile.
type KYCReview struct {
	// ID is the unique identifier for this KYC review.
	ID string `json:"id"`

	// ProfileID is the Wise profile this review belongs to.
	ProfileID int64 `json:"profileId"`

	// Status is the current lifecycle state of the review.
	Status KYCReviewStatus `json:"status"`

	// Link is the Wise-hosted KYC URL the customer must visit.
	// Populated after calling UpdateRedirectURL.
	Link string `json:"link,omitempty"`

	// Requirements lists the verification requirements for this review.
	Requirements []KYCRequirement `json:"requirements,omitempty"`

	// CreatedAt is when this review was created.
	CreatedAt Time `json:"createdAt"`

	// UpdatedAt is when this review was last modified.
	UpdatedAt Time `json:"updatedAt"`
}

// KYCRequirement is a single identity data requirement within a KYC review.
type KYCRequirement struct {
	// Key identifies the requirement type (e.g. "PROOF_OF_IDENTITY").
	Key string `json:"key"`

	// State is the collection/verification status of this requirement.
	State KYCRequirementState `json:"state"`

	// APICollectionSupported indicates that this requirement can be submitted
	// via SubmitRequirement rather than the hosted KYC flow.
	APICollectionSupported bool `json:"apiCollectionSupported"`

	// Description is a human-readable label for the requirement.
	Description string `json:"description,omitempty"`
}

// CreateKYCReviewRequest is the body for creating a new KYC review.
type CreateKYCReviewRequest struct {
	// ProfileID is the profile for which the review is being created.
	ProfileID int64 `json:"profileId"`

	// Action describes the business action requiring verification
	// (e.g. "ONBOARDING", "PERIODIC_REVIEW").
	Action string `json:"action,omitempty"`
}

// UpdateKYCReviewRequest is the body for patching a KYC review with a redirect URL.
type UpdateKYCReviewRequest struct {
	// RedirectURL is where Wise redirects the customer after the hosted KYC flow.
	// Wise appends ?status=success, ?status=failed, or ?status=closed.
	RedirectURL string `json:"redirectUrl"`
}

// Create creates a KYC review for a specific customer action.
//
// POST /v1/profiles/{profileId}/kyc-reviews.
func (s *KYCReviewService) Create(ctx context.Context, profileID int64, req CreateKYCReviewRequest) (*KYCReview, error) {
	var review KYCReview
	if err := s.c.post(ctx, fmt.Sprintf("/v1/profiles/%d/kyc-reviews", profileID), req, &review); err != nil {
		return nil, err
	}

	return &review, nil
}

// List returns all active KYC reviews for a profile.
//
// GET /v1/profiles/{profileId}/kyc-reviews.
func (s *KYCReviewService) List(ctx context.Context, profileID int64) ([]KYCReview, error) {
	var reviews []KYCReview
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/kyc-reviews", profileID), nil, &reviews); err != nil {
		return nil, err
	}

	return reviews, nil
}

// UpdateRedirectURL patches a KYC review with a redirect URL and returns the
// populated link field — the hosted Wise URL the customer must visit.
//
// After the customer completes the flow, Wise redirects to your redirectURL with
// one of: ?status=success, ?status=failed, or ?status=closed appended.
//
// PATCH /v1/profiles/{profileId}/kyc-reviews/{kycReviewId}.
func (s *KYCReviewService) UpdateRedirectURL(ctx context.Context, profileID int64, kycReviewID, redirectURL string) (*KYCReview, error) {
	req := UpdateKYCReviewRequest{RedirectURL: redirectURL}
	var review KYCReview

	if err := s.c.patch(ctx, fmt.Sprintf("/v1/profiles/%d/kyc-reviews/%s", profileID, kycReviewID), req, &review); err != nil {
		return nil, err
	}

	return &review, nil
}

// GetByID retrieves a single KYC review by ID (v2 — current).
//
// GET /v2/profiles/{profileId}/kyc-reviews/{kycReviewId}.
func (s *KYCReviewService) GetByID(ctx context.Context, profileID int64, kycReviewID string) (*KYCReview, error) {
	var review KYCReview
	if err := s.c.get(ctx, fmt.Sprintf("/v2/profiles/%d/kyc-reviews/%s", profileID, kycReviewID), nil, &review); err != nil {
		return nil, err
	}

	return &review, nil
}

// GetByIDV1 retrieves a KYC review using the deprecated v1 endpoint.
//
// Deprecated: Use GetByID (v2) instead.
//
// GET /v1/profiles/{profileId}/kyc-reviews/{kycReviewId}.
func (s *KYCReviewService) GetByIDV1(ctx context.Context, profileID int64, kycReviewID string) (*KYCReview, error) {
	var review KYCReview
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/kyc-reviews/%s", profileID, kycReviewID), nil, &review); err != nil {
		return nil, err
	}

	return &review, nil
}

// SubmitRequirement submits data for a specific KYC requirement.
// Only call this when the requirement's State is NOT_PROVIDED and
// APICollectionSupported is true.
//
// The payload structure varies per requirement type — consult the
// Wise KYC requirement types guide for accepted field structures.
//
// POST /v2/profiles/{profileId}/kyc-requirements/{requirementKey}.
func (s *KYCReviewService) SubmitRequirement(ctx context.Context, profileID int64, requirementKey string, payload map[string]any) error {
	return s.c.post(ctx, fmt.Sprintf("/v2/profiles/%d/kyc-requirements/%s", profileID, requirementKey), payload, nil)
}
