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
