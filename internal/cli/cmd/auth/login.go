package auth

import (
	"github.com/spf13/cobra"
)

// LoginCmd is the command for logging into a PlanetScale account.
func LoginCmd() *cobra.Command {
	var clientID string
	var clientSecret string
	var authURL string

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.ExactArgs(0),
		Short: "Authenticate with the PlanetScale API",
		RunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", OAuthClientID, "The client ID for the PlanetScale CLI application.")
	cmd.Flags().StringVar(&clientSecret, "client-secret", OAuthClientSecret, "The client ID for the PlanetScale CLI application")
	cmd.Flags().StringVar(&authURL, "api-url", DefaultBaseURL, "The PlanetScale Auth API base URL.")

	return cmd
}
