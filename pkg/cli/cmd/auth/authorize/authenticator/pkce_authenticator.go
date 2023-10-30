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

package authenticator

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/benbjohnson/clock"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/auth/utils"
)

type OIDCWellKnownEndpoints struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

type PKCEAuthenticator struct {
	client             *http.Client
	Clock              clock.Clock
	ClientID           string
	AuthURL            string
	AuthAudience       string
	Challenge          Challenge
	WellKnownEndpoints *OIDCWellKnownEndpoints
}

// Challenge holds challenge and verification data needed for the PKCE flow
type Challenge struct {
	Code     string
	Verifier string
	Method   string
}

// CallbackResponse holds the code gotten from the authorization callback.
// Error will hold an error struct if an error occurred.
type CallbackResponse struct {
	Code  string
	Error error
}

type AuthorizationResponse struct {
	CallbackURL string
	Code        string
}

func newPKCEAuthenticator(client *http.Client, clientID string, authURL string) (*PKCEAuthenticator, error) {
	if client == nil {
		client = cleanhttp.DefaultClient()
	}
	p := &PKCEAuthenticator{
		client:       client,
		Clock:        clock.New(),
		ClientID:     clientID,
		AuthURL:      authURL,
		AuthAudience: authURL + "/api/v2/",
	}
	var err error
	p.Challenge, err = defaultChallengeGenerator()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate PKCE challenge")
	}

	return p, nil
}

func (p *PKCEAuthenticator) GetAuthorization(ctx context.Context, openURLFunc func(URL string), states ...string) (interface{}, error) {
	callbackService := newCallbackService("8000")
	codeReceiverCh := make(chan CallbackResponse)
	defer close(codeReceiverCh)

	var state string
	var err error
	if states == nil {
		state, err = defaultStateGenerator()
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate state")
		}
	} else {
		state = states[0]
	}

	go callbackService.awaitResponse(codeReceiverCh, state)

	var endpoint string
	if err = p.getOIDCWellKnownEndpoints(p.AuthURL); err != nil {
		endpoint = strings.Join([]string{p.AuthURL, "authorization"}, "/")
	} else {
		endpoint = p.WellKnownEndpoints.AuthorizationEndpoint
	}

	params := url.Values{
		"audience":              []string{p.AuthAudience},
		"client_id":             []string{p.ClientID},
		"code_challenge":        []string{p.Challenge.Code},
		"code_challenge_method": []string{p.Challenge.Method},
		"response_type":         []string{"code"},
		"state":                 []string{state},
		"redirect_uri":          []string{callbackService.getCallbackURL()},
		"scope": []string{strings.Join([]string{
			"read_databases", "write_databases", "read_user", "read_organization", "offline_access", "openid", "profile", "email",
		}, " ")},
	}

	URL := fmt.Sprintf("%s?%s",
		endpoint,
		params.Encode(),
	)
	openURLFunc(URL)

	callbackResult, ok := <-codeReceiverCh
	if !ok {
		return nil, errors.New("codeReceiverCh closed")
	}
	if callbackResult.Error != nil {
		return nil, callbackResult.Error
	}
	callbackService.close()

	return &AuthorizationResponse{
		Code:        callbackResult.Code,
		CallbackURL: callbackService.getCallbackURL(),
	}, nil
}

func (p *PKCEAuthenticator) GetToken(ctx context.Context, authorization interface{}) (*TokenResponse, error) {
	authorize, ok := authorization.(*AuthorizationResponse)
	if !ok {
		return nil, errors.New("invalid authorization response")
	}

	var endpoint string
	if err := p.getOIDCWellKnownEndpoints(p.AuthURL); err != nil {
		endpoint = strings.Join([]string{p.AuthURL, "oauth/token"}, "/")
	} else {
		endpoint = p.WellKnownEndpoints.TokenEndpoint
	}

	req, err := utils.NewRequest(ctx, endpoint, url.Values{
		"grant_type":    []string{"authorization_code"},
		"code_verifier": []string{p.Challenge.Verifier},
		"client_id":     []string{p.ClientID},
		"code":          []string{authorize.Code},
		"redirect_uri":  []string{authorize.CallbackURL},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating request for token")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error performing http request for token")
	}
	defer res.Body.Close()

	if _, err = checkErrorResponse(res); err != nil {
		return nil, err
	}

	tokenRes := &TokenResponse{}
	err = json.NewDecoder(res.Body).Decode(tokenRes)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding token response")
	}

	return tokenRes, nil
}

func (p *PKCEAuthenticator) GetUserInfo(ctx context.Context, token string) (*UserInfoResponse, error) {
	URL := fmt.Sprintf("https://%s/api/v1/user", utils.OpenAPIHost)
	req, err := utils.NewFullRequest(ctx, URL, http.MethodGet, map[string]string{
		"Authorization": "Bearer " + token,
	}, url.Values{})
	if err != nil {
		return nil, errors.Wrap(err, "error creating request for userinfo")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error performing http request for userinfo")
	}
	defer res.Body.Close()

	if _, err = checkErrorResponse(res); err != nil {
		return nil, err
	}

	userInfo := &UserInfoResponse{}
	err = json.NewDecoder(res.Body).Decode(userInfo)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding userinfo")
	}

	return userInfo, err
}

func (p *PKCEAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	var endpoint string
	if err := p.getOIDCWellKnownEndpoints(p.AuthURL); err != nil {
		endpoint = strings.Join([]string{p.AuthURL, "oauth/token"}, "/")
	} else {
		endpoint = p.WellKnownEndpoints.TokenEndpoint
	}

	req, err := utils.NewRequest(ctx, endpoint, url.Values{
		"grant_type":    []string{"refresh_token"},
		"client_id":     []string{p.ClientID},
		"refresh_token": []string{refreshToken},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating request for refresh token")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error performing http request for refresh token")
	}
	defer res.Body.Close()

	if _, err = checkErrorResponse(res); err != nil {
		return nil, err
	}

	refreshTokenRes := &RefreshTokenResponse{}
	err = json.NewDecoder(res.Body).Decode(refreshTokenRes)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding refresh token response")
	}

	return &TokenResponse{
		AccessToken:  refreshTokenRes.AccessToken,
		IDToken:      refreshTokenRes.IDToken,
		RefreshToken: refreshToken,
		ExpiresIn:    refreshTokenRes.ExpiresIn,
	}, nil
}

func (p *PKCEAuthenticator) Logout(ctx context.Context, token string, openURLFunc func(URL string)) error {
	URL := fmt.Sprintf("%s/oidc/logout", p.AuthURL)
	req, err := utils.NewRequest(ctx, URL, url.Values{
		"id_token_hint": []string{token},
		"client_id":     []string{p.ClientID},
	})
	if err != nil {
		return errors.Wrap(err, "error creating request for logout")
	}

	res, err := p.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "error performing http request for logout")
	}
	defer res.Body.Close()

	if _, err = checkErrorResponse(res); err != nil {
		return err
	}

	logoutURL := fmt.Sprintf(p.AuthURL + "/oidc/logout?federated")
	openURLFunc(logoutURL)

	return nil
}

// Get authorize endpoint and token endpoint
func (p *PKCEAuthenticator) getOIDCWellKnownEndpoints(authURL string) error {
	u, err := url.Parse(authURL)
	if err != nil {
		return errors.Wrap(err, "could not parse issuer url to build well known endpoints")
	}
	u.Path = path.Join(u.Path, ".well-known/openid-configuration")

	r, err := http.Get(u.String())
	if err != nil {
		return errors.Wrapf(err, "could not get well known endpoints from url %s", u.String())
	}

	if _, err = checkErrorResponse(r); err != nil {
		return err
	}

	var wkEndpoints OIDCWellKnownEndpoints
	err = json.NewDecoder(r.Body).Decode(&wkEndpoints)
	if err != nil {
		return errors.Wrap(err, "could not decode json body when getting well known endpoints")
	}

	p.WellKnownEndpoints = &wkEndpoints
	return nil
}

// generateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateChallenge(length int) (Challenge, error) {
	c := Challenge{}

	var err error
	c.Verifier, err = generateRandomString(length)
	if err != nil {
		return c, err
	}

	sum := sha256.Sum256([]byte(c.Verifier))
	c.Code = base64.RawURLEncoding.EncodeToString(sum[:])
	c.Method = "S256"

	return c, nil
}

func defaultChallengeGenerator() (Challenge, error) {
	return generateChallenge(32)
}

func defaultStateGenerator() (string, error) {
	return generateRandomString(32)
}
