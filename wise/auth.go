package wise

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// OAuth Service
// =============================================================================

// OAuthService provides OAuth 2.0 token exchange helpers.
type OAuthService struct{ c *Client }

// AuthorizationURLParams configures the OAuth 2.0 authorization redirect URL.
type AuthorizationURLParams struct {
	RedirectURI string
	State       string
	Scope       string
}

// TokenResponse is returned by all token-exchange endpoints.
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"-"`
}

// AuthorizationURL builds the URL for redirecting users to the Wise OAuth consent page.
func (s *OAuthService) AuthorizationURL(p AuthorizationURLParams) string {
	v := url.Values{}
	v.Set("client_id", s.c.cfg.clientID)
	v.Set("redirect_uri", p.RedirectURI)
	v.Set("response_type", "code")
	if p.State != "" {
		v.Set("state", p.State)
	}
	if p.Scope != "" {
		v.Set("scope", p.Scope)
	}
	return s.c.baseURL + "/oauth/authorize?" + v.Encode()
}

// ExchangeCode exchanges an authorization code for an access token and refresh token.
func (s *OAuthService) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	params := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {s.c.cfg.clientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	return s.postToken(ctx, params)
}

// ExchangeRegistrationCode exchanges a registration_code grant for an access token.
func (s *OAuthService) ExchangeRegistrationCode(ctx context.Context, registrationCode string) (*TokenResponse, error) {
	params := url.Values{
		"grant_type":        {"registration_code"},
		"client_id":         {s.c.cfg.clientID},
		"registration_code": {registrationCode},
	}
	return s.postToken(ctx, params)
}

// RefreshToken uses a refresh token to obtain a new access token.
func (s *OAuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {s.c.cfg.clientID},
		"refresh_token": {refreshToken},
	}
	return s.postToken(ctx, params)
}

func (s *OAuthService) postToken(ctx context.Context, params url.Values) (*TokenResponse, error) {
	creds := base64.StdEncoding.EncodeToString([]byte(s.c.cfg.clientID + ":" + s.c.cfg.clientSecret))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.c.baseURL+"/oauth/token", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("wise: build token request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(headerUserAgent, s.c.cfg.userAgent)

	resp, err := s.c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wise: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("wise: read token response: %w", err)
	}
	if !isSuccess(resp.StatusCode) {
		return nil, parseAPIError(resp, body)
	}
	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("wise: decode token: %w", err)
	}
	tok.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return &tok, nil
}

// =============================================================================
// User Service
// =============================================================================

// UserService manages Wise user accounts.
type UserService struct{ c *Client }

// User represents a Wise user account.
type User struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	Active           bool   `json:"active"`
	RegistrationCode string `json:"registrationCode,omitempty"`
	CreatedAt        Time   `json:"createdAt"`
}

// CreateUserRequest is the body for creating a user with a registration code.
type CreateUserRequest struct {
	Email            string `json:"email"`
	RegistrationCode string `json:"registrationCode,omitempty"`
}

// CreateUserResponse is returned when a new user is created.
type CreateUserResponse struct {
	ID               int64  `json:"id"`
	Email            string `json:"email"`
	RegistrationCode string `json:"registrationCode"`
}

// Create creates a new Wise user account.
func (s *UserService) Create(ctx context.Context, req CreateUserRequest) (*CreateUserResponse, error) {
	var resp CreateUserResponse
	if err := s.c.post(ctx, "/v1/users", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Me returns the current authenticated user.
func (s *UserService) Me(ctx context.Context) (*User, error) {
	var u User
	if err := s.c.get(ctx, "/v1/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// Get returns a user by ID.
func (s *UserService) Get(ctx context.Context, userID int64) (*User, error) {
	var u User
	if err := s.c.get(ctx, fmt.Sprintf("/v1/users/%d", userID), nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// =============================================================================
// User Security Service
// =============================================================================

// UserSecurityService manages PIN, FaceMap, phone number, and device fingerprint setup.
type UserSecurityService struct{ c *Client }

// PINRequest is the body for setting a user PIN.
type PINRequest struct {
	PIN        string `json:"pin"`
	ConfirmPIN string `json:"confirmPin"`
}

// FaceMapRequest is the body for enrolling a FaceMap biometric.
type FaceMapRequest struct {
	FaceMapEncrypted          string `json:"faceMapEncrypted"`
	LowQualityAuditTrailImage string `json:"lowQualityAuditTrailImage,omitempty"`
}

// PhoneNumber represents a phone number registered for OTP delivery.
type PhoneNumber struct {
	ID          int64  `json:"id"`
	PhoneNumber string `json:"phoneNumber"`
	Active      bool   `json:"active"`
	CountryCode string `json:"countryCode"`
}

// CreatePhoneNumberRequest is the body for registering a phone number.
type CreatePhoneNumberRequest struct {
	PhoneNumber string `json:"phoneNumber"`
}

// DeviceFingerprint represents a registered device fingerprint.
type DeviceFingerprint struct {
	ID          string `json:"id"`
	DeviceToken string `json:"deviceToken"`
	Active      bool   `json:"active"`
}

// CreateDeviceFingerprintRequest is the body for registering a device fingerprint.
type CreateDeviceFingerprintRequest struct {
	DeviceToken string `json:"deviceToken"`
	Name        string `json:"name,omitempty"`
}

// CreatePIN sets up a PIN for the user. Required before PIN-based SCA challenges.
func (s *UserSecurityService) CreatePIN(ctx context.Context, userID int64, req PINRequest) error {
	return s.c.post(ctx, fmt.Sprintf("/v1/users/%d/pin", userID), req, nil)
}

// EnrolFaceMap registers a FaceMap for biometric SCA challenges.
func (s *UserSecurityService) EnrolFaceMap(ctx context.Context, userID int64, req FaceMapRequest) error {
	return s.c.post(ctx, fmt.Sprintf("/v1/users/%d/facemap", userID), req, nil)
}

// CreatePhoneNumber registers a phone number for OTP delivery.
func (s *UserSecurityService) CreatePhoneNumber(ctx context.Context, userID int64, req CreatePhoneNumberRequest) (*PhoneNumber, error) {
	var p PhoneNumber
	if err := s.c.post(ctx, fmt.Sprintf("/v1/users/%d/phone-numbers", userID), req, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPhoneNumbers returns all phone numbers registered for the user.
func (s *UserSecurityService) ListPhoneNumbers(ctx context.Context, userID int64) ([]PhoneNumber, error) {
	var phones []PhoneNumber
	if err := s.c.get(ctx, fmt.Sprintf("/v1/users/%d/phone-numbers", userID), nil, &phones); err != nil {
		return nil, err
	}
	return phones, nil
}

// CreateDeviceFingerprint registers a device fingerprint for partner-device SCA.
func (s *UserSecurityService) CreateDeviceFingerprint(ctx context.Context, userID int64, req CreateDeviceFingerprintRequest) (*DeviceFingerprint, error) {
	var df DeviceFingerprint
	if err := s.c.post(ctx, fmt.Sprintf("/v1/users/%d/device-fingerprints", userID), req, &df); err != nil {
		return nil, err
	}
	return &df, nil
}

// =============================================================================
// Strong Customer Authentication (SCA) Service
// =============================================================================

// SCAService provides access to the Wise Strong Customer Authentication API.
type SCAService struct{ c *Client }

// SCAStatus describes the current SCA challenge state.
type SCAStatus struct {
	Passed     bool           `json:"passed"`
	Challenges []SCAChallenge `json:"challenges"`
	ExpiresAt  Time           `json:"expiresAt,omitempty"`
}

// SCAChallenge is a single authentication factor required to pass SCA.
type SCAChallenge struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Passed   bool   `json:"passed"`
	OtpID    string `json:"otpId,omitempty"`
}

// ChallengeType is the type of an SCA challenge.
type ChallengeType string

const (
	// ChallengePIN is a PIN-based SCA challenge.
	ChallengePIN ChallengeType = "PIN"

	// ChallengeFaceMap is a facial biometric SCA challenge.
	ChallengeFaceMap ChallengeType = "FACE_MAP"

	// ChallengeSMS is an SMS one-time password challenge.
	ChallengeSMS ChallengeType = "SMS"

	// ChallengeWhatsApp is a WhatsApp one-time password challenge.
	ChallengeWhatsApp ChallengeType = "WHATSAPP"

	// ChallengeVoice is a voice call one-time password challenge.
	ChallengeVoice ChallengeType = "VOICE"

	// ChallengePartnerDevice is a partner device fingerprint challenge.
	ChallengePartnerDevice ChallengeType = "PARTNER_DEVICE_FINGERPRINT"
)

// SCAVerifyRequest is the body for completing an SCA challenge.
type SCAVerifyRequest struct {
	Type             ChallengeType `json:"type"`
	OTP              string        `json:"otp,omitempty"`
	PIN              string        `json:"pin,omitempty"`
	FaceMapEncrypted string        `json:"faceMapEncrypted,omitempty"`
	DeviceToken      string        `json:"deviceToken,omitempty"`
}

// Status retrieves the current SCA challenge state.
func (s *SCAService) Status(ctx context.Context) (*SCAStatus, error) {
	var status SCAStatus
	if err := s.c.get(ctx, "/v1/auth/sca/status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// Verify submits a completed challenge response.
func (s *SCAService) Verify(ctx context.Context, req SCAVerifyRequest) (*SCAStatus, error) {
	var status SCAStatus
	if err := s.c.post(ctx, "/v1/auth/sca/verify", req, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// IsPassed reports whether all required SCA challenges have been completed.
func (s *SCAService) IsPassed(status *SCAStatus) bool {
	if status == nil {
		return false
	}
	for _, ch := range status.Challenges {
		if ch.Required && !ch.Passed {
			return false
		}
	}
	return true
}

// PendingChallenges returns required challenges that have not yet been completed.
func (s *SCAService) PendingChallenges(status *SCAStatus) []SCAChallenge {
	if status == nil {
		return nil
	}
	var pending []SCAChallenge
	for _, ch := range status.Challenges {
		if ch.Required && !ch.Passed {
			pending = append(pending, ch)
		}
	}
	return pending
}

// =============================================================================
// One Time Token (OTT) Service — Deprecated
// =============================================================================

// OTTService provides the legacy One Time Token SCA framework.
// Deprecated: Use SCAService for new integrations.
type OTTService struct{ c *Client }

// OTTStatus describes the current OTT challenge state.
type OTTStatus struct {
	ExpiresAt  Time           `json:"expiresAt"`
	Challenges []OTTChallenge `json:"challenges"`
}

// OTTChallenge is a single challenge in an OTT session.
type OTTChallenge struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Passed   bool   `json:"passed"`
	OTPID    string `json:"otpId,omitempty"`
}

// Status retrieves the current OTT challenge status.
func (s *OTTService) Status(ctx context.Context) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.get(ctx, "/v1/one-time-token/status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// VerifyPIN completes a PIN challenge.
func (s *OTTService) VerifyPIN(ctx context.Context, pin string) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/pin/verify", map[string]string{"pin": pin}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// VerifyDeviceFingerprint completes a partner-device-fingerprint challenge.
func (s *OTTService) VerifyDeviceFingerprint(ctx context.Context, deviceToken string) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/partner-device-fingerprint/verify", map[string]string{"deviceToken": deviceToken}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// VerifyFaceMap completes a FACE_MAP challenge.
func (s *OTTService) VerifyFaceMap(ctx context.Context, faceMapEncrypted string) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/facemap/verify", map[string]string{"faceMapEncrypted": faceMapEncrypted}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// TriggerSMS sends an OTP via SMS to the registered phone number.
func (s *OTTService) TriggerSMS(ctx context.Context) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/sms/trigger", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// VerifySMS completes an SMS OTP challenge.
func (s *OTTService) VerifySMS(ctx context.Context, otp string) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/sms/verify", map[string]string{"otp": otp}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// TriggerWhatsApp sends an OTP via WhatsApp.
func (s *OTTService) TriggerWhatsApp(ctx context.Context) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/whatsapp/trigger", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// VerifyWhatsApp completes a WhatsApp OTP challenge.
func (s *OTTService) VerifyWhatsApp(ctx context.Context, otp string) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/whatsapp/verify", map[string]string{"otp": otp}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// TriggerVoice sends an OTP via automated voice call.
func (s *OTTService) TriggerVoice(ctx context.Context) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/voice/trigger", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// VerifyVoice completes a voice OTP challenge.
func (s *OTTService) VerifyVoice(ctx context.Context, otp string) (*OTTStatus, error) {
	var status OTTStatus
	if err := s.c.post(ctx, "/v1/one-time-token/voice/verify", map[string]string{"otp": otp}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// IsPassed reports whether all required OTT challenges have been completed.
func (s *OTTService) IsPassed(status *OTTStatus) bool {
	if status == nil {
		return false
	}
	for _, ch := range status.Challenges {
		if ch.Required && !ch.Passed {
			return false
		}
	}
	return true
}

// =============================================================================
// 3D Secure Service
// =============================================================================

// ThreeDSService manages 3D Secure challenge results for card transactions.
type ThreeDSService struct{ c *Client }

// ThreeDSChallengeResult is the accepted/rejected status of a 3DS challenge.
type ThreeDSChallengeResult string

const (
	// ThreeDSChallengeAccepted means the cardholder approved the 3DS request.
	ThreeDSChallengeAccepted ThreeDSChallengeResult = "ACCEPTED"

	// ThreeDSChallengeRejected means the cardholder rejected the 3DS request.
	ThreeDSChallengeRejected ThreeDSChallengeResult = "REJECTED"
)

// InformChallengeResultRequest is the body for the 3DS challenge result endpoint.
type InformChallengeResultRequest struct {
	ChallengeID string                 `json:"challengeId"`
	Result      ThreeDSChallengeResult `json:"result"`
}

// InformChallengeResult notifies Wise of the cardholder's 3DS challenge decision.
func (s *ThreeDSService) InformChallengeResult(ctx context.Context, profileID int64, req InformChallengeResultRequest) error {
	return s.c.post(ctx, fmt.Sprintf("/v3/spend/profiles/%d/3dsecure/challenge-result", profileID), req, nil)
}

// =============================================================================
// KYC Service
// =============================================================================

// KYCService manages additional customer verification (KYC evidence upload).
type KYCService struct{ c *Client }

// KYCStatus is the overall KYC state of a profile.
type KYCStatus struct {
	ProfileID int64  `json:"profileId"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

// RequiredEvidence describes a KYC evidence type that must be submitted.
type RequiredEvidence struct {
	Type        string `json:"type"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// EvidenceUploadRequest is the body for uploading verification evidence.
type EvidenceUploadRequest struct {
	EvidenceType string         `json:"evidenceType"`
	Content      map[string]any `json:"content"`
}

// GetRequiredEvidences returns the evidence types the profile still needs to submit.
func (s *KYCService) GetRequiredEvidences(ctx context.Context, profileID int64) ([]RequiredEvidence, error) {
	var evidences []RequiredEvidence
	path := fmt.Sprintf("/v3/profiles/%d/verification-status/required-evidences", profileID)
	if err := s.c.get(ctx, path, nil, &evidences); err != nil {
		return nil, err
	}
	return evidences, nil
}

// UploadEvidences uploads KYC evidence for a profile (v5 — current).
func (s *KYCService) UploadEvidences(ctx context.Context, profileID int64, req EvidenceUploadRequest) error {
	path := fmt.Sprintf("/v5/profiles/%d/additional-verification/upload-evidences", profileID)
	return s.c.post(ctx, path, req, nil)
}

// UploadEvidencesV3 is the deprecated v3 evidence upload endpoint.
// Deprecated: Use UploadEvidences instead.
func (s *KYCService) UploadEvidencesV3(ctx context.Context, profileID int64, req EvidenceUploadRequest) error {
	path := fmt.Sprintf("/v3/profiles/%d/additional-verification/upload-evidences", profileID)
	return s.c.post(ctx, path, req, nil)
}

// GetKYCStatus returns the overall KYC state of a profile.
func (s *KYCService) GetKYCStatus(ctx context.Context, profileID int64) (*KYCStatus, error) {
	var status KYCStatus
	if err := s.c.get(ctx, fmt.Sprintf("/v1/profiles/%d/kyc/status", profileID), nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// SubmitKYCReview triggers a KYC review for the profile.
func (s *KYCService) SubmitKYCReview(ctx context.Context, profileID int64) error {
	return s.c.post(ctx, fmt.Sprintf("/v1/profiles/%d/kyc/review", profileID), nil, nil)
}

// =============================================================================
// Claim Account Service
// =============================================================================

// ClaimAccountService generates codes for users to claim partner-created accounts.
type ClaimAccountService struct{ c *Client }

// ClaimAccountCode is returned when generating an account claim code.
type ClaimAccountCode struct {
	Code      string `json:"claimAccountCode"`
	ExpiresAt Time   `json:"expiresAt"`
}

// GenerateCode generates a short-lived code for a user to claim their account.
func (s *ClaimAccountService) GenerateCode(ctx context.Context, userID int64) (*ClaimAccountCode, error) {
	body := map[string]int64{"userId": userID}
	var code ClaimAccountCode
	if err := s.c.post(ctx, "/v1/user/claim-account", body, &code); err != nil {
		return nil, err
	}
	return &code, nil
}

// =============================================================================
// Contact Service
// =============================================================================

// ContactService finds discoverable Wise profiles by Wisetag, email, or phone.
type ContactService struct{ c *Client }

// Contact represents a found Wise contact.
type Contact struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl,omitempty"`
	WiseTag  string `json:"wiseTag,omitempty"`
}

// FindContactRequest is the body for finding a contact.
// Exactly one of WiseTag, Email, or PhoneNumber must be provided.
type FindContactRequest struct {
	WiseTag     string `json:"wiseTag,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
}

// Find searches for a Wise profile by Wisetag, email, or phone number.
func (s *ContactService) Find(ctx context.Context, profileID int64, req FindContactRequest) (*Contact, error) {
	var c Contact
	if err := s.c.post(ctx, fmt.Sprintf("/v2/profiles/%d/contacts", profileID), req, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// =============================================================================
// FaceTec Service
// =============================================================================

// FaceTecService provides the public key for FaceTec 3D FaceMap encryption.
type FaceTecService struct{ c *Client }

// FaceTecPublicKey is the RSA public key for encrypting FaceMap exports.
type FaceTecPublicKey struct {
	Key string `json:"key"`
}

// GetPublicKey retrieves the Wise FaceTec public key.
func (s *FaceTecService) GetPublicKey(ctx context.Context) (*FaceTecPublicKey, error) {
	var key FaceTecPublicKey
	if err := s.c.get(ctx, "/v1/facetec/public-key", nil, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// =============================================================================
// JOSE Service
// =============================================================================

// JOSEService provides JOSE key management and playground endpoints.
type JOSEService struct{ c *Client }

// JOSEPublicKey represents a Wise JWK public key.
type JOSEPublicKey struct {
	KID string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// JOSEPublicKeySet is a JSON Web Key Set response.
type JOSEPublicKeySet struct {
	Keys []JOSEPublicKey `json:"keys"`
}

// RegisterPublicKeyRequest is the body for registering a JWS signing key.
type RegisterPublicKeyRequest struct {
	PublicKey map[string]any `json:"publicKey"`
}

// JOSEPlaygroundResult is the response from JOSE playground endpoints.
type JOSEPlaygroundResult struct {
	Result   string         `json:"result,omitempty"`
	Verified bool           `json:"verified,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

// GetResponsePublicKeys fetches Wise's JWK public keys for response verification.
func (s *JOSEService) GetResponsePublicKeys(ctx context.Context) (*JOSEPublicKeySet, error) {
	var keys JOSEPublicKeySet
	if err := s.c.get(ctx, "/v1/auth/jose/response/public-keys", nil, &keys); err != nil {
		return nil, err
	}
	return &keys, nil
}

// RegisterRequestPublicKey registers a JWS signing public key with Wise.
func (s *JOSEService) RegisterRequestPublicKey(ctx context.Context, req RegisterPublicKeyRequest) error {
	return s.c.post(ctx, "/v1/auth/jose/request/public-keys", req, nil)
}

// PlaygroundVerifyJWS submits a JWS token to the playground for validation.
func (s *JOSEService) PlaygroundVerifyJWS(ctx context.Context, token string) (*JOSEPlaygroundResult, error) {
	var result JOSEPlaygroundResult
	if err := s.c.post(ctx, "/v1/auth/jose/playground/jws", map[string]string{"token": token}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PlaygroundGetJWE fetches a sample JWE-encrypted payload from Wise.
func (s *JOSEService) PlaygroundGetJWE(ctx context.Context) (*JOSEPlaygroundResult, error) {
	var result JOSEPlaygroundResult
	if err := s.c.get(ctx, "/v1/auth/jose/playground/jwe", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PlaygroundEncryptJWE submits a payload to be encrypted by the playground.
func (s *JOSEService) PlaygroundEncryptJWE(ctx context.Context, payload map[string]any) (*JOSEPlaygroundResult, error) {
	var result JOSEPlaygroundResult
	if err := s.c.post(ctx, "/v1/auth/jose/playground/jwe", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PlaygroundEncryptJWEDirect submits a payload for JWE direct-encryption testing.
func (s *JOSEService) PlaygroundEncryptJWEDirect(ctx context.Context, payload map[string]any) (*JOSEPlaygroundResult, error) {
	var result JOSEPlaygroundResult
	if err := s.c.post(ctx, "/v1/auth/jose/playground/jwe-direct-encryption", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PlaygroundEncryptJWSJWE submits a payload for combined JWS+JWE testing.
func (s *JOSEService) PlaygroundEncryptJWSJWE(ctx context.Context, payload map[string]any) (*JOSEPlaygroundResult, error) {
	var result JOSEPlaygroundResult
	if err := s.c.post(ctx, "/v1/auth/jose/playground/jwsjwe", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// Partner Cases Service
// =============================================================================

// CasesService manages partner support cases.
type CasesService struct{ c *Client }

// CaseStatus describes the lifecycle state of a partner support case.
type CaseStatus string

const (
	// CaseStatusCreating means the case is being created.
	CaseStatusCreating CaseStatus = "CREATING"

	// CaseStatusOpen means the case is open and being actioned by Wise.
	CaseStatusOpen CaseStatus = "OPEN"

	// CaseStatusPending means the case requires action from the partner.
	CaseStatusPending CaseStatus = "PENDING"

	// CaseStatusSolved means the case has been solved.
	CaseStatusSolved CaseStatus = "SOLVED"

	// CaseStatusClosed means the case is closed and cannot be reopened.
	CaseStatusClosed CaseStatus = "CLOSED"
)

// Case is a partner support or operations query.
type Case struct {
	ID        string     `json:"id"`
	Subject   string     `json:"subject"`
	Status    CaseStatus `json:"status"`
	Type      string     `json:"type"`
	CreatedAt Time       `json:"createdAt"`
	UpdatedAt Time       `json:"updatedAt"`
}

// CaseComment is a single message in a case thread.
type CaseComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	Author    string `json:"author"`
	CreatedAt Time   `json:"createdAt"`
}

// CreateCaseRequest is the body for opening a new partner case.
type CreateCaseRequest struct {
	Subject     string `json:"subject"`
	Type        string `json:"type"`
	Description string `json:"description"`
	TransferID  int64  `json:"transferId,omitempty"`
	ProfileID   int64  `json:"profileId,omitempty"`
}

// Create opens a new partner support case.
func (s *CasesService) Create(ctx context.Context, req CreateCaseRequest) (*Case, error) {
	var c Case
	if err := s.c.post(ctx, "/v1/cases", req, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Get retrieves a case by ID.
func (s *CasesService) Get(ctx context.Context, caseID string) (*Case, error) {
	var c Case
	if err := s.c.get(ctx, fmt.Sprintf("/v1/cases/%s", caseID), nil, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListComments returns all comments on a case.
func (s *CasesService) ListComments(ctx context.Context, caseID string) ([]CaseComment, error) {
	var comments []CaseComment
	if err := s.c.get(ctx, fmt.Sprintf("/v1/cases/%s/comments", caseID), nil, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

// AddComment posts a reply to a case.
func (s *CasesService) AddComment(ctx context.Context, caseID, body string) (*CaseComment, error) {
	var comment CaseComment
	if err := s.c.put(ctx, fmt.Sprintf("/v1/cases/%s/comments", caseID), map[string]string{"body": body}, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// =============================================================================
// Multi Currency Account Service
// =============================================================================

// MultiCurrencyAccountService provides MCA configuration and eligibility.
type MultiCurrencyAccountService struct{ c *Client }

// MultiCurrencyAccount holds the top-level MCA details for a profile.
type MultiCurrencyAccount struct {
	ID        int64     `json:"id"`
	ProfileID int64     `json:"profileId"`
	Balances  []Balance `json:"balances,omitempty"`
}

// MCAEligibility describes whether a profile is eligible for an MCA.
type MCAEligibility struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason,omitempty"`
}

// Get returns the multi-currency account for the given profile.
func (s *MultiCurrencyAccountService) Get(ctx context.Context, profileID int64) (*MultiCurrencyAccount, error) {
	var mca MultiCurrencyAccount
	if err := s.c.get(ctx, fmt.Sprintf("/v4/profiles/%d/multi-currency-account", profileID), nil, &mca); err != nil {
		return nil, err
	}
	return &mca, nil
}

// CheckEligibility reports whether the user is eligible for an MCA.
func (s *MultiCurrencyAccountService) CheckEligibility(ctx context.Context) (*MCAEligibility, error) {
	var el MCAEligibility
	if err := s.c.get(ctx, "/v4/multi-currency-account/eligibility", nil, &el); err != nil {
		return nil, err
	}
	return &el, nil
}

// AvailableCurrencies returns currencies available for balances on this profile.
func (s *MultiCurrencyAccountService) AvailableCurrencies(ctx context.Context, profileID int64) ([]Currency, error) {
	var currencies []Currency
	path := fmt.Sprintf("/v2/borderless-accounts-configuration/profiles/%d/available-currencies", profileID)
	if err := s.c.get(ctx, path, nil, &currencies); err != nil {
		return nil, err
	}
	return currencies, nil
}

// PayInCurrencies returns currencies that can be used to pay in to the MCA.
func (s *MultiCurrencyAccountService) PayInCurrencies(ctx context.Context, profileID int64) ([]Currency, error) {
	var currencies []Currency
	path := fmt.Sprintf("/v2/borderless-accounts-configuration/profiles/%d/payin-currencies", profileID)
	if err := s.c.get(ctx, path, nil, &currencies); err != nil {
		return nil, err
	}
	return currencies, nil
}

// =============================================================================
// Webhook Service
// =============================================================================

// WebhookService manages webhook subscriptions and event routing.
type WebhookService struct{ c *Client }

// WebhookSubscription represents a registered webhook endpoint.
type WebhookSubscription struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	TriggerOn WebhookEventType `json:"triggerOn"`
	Delivery  WebhookDelivery  `json:"delivery"`
	Scope     WebhookScope     `json:"scope"`
	CreatedOn Time             `json:"createdOn"`
}

// WebhookDelivery describes the delivery endpoint configuration.
type WebhookDelivery struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// WebhookScope describes the profile scope of a subscription.
type WebhookScope struct {
	Domain string `json:"domain"`
	ID     string `json:"id"`
}

// CreateWebhookRequest is the body for registering a webhook subscription.
type CreateWebhookRequest struct {
	Name      string           `json:"name"`
	TriggerOn WebhookEventType `json:"triggerOn"`
	URL       string           `json:"url"`
	ProfileID int64            `json:"-"`
}

// Create registers a new webhook subscription.
func (s *WebhookService) Create(ctx context.Context, req CreateWebhookRequest) (*WebhookSubscription, error) {
	body := map[string]any{
		"name":      req.Name,
		"triggerOn": req.TriggerOn,
		"delivery":  map[string]string{"version": "2.0.0", "url": req.URL},
		"scope":     map[string]string{"domain": "profile", "id": fmt.Sprintf("%d", req.ProfileID)},
	}
	var sub WebhookSubscription
	if err := s.c.post(ctx, fmt.Sprintf("/v3/profiles/%d/subscriptions", req.ProfileID), body, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// List returns all webhook subscriptions for the given profile.
func (s *WebhookService) List(ctx context.Context, profileID int64) ([]WebhookSubscription, error) {
	var subs []WebhookSubscription
	if err := s.c.get(ctx, fmt.Sprintf("/v3/profiles/%d/subscriptions", profileID), nil, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// Get retrieves a webhook subscription by ID.
func (s *WebhookService) Get(ctx context.Context, profileID int64, subscriptionID string) (*WebhookSubscription, error) {
	var sub WebhookSubscription
	if err := s.c.get(ctx, fmt.Sprintf("/v3/profiles/%d/subscriptions/%s", profileID, subscriptionID), nil, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// Delete removes a webhook subscription.
func (s *WebhookService) Delete(ctx context.Context, profileID int64, subscriptionID string) error {
	return s.c.delete(ctx, fmt.Sprintf("/v3/profiles/%d/subscriptions/%s", profileID, subscriptionID))
}

// Test triggers a test delivery to the subscribed endpoint.
// Wise sends a sample event to confirm the endpoint is reachable.
func (s *WebhookService) Test(ctx context.Context, subscriptionID string) error {
	return s.c.post(ctx, fmt.Sprintf("/v3/subscriptions/%s/test", subscriptionID), nil, nil)
}

// WebhookEvent is the top-level envelope for Wise webhook deliveries.
type WebhookEvent struct {
	Data           json.RawMessage  `json:"data"`
	SubscriptionID string           `json:"subscriptionId"`
	EventType      WebhookEventType `json:"eventType"`
	SchemaVersion  string           `json:"schemaVersion"`
	SentAt         Time             `json:"sentAt"`
}

// UnmarshalData decodes the event Data field into dst.
func (e *WebhookEvent) UnmarshalData(dst any) error {
	return json.Unmarshal(e.Data, dst) //nolint:wrapcheck // pass-through
}

// TransferStateChangeEvent is delivered when a transfer changes status.
type TransferStateChangeEvent struct {
	Resource      TransferEventResource `json:"resource"`
	CurrentState  string                `json:"currentState"`
	PreviousState string                `json:"previousState"`
	OccurredAt    Time                  `json:"occurredAt"`
}

// TransferEventResource holds transfer identifiers in an event.
type TransferEventResource struct {
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	Profile int64  `json:"profile_id"`
	Account int64  `json:"account_id"`
}

// BalanceCreditEvent is delivered when a balance receives funds.
type BalanceCreditEvent struct {
	Resource BalanceEventResource `json:"resource"`
	Amount   Amount               `json:"amount"`
}

// BalanceEventResource holds balance identifiers in an event.
type BalanceEventResource struct {
	Type      string `json:"type"`
	ID        int64  `json:"id"`
	ProfileID int64  `json:"profile_id"`
}

// ParseEvent decodes raw webhook bytes into a WebhookEvent.
func ParseEvent(body []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("wise: parse webhook event: %w", err)
	}
	return &event, nil
}

// ParseAndVerifyEvent decodes and HMAC-SHA256-verifies a webhook payload.
func ParseAndVerifyEvent(body []byte, signatureHeader, secret string) (*WebhookEvent, error) {
	if err := VerifyWebhookSignature(body, signatureHeader, secret); err != nil {
		return nil, err
	}
	return ParseEvent(body)
}

// VerifyWebhookSignature validates the HMAC-SHA256 signature on a webhook payload.
func VerifyWebhookSignature(body []byte, signatureHeader, secret string) error {
	if signatureHeader == "" {
		return ErrInvalidWebhookSignature
	}
	sig := strings.TrimPrefix(signatureHeader, "sha256=")
	h := sha256.New()
	_, _ = h.Write([]byte(secret))
	_, _ = h.Write(body)
	expected := hex.EncodeToString(h.Sum(nil))
	if !hmacEqual(sig, expected) {
		return ErrInvalidWebhookSignature
	}
	return nil
}

// hmacEqual compares two hex strings in constant time.
func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range len(a) {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// HandlerFunc is the signature for webhook event handlers.
type HandlerFunc func(event *WebhookEvent) error

// EventRouter dispatches webhook deliveries to registered handlers.
// Register handlers at startup; EventRouter is then safe for concurrent use.
type EventRouter struct {
	mu       sync.RWMutex
	secret   string
	handlers map[WebhookEventType][]HandlerFunc
}

// NewEventRouter creates an EventRouter. Pass secret="" to skip signature verification.
func NewEventRouter(secret string) *EventRouter {
	return &EventRouter{
		mu:       sync.RWMutex{},
		secret:   secret,
		handlers: make(map[WebhookEventType][]HandlerFunc),
	}
}

// On registers a handler for an event type. Multiple handlers are called in order.
func (r *EventRouter) On(eventType WebhookEventType, h HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventType] = append(r.handlers[eventType], h)
}

// ServeHTTP implements http.Handler.
func (r *EventRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes))
	_ = req.Body.Close()

	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var event *WebhookEvent

	if r.secret != "" {
		event, err = ParseAndVerifyEvent(body, req.Header.Get("X-Signature-SHA256"), r.secret)
	} else {
		event, err = ParseEvent(body)
	}

	if err != nil {
		if errors.Is(err, ErrInvalidWebhookSignature) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	r.mu.RLock()
	handlers := append([]HandlerFunc(nil), r.handlers[event.EventType]...)
	r.mu.RUnlock()

	for _, h := range handlers {
		if hErr := h(event); hErr != nil {
			http.Error(w, "handler error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// UploadDocument uploads a verification document file (JPG, PNG, or PDF up to 10 MB)
// for KYC review. Multiple files can be uploaded in a single call.
//
// POST /v3/profiles/{profileId}/verification-status/upload-document
//
// Note: this endpoint accepts multipart/form-data. The documents parameter holds
// the base64-encoded file content and metadata; see the Wise documentation for the
// exact field structure per document type.
func (s *KYCService) UploadDocument(ctx context.Context, profileID int64, documents []map[string]any) error {
	body := map[string]any{"documents": documents}
	return s.c.post(ctx, fmt.Sprintf("/v3/profiles/%d/verification-status/upload-document", profileID), body, nil)
}
