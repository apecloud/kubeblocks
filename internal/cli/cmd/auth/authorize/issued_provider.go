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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/authorize/authenticator"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/utils"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Options struct {
	ClientID  string `json:"client_id"`
	AuthURL   string
	NoBrowser bool
	genericiooptions.IOStreams
}

type CloudIssuedTokenProvider struct {
	Options
	Authenticator authenticator.Authenticator
}

func newDefaultIssuedTokenProvider(o Options) (*CloudIssuedTokenProvider, error) {
	authenticator, err := authenticator.NewAuthenticator(authenticator.PKCE, cleanhttp.DefaultClient(), o.ClientID, o.AuthURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create authenticator")
	}
	return &CloudIssuedTokenProvider{
		Options:       o,
		Authenticator: authenticator,
	}, nil
}

func newIssuedTokenProvider(o Options, authenticator authenticator.Authenticator) (*CloudIssuedTokenProvider, error) {
	return &CloudIssuedTokenProvider{
		Options:       o,
		Authenticator: authenticator,
	}, nil
}

func (c *CloudIssuedTokenProvider) authenticate(ctx context.Context) (*authenticator.TokenResponse, error) {
	authorizeResponse, err := c.Authenticator.GetAuthorization(ctx, c.openURLFunc)
	if err != nil {
		return nil, err
	}

	end := c.printProgress("Waiting for confirmation...")
	defer end()

	tokenResponse, err := c.Authenticator.GetToken(ctx, authorizeResponse)
	if err != nil {
		return nil, err
	}
	return tokenResponse, nil
}

func (c *CloudIssuedTokenProvider) getUserInfo(token string) (*authenticator.UserInfoResponse, error) {
	return c.Authenticator.GetUserInfo(context.TODO(), token)
}

func (c *CloudIssuedTokenProvider) refreshToken(refreshToken string) (*authenticator.TokenResponse, error) {
	tokenResponse, err := c.Authenticator.RefreshToken(context.TODO(), refreshToken)
	if err != nil {
		return nil, err
	}
	return tokenResponse, nil
}

func (c *CloudIssuedTokenProvider) logout(ctx context.Context, token string) error {
	end := c.printProgress("Logging out...")
	defer end()
	err := c.Authenticator.Logout(ctx, token, c.openURLFunc)
	if err != nil {
		return err
	}
	return nil
}

func (c *CloudIssuedTokenProvider) printProgress(message string) func() {
	if !utils.IsTTY() {
		fmt.Fprintln(c.Out, message)
		return func() {}
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(c.Out))
	s.Suffix = fmt.Sprintf(" %s", message)

	_ = s.Color("bold", "green")
	s.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Fprintf(c.Out, "\033[?25h")
		os.Exit(0)
	}()

	return func() {
		s.Stop()
		fmt.Fprintf(c.Out, "\033[?25h")
	}
}

func (c *CloudIssuedTokenProvider) openURLFunc(url string) {
	if !c.NoBrowser {
		err := util.OpenBrowser(url)
		if err != nil {
			msg := fmt.Sprintf("failed to open a browser: %s", printer.BoldRed(err.Error()))
			fmt.Fprint(c.Out, msg)
		}
	} else {
		msg := fmt.Sprintf("\nPlease paste this URL into your browser: %s\n\n", printer.BoldGreen(url))
		fmt.Fprint(c.Out, msg)
	}
}
