package auth

import (
	"github.com/spf13/cobra"
)

func LogoutCmd() *cobra.Command {
	var clientID string
	var clientSecret string
	var apiURL string

	cmd := &cobra.Command{
		Use:   "logout",
		Args:  cobra.NoArgs,
		Short: "Log out of the PlanetScale API",
		RunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", OAuthClientID, "The client ID for the PlanetScale CLI application.")
	cmd.Flags().StringVar(&clientSecret, "client-secret", OAuthClientSecret, "The client ID for the PlanetScale CLI application")
	cmd.Flags().StringVar(&apiURL, "api-url", DefaultBaseURL, "The PlanetScale base API URL.")
	return cmd
}
