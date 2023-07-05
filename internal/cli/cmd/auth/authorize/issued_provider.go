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
	"runtime"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/hashicorp/go-cleanhttp"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/auth/utils"
)

type Options struct {
	ClientID  string `json:"client_id"`
	AuthURL   string
	NoBrowser bool
	genericclioptions.IOStreams
}

type CloudIssuedTokenProvider struct {
	Options
}

func NewCloudIssuedTokenProvider(o Options) *CloudIssuedTokenProvider {
	return &CloudIssuedTokenProvider{
		Options: o,
	}
}

func (c *CloudIssuedTokenProvider) DeviceAuthenticate() (*TokenResponse, error) {
	authenticator, err := NewAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return nil, err
	}

	deviceVerification, err := authenticator.VerifyDevice(context.TODO())
	if err != nil {
		return nil, err
	}

	bold := color.New(color.Bold)
	bold.Printf("\nConfirmation Code: ")
	boldGreen := bold.Add(color.FgGreen)
	boldGreen.Fprintln(color.Output, deviceVerification.UserCode)

	if !c.NoBrowser {
		openCmd := utils.OpenBrowser(runtime.GOOS, deviceVerification.VerificationCompleteURL)
		err = openCmd.Run()
		if err != nil {
			msg := fmt.Sprintf("failed to open a browser: %s", utils.BoldRed(err.Error()))
			fmt.Fprint(c.Out, msg)
		}
		msg := fmt.Sprintf("\nIf something goes wrong, copy and paste this URL into your browser: %s\n\n", utils.Bold(deviceVerification.VerificationCompleteURL))
		fmt.Fprint(c.Out, msg)
	} else {
		msg := fmt.Sprintf("\nPlease paste this URL into your browser: %s\n\n", utils.Bold(deviceVerification.VerificationCompleteURL))
		fmt.Fprint(c.Out, msg)
	}

	end := c.PrintProgress("Waiting for confirmation...")
	defer end()

	tokenResponse, err := authenticator.GetToken(context.TODO(), *deviceVerification)
	if err != nil {
		return nil, err
	}
	return tokenResponse, nil
}

func (c *CloudIssuedTokenProvider) PKCEAuthenticate(ctx context.Context) (*TokenResponse, error) {
	authenticator, err := NewPKCEAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return nil, err
	}

	authorizeResponse, err := authenticator.GetAuthorization(c.openURLFunc)
	if err != nil {
		return nil, err
	}

	end := c.PrintProgress("Waiting for confirmation...")
	defer end()

	tokenResponse, err := authenticator.GetToken(ctx, *authorizeResponse)
	if err != nil {
		return nil, err
	}
	return tokenResponse, nil
}

func (c *CloudIssuedTokenProvider) GetUserInfo(token string) (*UserInfoResponse, error) {
	authenticator, err := NewAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return nil, err
	}
	return authenticator.GetUserInfo(context.TODO(), token)
}

func (c *CloudIssuedTokenProvider) GetUserInfoFromPKCE(token string) (*UserInfoResponse, error) {
	authenticator, err := NewPKCEAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return nil, err
	}
	return authenticator.GetUserInfo(context.TODO(), token)
}

func (c *CloudIssuedTokenProvider) RefreshTokenFromPKCE(refreshToken string) (*TokenResponse, error) {
	authenticator, err := NewPKCEAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return nil, err
	}

	tokenResponse, err := authenticator.RefreshToken(context.TODO(), refreshToken)
	if err != nil {
		return nil, err
	}
	return tokenResponse, nil
}

func (c *CloudIssuedTokenProvider) Logout(token string) error {
	authenticator, err := NewAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return err
	}

	end := c.PrintProgress("Logging out...")
	defer end()
	err = authenticator.Logout(context.TODO(), token)
	if err != nil {
		return err
	}
	return nil
}

func (c *CloudIssuedTokenProvider) LogoutForPKCE(token string) error {
	authenticator, err := NewPKCEAuthenticator(cleanhttp.DefaultClient(), c.ClientID, c.AuthURL)
	if err != nil {
		return err
	}

	end := c.PrintProgress("Logging out...")
	defer end()
	err = authenticator.Logout(context.TODO(), token, c.openURLFunc)
	if err != nil {
		return err
	}
	return nil
}

func (c *CloudIssuedTokenProvider) PrintProgress(message string) func() {
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
		openCmd := utils.OpenBrowser(runtime.GOOS, url)
		err := openCmd.Run()
		if err != nil {
			msg := fmt.Sprintf("failed to open a browser: %s", utils.BoldRed(err.Error()))
			fmt.Fprint(c.Out, msg)
		}
		msg := fmt.Sprintf("\nIf something goes wrong, copy and paste this URL into your browser: %s\n\n", utils.Bold(url))
		fmt.Fprint(c.Out, msg)
	} else {
		msg := fmt.Sprintf("\nPlease paste this URL into your browser: %s\n\n", utils.Bold(url))
		fmt.Fprint(c.Out, msg)
	}
}
