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

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/authorize"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/utils"
	cloudctx "github.com/apecloud/kubeblocks/internal/cli/cmd/context"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

const (
	DefaultBaseURL = "https://tenent2.jp.auth0.com"
)

type LoginOptions struct {
	authorize.Options
	Region      string
	OrgName     string
	ContextName string

	Provider authorize.Provider
}

func NewLogin(streams genericiooptions.IOStreams) *cobra.Command {
	o := &LoginOptions{Options: authorize.Options{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the KubeBlocks Cloud",
		Run: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(o.complete())
			cobra.CheckErr(o.validate())
			cobra.CheckErr(o.run(cmd.Context()))
		},
	}

	cmd.Flags().StringVarP(&o.Region, "region", "r", "jp", "Specify the region [jp] to log in.")
	cmd.Flags().BoolVar(&o.NoBrowser, "no-browser", false, "Do not open the browser for authentication.")
	cmd.Flags().StringVarP(&o.OrgName, "org", "o", "", "Organization name.")
	cmd.Flags().StringVarP(&o.ContextName, "context", "c", "", "Context name.")
	return cmd
}

func (o *LoginOptions) complete() error {
	o.AuthURL = getAuthURL(o.Region)

	var err error
	o.Provider, err = authorize.NewTokenProvider(o.Options)
	if err != nil {
		return err
	}

	if o.ClientID == "" {
		return o.loadConfig()
	}
	return nil
}

func (o *LoginOptions) validate() error {
	if o.ClientID == "" {
		return fmt.Errorf("client-id is required")
	}
	return nil
}

func (o *LoginOptions) run(ctx context.Context) error {
	if o.OrgName != "" {
		if o.ContextName != "" {
			return o.loginWithOrgAndContext(ctx)
		}
		return o.loginWithOrg(ctx)
	}

	if o.ContextName != "" {
		return o.loginWithContext(ctx)
	}

	return o.loginWithDefault(ctx)
}

func (o *LoginOptions) login(ctx context.Context) (string, error) {
	if !utils.IsTTY() {
		return "", fmt.Errorf("the 'login' command requires an interactive shell")
	}

	userInfo, idToken, err := o.Provider.Login(ctx)
	if err != nil {
		return "", err
	}

	msg := fmt.Sprintf("Successfully logged in as \"%s\" for organization \"%s\" (\"%s\") \"%s\".", userInfo.Email, userInfo.Subject, userInfo.Name, userInfo.Locale)
	fmt.Fprintln(o.Out, msg)

	return idToken, nil
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

	o.Provider, err = authorize.NewTokenProvider(o.Options)
	if err != nil {
		return err
	}

	return nil
}

func (o *LoginOptions) loginWithOrg(ctx context.Context) error {
	token, err := o.login(ctx)
	if err != nil {
		return err
	}

	org := &organization.OrganizationOption{
		Organization: &organization.CloudOrganization{
			Token:   token,
			APIURL:  organization.APIURL,
			APIPath: organization.APIPath,
		},
	}
	if ok, err := org.Organization.IsValidOrganization(o.OrgName); !ok {
		return err
	}

	firstContextName := getFirstContext(token, o.OrgName)
	if err != nil {
		return err
	}

	return o.setCurrentConfig(o.OrgName, firstContextName)
}

func (o *LoginOptions) loginWithContext(ctx context.Context) error {
	token, err := o.login(ctx)
	if err != nil {
		return err
	}

	firstOrgName := getFirstOrg(token)
	if err != nil {
		return err
	}

	return o.setCurrentConfig(firstOrgName, o.ContextName)
}

func (o *LoginOptions) loginWithOrgAndContext(ctx context.Context) error {
	_, err := o.login(ctx)
	if err != nil {
		return err
	}
	return o.setCurrentConfig(o.OrgName, o.ContextName)
}

func (o *LoginOptions) loginWithDefault(ctx context.Context) error {
	token, err := o.login(ctx)
	if err != nil {
		return err
	}

	firstOrgName := getFirstOrg(token)
	if err != nil {
		return err
	}

	firstContextName := getFirstContext(token, firstOrgName)
	if err != nil {
		return err
	}

	return o.setCurrentConfig(firstOrgName, firstContextName)
}

func (o *LoginOptions) setCurrentConfig(orgName, contextName string) error {
	currentOrgAndContext := organization.CurrentOrgAndContext{
		CurrentOrganization: orgName,
		CurrentContext:      contextName,
	}

	err := organization.SetCurrentOrgAndContext(&currentOrgAndContext)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "You are now logged in to the organization: %s and the context: %s.\n", orgName, contextName)
	return nil
}

func getFirstOrg(token string) string {
	org := &organization.OrganizationOption{
		Organization: &organization.CloudOrganization{
			Token:   token,
			APIURL:  organization.APIURL,
			APIPath: organization.APIPath,
		},
	}
	organizations, err := org.Organization.GetOrganizations()
	if err != nil {
		return ""
	}

	if organizations != nil && len(organizations.Items) > 0 {
		return organizations.Items[0].Name
	}
	return ""
}

func getFirstContext(token string, orgName string) string {
	c := &cloudctx.CloudContext{
		OrgName: orgName,
		Token:   token,
		APIURL:  organization.APIURL,
		APIPath: organization.APIPath,
	}
	contexts, err := c.GetContexts()
	if err != nil {
		return ""
	}

	if contexts != nil {
		return contexts.Items[0].Metadata.Name
	}
	return ""
}

func IsLoggedIn() bool {
	cached := authorize.NewKeyringCachedTokenProvider(nil)
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

func getAuthURL(region string) string {
	var authURL string
	switch region {
	case "jp":
		authURL = DefaultBaseURL
	default:
		authURL = DefaultBaseURL
	}
	return authURL
}
