package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInstallCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "install <package>[@version]",
		Short: "Install a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d := getDeps()
			if d.installer == nil {
				return fmt.Errorf("store unavailable; check ~/.pkgtool/store permissions")
			}

			name, version := splitPackageArg(args[0])

			return d.installer.Install(cmd.Context(), name, version)
		},
	}
}

func newUninstallCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <package>[@version]",
		Short: "Uninstall a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			d := getDeps()
			if d.installer == nil {
				return fmt.Errorf("store unavailable")
			}

			name, version := splitPackageArg(args[0])

			return d.installer.Uninstall(name, version)
		},
	}
}

func newListCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed packages",
		RunE: func(_ *cobra.Command, _ []string) error {
			d := getDeps()
			if d.installer == nil {
				return fmt.Errorf("store unavailable")
			}

			pkgs, err := d.installer.List()
			if err != nil {
				return fmt.Errorf("list packages: %w", err)
			}

			if len(pkgs) == 0 {
				fmt.Println("No packages installed.")
				return nil
			}

			for _, p := range pkgs {
				fmt.Printf("  %s@%s\n", p.Name, p.Version)
			}

			return nil
		},
	}
}

func newSearchCmd(getDeps func() *deps) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search for packages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d := getDeps()

			resp, err := d.apiClient.Search(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			if len(resp.Results) == 0 {
				fmt.Printf("No results for %q.\n", args[0])
				return nil
			}

			for _, p := range resp.Results {
				fmt.Printf("  %-30s %s [%s]\n", p.Name, p.Description, p.Tier)
			}

			return nil
		},
	}
}

// splitPackageArg splits "name@version" into (name, version).
// If no @ is present, version is empty.
func splitPackageArg(arg string) (name, version string) {
	for i, c := range arg {
		if c == '@' {
			return arg[:i], arg[i+1:]
		}
	}

	return arg, ""
}
