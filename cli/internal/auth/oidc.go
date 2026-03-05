package auth

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

// OIDCConfig holds the OIDC provider configuration for device flow auth.
type OIDCConfig struct {
	// Issuer is the base URL of the OIDC provider (e.g. "https://accounts.example.com").
	Issuer string
	// ClientID is the registered client identifier.
	ClientID string
	// Scopes requested during login. Defaults to openid, email, profile if empty.
	Scopes []string
}

// Manager orchestrates OIDC device-flow login and token lifecycle management.
type Manager struct {
	cfg OIDCConfig
}

// NewManager creates a Manager for the given OIDC configuration.
func NewManager(cfg OIDCConfig) *Manager {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile", "offline_access"}
	}

	return &Manager{cfg: cfg}
}

// Login performs the OIDC device authorization grant flow.
// It prints the user code and verification URL, opens the browser, then polls
// for the token until the user completes authentication.
func (m *Manager) Login(ctx context.Context) (*TokenInfo, error) {
	oauthCfg := m.oauthConfig()

	// Request a device code.
	devAuth, err := oauthCfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("start device auth: %w", err)
	}

	// Print instructions.
	fmt.Printf("\nTo complete sign-in, open the following URL in your browser:\n\n  %s\n\n", devAuth.VerificationURIComplete)
	fmt.Printf("Confirmation code: %s\n\n", devAuth.UserCode)

	// Best-effort browser launch; user can always open the URL manually.
	//nolint:errcheck // intentionally ignored
	_ = openBrowser(devAuth.VerificationURIComplete)

	fmt.Println("Waiting for authentication...")

	// Poll until the user completes the device flow.
	token, err := oauthCfg.DeviceAccessToken(ctx, devAuth)
	if err != nil {
		return nil, fmt.Errorf("poll for token: %w", err)
	}

	ti := tokenFromOAuth(token)

	if err := SaveToken(ti); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}

	return ti, nil
}

// GetToken returns a valid access token, refreshing it automatically when it
// is within 5 minutes of expiry. Returns ErrNoToken if the user has not logged in.
func (m *Manager) GetToken(ctx context.Context) (string, error) {
	ti, err := LoadToken()
	if err != nil {
		return "", err
	}

	if ti.ExpiresWithin(5 * time.Minute) {
		ti, err = m.refresh(ctx, ti)
		if err != nil {
			return "", fmt.Errorf("token refresh failed; run 'pkgtool auth login': %w", err)
		}
	}

	return ti.AccessToken, nil
}

// Logout deletes the stored token.
func (*Manager) Logout() error {
	return DeleteToken()
}

// Status returns the stored token info without refreshing. Returns ErrNoToken if
// no token is stored.
func (*Manager) Status() (*TokenInfo, error) {
	return LoadToken()
}

// =============================================================================
// helpers
// =============================================================================

func (m *Manager) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID: m.cfg.ClientID,
		Scopes:   m.cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:       m.cfg.Issuer + "/authorize",
			TokenURL:      m.cfg.Issuer + "/oauth/token",
			DeviceAuthURL: m.cfg.Issuer + "/oauth/device/code",
		},
	}
}

func (m *Manager) refresh(ctx context.Context, ti *TokenInfo) (*TokenInfo, error) {
	if ti.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	oauthCfg := m.oauthConfig()
	src := oauthCfg.TokenSource(ctx, &oauth2.Token{
		AccessToken:  ti.AccessToken,
		RefreshToken: ti.RefreshToken,
		Expiry:       ti.Expiry,
	})

	newToken, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	updated := tokenFromOAuth(newToken)
	updated.Subject = ti.Subject
	updated.Email = ti.Email

	if err := SaveToken(updated); err != nil {
		return nil, fmt.Errorf("save refreshed token: %w", err)
	}

	return updated, nil
}

func tokenFromOAuth(t *oauth2.Token) *TokenInfo {
	return &TokenInfo{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	}
}

// openBrowser attempts to open uri in the default browser. Errors are silently
// ignored because the user can always open the URL manually.
func openBrowser(uri string) error {
	var cmdName string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
		args = []string{uri}
	case "linux":
		cmdName = "xdg-open"
		args = []string{uri}
	default:
		return nil
	}

	//nolint:gosec,noctx // cmdName is a fixed OS-specific open command, not user input
	return exec.Command(cmdName, args...).Start()
}
