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
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
)

type TokenProvider struct {
	cached CachedTokenProvider
	issued IssuedTokenProvider
}

func NewTokenProvider(o Options) Provider {
	cached := NewKeyringCachedTokenProvider(nil)
	issued := NewCloudIssuedTokenProvider(o)

	return &TokenProvider{
		cached: cached,
		issued: issued,
	}
}

func (p *TokenProvider) Login(ctx context.Context) (*UserInfoResponse, error) {
	isAccessTokenValid := func(tokenResponse TokenResponse) bool { return IsValidToken(tokenResponse.AccessToken) }
	tokenResult, err := p.refreshFromCache(isAccessTokenValid)
	if err != nil {
		return nil, errors.Wrap(err, "could not refresh from cache")
	}

	var userInfo *UserInfoResponse
	if tokenResult != nil {
		userInfo, err = p.cached.GetUserInfo()
		if err != nil {
			return nil, errors.Wrap(err, "could not get user info from cache")
		}
	} else {
		tokenResult, err = p.issued.PKCEAuthenticate(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "could not authenticate with cloud")
		}
		userInfo, err = p.issued.GetUserInfoFromPKCE(tokenResult.AccessToken)
		if err != nil {
			return nil, errors.Wrap(err, "could not get user info from cloud")
		}
		err = p.cached.StoreUserInfo(userInfo)
		if err != nil {
			return nil, errors.Wrap(err, "could not store user info")
		}
	}

	err = p.cached.CacheTokens(tokenResult)
	if err != nil {
		return nil, errors.Wrap(err, "could not cache tokens")
	}

	return userInfo, nil
}

func (p *TokenProvider) Logout(ctx context.Context) error {
	tokenResult, err := p.cached.GetTokens()
	if err != nil {
		return err
	}
	if tokenResult == nil {
		return fmt.Errorf("token not found in cache, already logged out")
	}

	err = p.cached.DeleteTokens()
	if err != nil {
		return err
	}

	err = p.issued.LogoutForPKCE(ctx, tokenResult.IDToken)
	if err != nil {
		return err
	}
	return nil
}

func (p *TokenProvider) refreshFromCache(isTokenValid func(TokenResponse) bool) (*TokenResponse, error) {
	tokenResult, err := p.cached.GetTokens()
	if err != nil {
		return nil, errors.Wrap(err, "could get tokens from the cache")
	}
	// if the token is not in the cache, return nil
	if tokenResult == nil {
		return nil, nil
	}

	if isTokenValid(*tokenResult) {
		return tokenResult, nil
	}

	if tokenResult.RefreshToken == "" {
		return nil, nil
	}

	return p.getRefreshToken(tokenResult.RefreshToken), nil
}

// getRefreshToken gets a new token from the refresh token
func (p *TokenProvider) getRefreshToken(refreshToken string) *TokenResponse {
	tokenResult, err := p.issued.RefreshTokenFromPKCE(refreshToken)
	if err != nil {
		return nil
	}
	return tokenResult
}

// IsValidToken checks to see if the token is valid and has not expired
func IsValidToken(tokenString string) bool {
	jwtParser := jwt.Parser{}
	claims := jwt.MapClaims{}
	if _, _, err := jwtParser.ParseUnverified(tokenString, claims); err != nil {
		fmt.Println("Token parsing failed:", err)
		return false
	}
	return claims.VerifyExpiresAt(time.Now().Unix(), true)
}
