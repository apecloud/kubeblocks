/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kafka

import (
	ctx "context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	"golang.org/x/oauth2"
	ccred "golang.org/x/oauth2/clientcredentials"
)

type OAuthTokenSource struct {
	CachedToken   oauth2.Token
	Extensions    map[string]string
	TokenEndpoint oauth2.Endpoint
	ClientID      string
	ClientSecret  string
	Scopes        []string
	httpClient    *http.Client
	trustedCas    []*x509.Certificate
	skipCaVerify  bool
}

func newOAuthTokenSource(oidcTokenEndpoint, oidcClientID, oidcClientSecret string, oidcScopes []string) OAuthTokenSource {
	return OAuthTokenSource{TokenEndpoint: oauth2.Endpoint{TokenURL: oidcTokenEndpoint}, ClientID: oidcClientID, ClientSecret: oidcClientSecret, Scopes: oidcScopes}
}

var tokenRequestTimeout, _ = time.ParseDuration("30s")

func (ts *OAuthTokenSource) addCa(caPem string) error {
	pemBytes := []byte(caPem)

	block, _ := pem.Decode(pemBytes)

	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("PEM data not valid or not of a valid type (CERTIFICATE)")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing PEM certificate: %w", err)
	}

	if ts.trustedCas == nil {
		ts.trustedCas = make([]*x509.Certificate, 0)
	}
	ts.trustedCas = append(ts.trustedCas, caCert)

	return nil
}

func (ts *OAuthTokenSource) configureClient() {
	if ts.httpClient != nil {
		return
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: ts.skipCaVerify, //nolint:gosec
	}

	if ts.trustedCas != nil {
		caPool, err := x509.SystemCertPool()
		if err != nil {
			caPool = x509.NewCertPool()
		}

		for _, c := range ts.trustedCas {
			caPool.AddCert(c)
		}
		tlsConfig.RootCAs = caPool
	}

	ts.httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

func (ts *OAuthTokenSource) Token() (*sarama.AccessToken, error) {
	if ts.CachedToken.Valid() {
		return ts.asSaramaToken(), nil
	}

	if ts.TokenEndpoint.TokenURL == "" || ts.ClientID == "" || ts.ClientSecret == "" {
		return nil, fmt.Errorf("cannot generate token, OAuthTokenSource not fully configured")
	}

	oidcCfg := ccred.Config{ClientID: ts.ClientID, ClientSecret: ts.ClientSecret, Scopes: ts.Scopes, TokenURL: ts.TokenEndpoint.TokenURL, AuthStyle: ts.TokenEndpoint.AuthStyle}

	timeoutCtx, cancel := ctx.WithTimeout(ctx.TODO(), tokenRequestTimeout)
	defer cancel()

	ts.configureClient()

	timeoutCtx = ctx.WithValue(timeoutCtx, oauth2.HTTPClient, ts.httpClient)

	token, err := oidcCfg.Token(timeoutCtx)
	if err != nil {
		return nil, fmt.Errorf("error generating oauth2 token: %w", err)
	}

	ts.CachedToken = *token
	return ts.asSaramaToken(), nil
}

func (ts *OAuthTokenSource) asSaramaToken() *sarama.AccessToken {
	return &(sarama.AccessToken{Token: ts.CachedToken.AccessToken, Extensions: ts.Extensions})
}
