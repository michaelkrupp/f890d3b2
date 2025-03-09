package authsvc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mkrupp/homecase-michael/internal/domain"
)

// ValidateToken validates an authentication token by:
// - Decoding the base64url-encoded token
// - Verifying the RSA-PSS signature using SHA256
// - Parsing the JSON payload into an AuthToken
// - Checking if the token has expired
// Returns the parsed AuthToken if valid, or an error if validation fails.
// Returns domain.ErrInvalidAuthToken for any validation failure.
func ValidateToken(ctx context.Context, tokenString string, publicKey *rsa.PublicKey) (domain.AuthToken, error) {
	// Decode token
	tokenData, err := base64.URLEncoding.DecodeString(tokenString)
	if err != nil {
		return domain.AuthToken{}, errors.Join(domain.ErrInvalidAuthToken, fmt.Errorf("decode token: %w", err))
	}

	if len(tokenData) < rsa.PSSSaltLengthAuto { // Minimum signature length
		return domain.AuthToken{}, domain.ErrInvalidAuthToken
	}

	signatureStart := len(tokenData) - publicKey.Size()
	if signatureStart < 0 {
		return domain.AuthToken{}, domain.ErrInvalidAuthToken
	}

	payload := tokenData[:signatureStart]
	signature := tokenData[signatureStart:]

	// Verify signature
	hashed := sha256.Sum256(payload)
	if err := rsa.VerifyPSS(publicKey, crypto.SHA256, hashed[:], signature, nil); err != nil {
		return domain.AuthToken{}, domain.ErrInvalidAuthToken
	}

	// Parse token
	var token domain.AuthToken
	if err := json.Unmarshal(payload, &token); err != nil {
		return domain.AuthToken{}, errors.Join(domain.ErrInvalidAuthToken, fmt.Errorf("unmarshal token: %w", err))
	}

	// Check expiration
	if token.ExpiresAt < time.Now().Unix() {
		return domain.AuthToken{}, domain.ErrInvalidAuthToken
	}

	return token, nil
}
