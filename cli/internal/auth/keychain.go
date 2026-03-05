// Package auth handles OIDC authentication and token storage for the bark CLI.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "com.bark.pkgtool"
	keychainUser    = "token"
)

// TokenInfo holds a stored OAuth2 token alongside its associated identity.
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Subject      string    `json:"subject"`
	Email        string    `json:"email"`
}

// IsExpired reports whether the token is past its expiry time.
func (t *TokenInfo) IsExpired() bool {
	return !t.Expiry.IsZero() && time.Now().After(t.Expiry)
}

// ExpiresWithin reports whether the token expires within d.
func (t *TokenInfo) ExpiresWithin(d time.Duration) bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Until(t.Expiry) < d
}

// SaveToken persists ti to the macOS keychain (or OS credential store).
func SaveToken(ti *TokenInfo) error {
	data, err := json.Marshal(ti)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	if err := keyring.Set(keychainService, keychainUser, string(data)); err != nil {
		return fmt.Errorf("save token to keyring: %w", err)
	}

	return nil
}

// LoadToken retrieves the stored token. Returns ErrNoToken if nothing is stored.
func LoadToken() (*TokenInfo, error) {
	raw, err := keyring.Get(keychainService, keychainUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNoToken
		}

		return nil, fmt.Errorf("read token from keyring: %w", err)
	}

	var ti TokenInfo
	if err := json.Unmarshal([]byte(raw), &ti); err != nil {
		return nil, fmt.Errorf("parse stored token: %w", err)
	}

	return &ti, nil
}

// DeleteToken removes the stored token.
func DeleteToken() error {
	if err := keyring.Delete(keychainService, keychainUser); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil // already gone
		}

		return fmt.Errorf("delete token from keyring: %w", err)
	}

	return nil
}

// ErrNoToken is returned when no token is stored in the keychain.
var ErrNoToken = errors.New("no token stored; run 'pkgtool auth login'")
