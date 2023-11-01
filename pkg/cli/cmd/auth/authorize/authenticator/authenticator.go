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
	"fmt"
	"net/http"
)

const (
	PKCE   = "pkce"
	Device = "device"
)

type Authenticator interface {
	GetAuthorization(ctx context.Context, openURLFunc func(URL string), states ...string) (interface{}, error)
	GetToken(ctx context.Context, authorization interface{}) (*TokenResponse, error)
	GetUserInfo(ctx context.Context, token string) (*UserInfoResponse, error)
	Logout(ctx context.Context, tokenResult *TokenResponse, openURLFunc func(URL string)) error
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

type UserInfoResponse struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Locale  string `json:"locale"`
	Subject string `json:"sub"`
}

func NewAuthenticator(typeAuth string, client *http.Client, clientID string, authURL string) (Authenticator, error) {
	if typeAuth == PKCE {
		return newPKCEAuthenticator(client, clientID, authURL)
	} else if typeAuth == Device {
		return newDeviceAuthenticator(client, clientID, authURL)
	}
	return nil, fmt.Errorf("invalid type of authentication")
}
