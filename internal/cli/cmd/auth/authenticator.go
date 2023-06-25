package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
)

const (
	DefaultBaseURL = "https://auth.planetscale.com/"
	OAuthClientID  = "wzzkYKOfRcxFAiMgDgfbhO9yIikNIlt9-yhosmvPBQA"

	// This is safe to be committed to version control, since our OAuth
	// Application isn't confidential.
	OAuthClientSecret = "eIDdgw21BYsovcrpC4iKZQ0o7ol9cN1LsSr8fuNyg5o"

	formMediaType = "application/x-www-form-urlencoded"
	jsonMediaType = "application/json"
)

// Authenticator is the interface for authentication via device oauth
type Authenticator interface {
	VerifyDevice(ctx context.Context) (*DeviceVerification, error)
	GetAccessTokenForDevice(ctx context.Context, v DeviceVerification) (string, error)
	RevokeToken(ctx context.Context, token string) error
}

var _ Authenticator = (*DeviceAuthenticator)(nil)

type AuthenticatorOption func(c *DeviceAuthenticator) error

// SetBaseURL overrides the base URL for the DeviceAuthenticator.
func SetBaseURL(baseURL string) AuthenticatorOption {
	return func(d *DeviceAuthenticator) error {
		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			return err
		}

		d.BaseURL = parsedURL
		return nil
	}
}

// WithMockClock replaces the clock on the authenticator with a mock clock.
func WithMockClock(mock *clock.Mock) AuthenticatorOption {
	return func(d *DeviceAuthenticator) error {
		d.Clock = mock
		return nil
	}
}

// DeviceCodeResponse encapsulates the response for obtaining a device code.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationCompleteURI string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	PollingInterval         int    `json:"interval"`
}

// DeviceVerification represents the response from verifying a device.
type DeviceVerification struct {
	DeviceCode              string
	UserCode                string
	VerificationURL         string
	VerificationCompleteURL string
	CheckInterval           time.Duration
	ExpiresAt               time.Time
}

// ErrorResponse is an error response from the API.
type ErrorResponse struct {
	ErrorCode   string `json:"error"`
	Description string `json:"error_description"`
}

func (e ErrorResponse) Error() string {
	return e.Description
}

// DeviceAuthenticator performs the authentication flow for logging in.
type DeviceAuthenticator struct {
	client       *http.Client
	BaseURL      *url.URL
	Clock        clock.Clock
	ClientID     string
	ClientSecret string
}

// New returns an instance of the DeviceAuthenticator
func New(client *http.Client, clientID, clientSecret string, opts ...AuthenticatorOption) (*DeviceAuthenticator, error) {
	if client == nil {
		client = cleanhttp.DefaultClient()
	}

	baseURL, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return nil, err
	}

	authenticator := &DeviceAuthenticator{
		client:       client,
		BaseURL:      baseURL,
		Clock:        clock.New(),
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	for _, opt := range opts {
		err := opt(authenticator)
		if err != nil {
			return nil, err
		}
	}

	return authenticator, nil
}

// VerifyDevice performs the device verification API calls.
func (d *DeviceAuthenticator) VerifyDevice(ctx context.Context) (*DeviceVerification, error) {
	req, err := d.newFormRequest(ctx, "oauth/authorize_device", url.Values{
		"client_id": []string{d.ClientID},
		// Scopes are concatenated with strings and ultimately URL-encoded for
		// use by the server.
		"scope": []string{strings.Join([]string{
			"read_databases", "write_databases", "read_user", "read_organization",
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

	checkInterval := time.Duration(deviceCodeRes.PollingInterval) * time.Second
	if checkInterval == 0 {
		checkInterval = time.Duration(5) * time.Second
	}

	expiresAt := d.Clock.Now().Add(time.Duration(deviceCodeRes.ExpiresIn) * time.Second)

	return &DeviceVerification{
		DeviceCode:              deviceCodeRes.DeviceCode,
		UserCode:                deviceCodeRes.UserCode,
		VerificationCompleteURL: deviceCodeRes.VerificationCompleteURI,
		VerificationURL:         deviceCodeRes.VerificationURI,
		ExpiresAt:               expiresAt,
		CheckInterval:           checkInterval,
	}, nil
}

// GetAccessTokenForDevice uses the device verification response to fetch an
// access token.
func (d *DeviceAuthenticator) GetAccessTokenForDevice(ctx context.Context, v DeviceVerification) (string, error) {
	for {
		// This loop begins right after we open the user's browser to send an
		// authentication code. We don't request a token immediately because the
		// has to complete that authentication flow before we can provide a
		// token anyway.
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(v.CheckInterval):
			// Ready to check again.
		}

		token, err := d.requestToken(ctx, v.DeviceCode, d.ClientID)
		if err != nil {
			// Fatal error.
			return "", err
		}
		if token != "" {
			// Successful authentication.
			return token, nil
		}

		if token == "" && d.Clock.Now().After(v.ExpiresAt) {
			return "", errors.New("authentication timed out")
		}
	}
}

// OAuthTokenResponse contains the information returned after fetching an access
// token for a device.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (d *DeviceAuthenticator) requestToken(ctx context.Context, deviceCode string, clientID string) (string, error) {
	req, err := d.newFormRequest(ctx, "oauth/token", url.Values{
		"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": []string{deviceCode},
		"client_id":   []string{clientID},
	})
	if err != nil {
		return "", errors.Wrap(err, "error creating request")
	}

	res, err := d.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "error performing http request")
	}
	defer res.Body.Close()

	isRetryable, err := checkErrorResponse(res)
	if err != nil {
		return "", err
	}

	// Bail early so the token fetching is retried.
	if isRetryable {
		return "", nil
	}

	tokenRes := &OAuthTokenResponse{}

	err = json.NewDecoder(res.Body).Decode(tokenRes)
	if err != nil {
		return "", errors.Wrap(err, "error decoding token response")
	}

	return tokenRes.AccessToken, nil
}

// RevokeToken revokes an access token.
func (d *DeviceAuthenticator) RevokeToken(ctx context.Context, token string) error {
	req, err := d.newFormRequest(ctx, "oauth/revoke", url.Values{
		"client_id":     []string{d.ClientID},
		"client_secret": []string{d.ClientSecret},
		"token":         []string{token},
	})
	if err != nil {
		return errors.Wrap(err, "error creating request")
	}

	res, err := d.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "error performing http request")
	}
	defer res.Body.Close()

	if _, err = checkErrorResponse(res); err != nil {
		return err
	}
	return nil
}

// newFormRequest creates a new form URL encoded request
func (d *DeviceAuthenticator) newFormRequest(
	ctx context.Context,
	path string,
	payload url.Values,
) (*http.Request, error) {
	u, err := d.BaseURL.Parse(path)
	if err != nil {
		return nil, err
	}

	// Emulate the format of data sent by http.Client's PostForm method, but
	// also preserve context support.
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u.String(),
		strings.NewReader(payload.Encode()),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", formMediaType)
	req.Header.Set("Accept", jsonMediaType)
	return req, nil
}

// checkErrorResponse returns whether the error is retryable or not and the
// error itself.
func checkErrorResponse(res *http.Response) (bool, error) {
	if res.StatusCode < 400 {
		// 200 OK, etc.
		return false, nil
	}

	// Client or server error.
	errorRes := &ErrorResponse{}
	err := json.NewDecoder(res.Body).Decode(errorRes)
	if err != nil {
		return false, errors.Wrap(err, "error decoding error response")
	}

	// If we're polling and haven't authorized yet or we need to slow down, we
	// don't wanna terminate the polling
	if errorRes.ErrorCode == "authorization_pending" || errorRes.ErrorCode == "slow_down" {
		return true, nil
	}

	return false, errorRes
}
