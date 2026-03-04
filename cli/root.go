// Package cli provides the pkgtool CLI commands.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/bark/cli/internal/apiclient"
	"github.com/donaldgifford/bark/cli/internal/auth"
	"github.com/donaldgifford/bark/cli/internal/install"
	"github.com/donaldgifford/bark/cli/internal/store"
	"github.com/donaldgifford/bark/cli/internal/verifier"
)

// Execute is the entry point for the pkgtool CLI.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// deps bundles runtime dependencies shared across commands.
type deps struct {
	authManager *auth.Manager
	apiClient   *apiclient.Client
	installer   *install.Installer
}

func newRootCmd() *cobra.Command {
	var (
		apiURL   string
		logLevel string
		prefix   string
	)

	root := &cobra.Command{
		Use:   "pkgtool",
		Short: "bark package manager — install and manage internal tooling",
		Long: `pkgtool is the developer-facing CLI for the bark internal package manager.
It authenticates via OIDC, fetches signed package manifests, and installs
verified tools into ~/.pkgtool/prefix.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&apiURL, "api-url", envOr("BARK_API_URL", ""), "bark API base URL")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	root.PersistentFlags().StringVar(&prefix, "prefix", "", "install prefix (default ~/.pkgtool/prefix)")

	root.AddCommand(newAuthCmd(func() *deps {
		return buildDeps(apiURL, prefix)
	}))
	root.AddCommand(newInstallCmd(func() *deps {
		return buildDeps(apiURL, prefix)
	}))
	root.AddCommand(newUninstallCmd(func() *deps {
		return buildDeps(apiURL, prefix)
	}))
	root.AddCommand(newListCmd(func() *deps {
		return buildDeps(apiURL, prefix)
	}))
	root.AddCommand(newSearchCmd(func() *deps {
		return buildDeps(apiURL, prefix)
	}))

	return root
}

// buildDeps constructs all runtime dependencies from the given configuration.
func buildDeps(apiURL, prefix string) *deps {
	if prefix == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.TempDir()
		}

		prefix = filepath.Join(home, ".pkgtool", "prefix")
	}

	authMgr := auth.NewManager(auth.OIDCConfig{
		Issuer:   envOr("BARK_OIDC_ISSUER", ""),
		ClientID: envOr("BARK_OIDC_CLIENT_ID", "pkgtool"),
	})

	apiClient := apiclient.New(apiURL, nil, authMgr.GetToken)

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}

	storePath := filepath.Join(home, ".pkgtool", "store")

	st, err := store.New(storePath)
	if err != nil {
		// If we can't create the store, we'll fail later when installing.
		st = nil
	}

	v := verifier.NewVerifier(verifier.Config{
		APIURL: apiURL,
	})

	var installer *install.Installer
	if st != nil {
		installer = install.New(apiClient, st, v, prefix)
	}

	return &deps{
		authManager: authMgr,
		apiClient:   apiClient,
		installer:   installer,
	}
}

// envOr returns the value of the env var key, or fallback if not set.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
