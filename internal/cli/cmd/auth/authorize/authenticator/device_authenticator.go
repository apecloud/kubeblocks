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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
)

// DeviceAuthenticator performs the authentication flow for logging in.
type DeviceAuthenticator struct {
	client       *http.Client
	AuthURL      *url.URL
	AuthAudience string
	Clock        clock.Clock
	ClientID     string
}

type DeviceVerification struct {
	// DeviceCode is the unique code for the device. When the user goes to the VerificationURL in their browser-based device, this code will be bound to their session.
	DeviceCode string
	// UserCode contains the code that should be input at the VerificationURL to authorize the device.
	UserCode string
	// VerificationURL contains the URL the user should visit to authorize the device.
	VerificationURL string
	// VerificationCompleteURL contains the complete URL the user should visit to authorize the device. This allows your app to embed the user_code in the URL, if you so choose.
	VerificationCompleteURL string
	// CheckInterval indicates the interval (in seconds) at which the app should poll the token URL to request a token.
	Interval time.Duration
	// ExpiresAt indicates the lifetime (in seconds) of the device_code and user_code.
	ExpiresAt time.Time
}

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationCompleteURI string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	PollingInterval         int    `json:"interval"`
}

type ErrorResponse struct {
	ErrorCode   string `json:"error"`
	Description string `json:"error_description"`
}

func (e ErrorResponse) Error() string {
	return e.Description
}

func newDeviceAuthenticator(client *http.Client, clientID string, authURL string) (*DeviceAuthenticator, error) {
	if client == nil {
		client = cleanhttp.DefaultClient()
	}

	baseURL, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}

	authenticator := &DeviceAuthenticator{
		client:       client,
		AuthURL:      baseURL,
		AuthAudience: authURL + "/api/v2/",
		Clock:        clock.New(),
		ClientID:     clientID,
	}
	return authenticator, nil
}

// GetAuthorization performs the device verification API calls.
func (d *DeviceAuthenticator) GetAuthorization(ctx context.Context, openURLFunc func(URL string), states ...string) (interface{}, error) {
	req, err := d.newRequest(ctx, "oauth/device/code", url.Values{
		"client_id": []string{d.ClientID},
		"audience":  []string{d.AuthAudience},
		"scope": []string{strings.Join([]string{
			"read_databases", "write_databases", "read_user", "read_organization", "offline_access", "openid", "profile", "email",
		}, " ")},
	})
	if err != nil {
		return nil, err
	}
	res, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if _, err = checkErrorResponse(res); err != nil {
		return nil, err
	}

	deviceCodeRes := &DeviceCodeResponse{}
	err = json.NewDecoder(res.Body).Decode(deviceCodeRes)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding device code response")
	}

	interval := time.Duration(deviceCodeRes.PollingInterval) * time.Second
	if interval == 0 {
		interval = time.Duration(5) * time.Second
	}

	expiresAt := d.Clock.Now().Add(time.Duration(deviceCodeRes.ExpiresIn) * time.Second)

	openURLFunc(deviceCodeRes.VerificationURI)

	return &DeviceVerification{
		DeviceCode:              deviceCodeRes.DeviceCode,
		UserCode:                deviceCodeRes.UserCode,
		VerificationCompleteURL: deviceCodeRes.VerificationCompleteURI,
		VerificationURL:         deviceCodeRes.VerificationURI,
		ExpiresAt:               expiresAt,
		Interval:                interval,
	}, nil
}

func (d *DeviceAuthenticator) GetToken(ctx context.Context, authorization interface{}) (*TokenResponse, error) {
	v, ok := authorization.(*DeviceVerification)
	if !ok {
		return nil, errors.New("invalid authorization")
	}

	for {
		// This loop begins right after we open the user's browser to send an
		// authentication code. We don't request a token immediately because the
		// has to complete that authentication flow before we can provide a
		// token anyway.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(v.Interval):
		}
		tokenResponse, err := d.requestToken(ctx, v.DeviceCode, d.ClientID)
		if err != nil {
			// Fatal error.
			return nil, err
		}

		if tokenResponse != nil {
			return tokenResponse, nil
		}

		if tokenResponse == nil && d.Clock.Now().After(v.ExpiresAt) {
			return nil, errors.New("authentication timed out")
		}
	}
}

func (d *DeviceAuthenticator) requestToken(ctx context.Context, deviceCode string, clientID string) (*TokenResponse, error) {
	req, err := d.newRequest(ctx, "oauth/token", url.Values{
		"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": []string{deviceCode},
		"client_id":   []string{clientID},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating request")
	}

	res, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error performing http request")
	}
	defer res.Body.Close()

	isRetryable, err := checkErrorResponse(res)
	if err != nil {
		return nil, err
	}

	if isRetryable {
		return nil, nil
	}

	tokenRes := &TokenResponse{}

	err = json.NewDecoder(res.Body).Decode(tokenRes)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding token response")
	}
	return tokenRes, nil
}

func (d *DeviceAuthenticator) GetUserInfo(ctx context.Context, token string) (*UserInfoResponse, error) {
	req, err := d.newRequest(ctx, "userinfo", url.Values{
		"access_token": []string{token},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating request")
	}

	res, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "error performing http request")
	}
	defer res.Body.Close()

	userInfo := &UserInfoResponse{}
	err = json.NewDecoder(res.Body).Decode(userInfo)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding userinfo")
	}

	return userInfo, err
}

// RefreshToken The device authenticator needs the clientSecret when refreshing the token,
// and the kbcli client does not hold it, so this method does not need to be implemented.
func (d *DeviceAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	return nil, nil
}

func (d *DeviceAuthenticator) Logout(ctx context.Context, token string, openURLFunc func(URL string)) error {
	req, err := d.newRequest(ctx, "oidc/logout", url.Values{
		"id_token_hint": []string{token},
		"client_id":     []string{d.ClientID},
	})
	if err != nil {
		return errors.Wrap(err, "error creating request")
	}

	res, err := d.client.Do(req)
	fmt.Println(res.StatusCode)
	if err != nil {
		return errors.Wrap(err, "error performing http request")
	}
	defer res.Body.Close()
	if _, err = checkErrorResponse(res); err != nil {
		return err
	}

	logoutURL := fmt.Sprintf(d.AuthURL.Path + "/oidc/logout?federated")
	openURLFunc(logoutURL)

	return nil
}

func (d *DeviceAuthenticator) newRequest(ctx context.Context, path string, payload url.Values) (*http.Request, error) {
	u, err := d.AuthURL.Parse(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u.String(),
		strings.NewReader(payload.Encode()),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// CheckErrorResponse returns whether the error is retryable or not and the error itself.
func checkErrorResponse(res *http.Response) (bool, error) {
	if res.StatusCode < 400 {
		return false, nil
	}

	// Client or server error.
	errorRes := &ErrorResponse{}
	err := json.NewDecoder(res.Body).Decode(errorRes)
	if err != nil {
		return false, errors.Wrap(err, "error decoding response")
	}

	// Authentication is not yet complete or requests need to be slowed down.
	if errorRes.ErrorCode == "authorization_pending" || errorRes.ErrorCode == "slow_down" {
		return true, nil
	}

	return false, errorRes
}
