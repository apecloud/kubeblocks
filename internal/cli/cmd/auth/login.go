/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/authorize"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/utils"
)

const (
	// DefaultBaseURL = "https://tenent2.jp.auth0.com"
	DefaultBaseURL = "https://dev-0cf3xqbt63n7rs7t.us.auth0.com"
	clientID       = "lYbU8d2i8WqsM1YszomQZPuvg5F4MIgS"
)

type LoginOptions struct {
	authorize.Options

	Provider authorize.Provider
}

func NewLogin(streams genericclioptions.IOStreams) *cobra.Command {
	o := &LoginOptions{Options: authorize.Options{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Kubeblocks Cloud",
		Run: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(o.complete())
			cobra.CheckErr(o.validate())
			cobra.CheckErr(o.run(cmd.Context()))
		},
	}

	cmd.Flags().StringVar(&o.ClientID, "client-id", clientID, "The client ID for the Kubeblocks CLI application.")
	cmd.Flags().StringVar(&o.AuthURL, "site", DefaultBaseURL, "The Kubeblocks Auth API base URL.")
	cmd.Flags().BoolVar(&o.NoBrowser, "no-browser", false, "Do not open the browser for authentication.")
	return cmd
}

func (o *LoginOptions) complete() error {
	o.Provider = authorize.NewTokenProvider(o.Options)
	if o.ClientID == "" {
		return o.loadConfig()
	}
	return nil
}

func (o *LoginOptions) validate() error {
	return nil
}

func (o *LoginOptions) run(ctx context.Context) error {
	if !utils.IsTTY() {
		return errors.New("the 'login' command requires an interactive shell")
	}

	userInfo, err := o.Provider.Login(ctx)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Successfully logged in as \"%s\" for organization \"%s\" (\"%s\") \"%s\".", userInfo.Email, userInfo.Subject, userInfo.Name, userInfo.Locale)
	fmt.Fprint(o.Out, msg)

	return nil
}

func (o *LoginOptions) loadConfig() error {
	data, err := utils.Asset("config/config.enc")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &o.Options)
	if err != nil {
		return err
	}

	o.Provider = authorize.NewTokenProvider(o.Options)
	return nil
}

func IsLoggedIn() bool {
	cached := authorize.NewKeyringCachedTokenProvider()
	tokenResult, err := cached.GetTokens()
	if err != nil {
		return false
	}
	if tokenResult == nil {
		return false
	}

	if !authorize.IsValidToken(tokenResult.AccessToken) {
		return false
	}

	return checkTokenAvailable(tokenResult.AccessToken, DefaultBaseURL)
}

// CheckTokenAvailable Check whether the token is available by getting user info.
func checkTokenAvailable(token, domain string) bool {
	URL := fmt.Sprintf("%s/userinfo", domain)
	req, err := utils.NewRequest(context.TODO(), URL, url.Values{
		"access_token": []string{token},
	})
	if err != nil {
		return false
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}
	_, err = io.ReadAll(resp.Body)

	return err == nil
}
