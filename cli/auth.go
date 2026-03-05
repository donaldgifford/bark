package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/bark/cli/internal/auth"
)

func newAuthCmd(getDeps func() *deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	cmd.AddCommand(newAuthLoginCmd(getDeps))
	cmd.AddCommand(newAuthStatusCmd(getDeps))
	cmd.AddCommand(newAuthLogoutCmd(getDeps))

	return cmd
}

func newAuthLoginCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in via OIDC device flow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d := getDeps()

			ti, err := d.authManager.Login(cmd.Context())
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			fmt.Printf("✓ Logged in as %s\n", identityDisplay(ti))

			return nil
		},
	}
}

func newAuthStatusCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(_ *cobra.Command, _ []string) error {
			d := getDeps()

			ti, err := d.authManager.Status()
			if err != nil {
				if errors.Is(err, auth.ErrNoToken) {
					fmt.Println("Not logged in. Run 'pkgtool auth login'.")
					return nil
				}

				return fmt.Errorf("check status: %w", err)
			}

			if ti.IsExpired() {
				fmt.Printf("Token expired (%s). Run 'pkgtool auth login'.\n", ti.Expiry.Format("2006-01-02 15:04"))
				return nil
			}

			fmt.Printf("Logged in as: %s\n", identityDisplay(ti))

			if !ti.Expiry.IsZero() {
				fmt.Printf("Token expires: %s\n", ti.Expiry.Format("2006-01-02 15:04 MST"))
			}

			return nil
		},
	}
}

func newAuthLogoutCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(_ *cobra.Command, _ []string) error {
			d := getDeps()

			if err := d.authManager.Logout(); err != nil {
				return fmt.Errorf("logout: %w", err)
			}

			fmt.Println("✓ Logged out.")

			return nil
		},
	}
}

// identityDisplay returns the best human-readable identity string from a token.
func identityDisplay(ti *auth.TokenInfo) string {
	if ti.Email != "" {
		return ti.Email
	}

	if ti.Subject != "" {
		return ti.Subject
	}

	return "(unknown)"
}
