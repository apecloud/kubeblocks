package auth

import "github.com/spf13/cobra"

// AuthCmd returns the base command for authentication.
func AuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <command>",
		Short: "Login and logout via the PlanetScale API",
		Long:  "Manage authentication",
	}

	cmd.AddCommand(LoginCmd())
	cmd.AddCommand(LogoutCmd())
	//cmd.AddCommand(CheckCmd())
	return cmd
}
