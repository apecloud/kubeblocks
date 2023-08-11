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

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/authorize/authenticator"
)

type CachedTokenProvider interface {
	GetTokens() (*authenticator.TokenResponse, error)
	cacheTokens(*authenticator.TokenResponse) error
	deleteTokens() error
	cacheUserInfo(info *authenticator.UserInfoResponse) error
	getUserInfo() (*authenticator.UserInfoResponse, error)
}

type KeyringProvider interface {
	get() ([]byte, error)
	set(data []byte) error
	remove() error
	isValid() bool
}

type IssuedTokenProvider interface {
	authenticate(ctx context.Context) (*authenticator.TokenResponse, error)
	refreshToken(refreshToken string) (*authenticator.TokenResponse, error)
	getUserInfo(token string) (*authenticator.UserInfoResponse, error)
	logout(ctx context.Context, token string) error
}

type Provider interface {
	Login(ctx context.Context) (*authenticator.UserInfoResponse, string, error)
	Logout(ctx context.Context) error
}
