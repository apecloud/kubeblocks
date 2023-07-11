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

package authorize

import (
	"context"
)

type CachedTokenProvider interface {
	GetTokens() (*TokenResponse, error)
	cacheTokens(*TokenResponse) error
	deleteTokens() error
	cacheUserInfo(info *UserInfoResponse) error
	getUserInfo() (*UserInfoResponse, error)
}

type KeyringProvider interface {
	get() ([]byte, error)
	set(data []byte) error
	remove() error
	isValid() bool
}

type IssuedTokenProvider interface {
	DeviceAuthenticate() (*TokenResponse, error)
	PKCEAuthenticate(ctx context.Context) (*TokenResponse, error)
	refreshTokenFromPKCE(refreshToken string) (*TokenResponse, error)
	getUserInfoForDevice(token string) (*UserInfoResponse, error)
	getUserInfoFromPKCE(token string) (*UserInfoResponse, error)
	logoutForDevice(token string) error
	logoutForPKCE(ctx context.Context, token string) error
}

type Provider interface {
	Login(ctx context.Context) (*UserInfoResponse, error)
	Logout(ctx context.Context) error
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
