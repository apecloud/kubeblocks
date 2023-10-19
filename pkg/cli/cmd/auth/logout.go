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

package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/auth/authorize"
	"github.com/apecloud/kubeblocks/pkg/cli/cmd/auth/utils"
)

type LogOutOptions struct {
	authorize.Options
	Region string

	Provider authorize.Provider
}

func NewLogout(streams genericiooptions.IOStreams) *cobra.Command {
	o := &LogOutOptions{Options: authorize.Options{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of the KubeBlocks Cloud",
		Run: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(o.complete())
			cobra.CheckErr(o.validate())
			cobra.CheckErr(o.run(cmd))
		},
	}

	cmd.Flags().StringVarP(&o.Region, "region", "r", "jp", "Specify the region [jp] to log in.")
	return cmd
}

func (o *LogOutOptions) complete() error {
	o.AuthURL = getAuthURL(o.Region)

	var err error
	o.Provider, err = authorize.NewTokenProvider(o.Options)
	if err != nil {
		return err
	}
	if o.ClientID == "" {
		return o.loadConfig()
	}
	return nil
}

func (o *LogOutOptions) validate() error {
	if o.ClientID == "" {
		return fmt.Errorf("client ID is required")
	}
	return nil
}

func (o *LogOutOptions) run(cmd *cobra.Command) error {
	if utils.IsTTY() {
		fmt.Fprintln(o.Out, "Press Enter to log out of the KubeBlocks Cloud.")
		_ = waitForEnter(cmd.InOrStdin())
	}

	err := o.Provider.Logout(cmd.Context())
	if err != nil {
		return err
	}

	fmt.Fprintln(o.Out, "Successfully logged out.")
	return nil
}

func waitForEnter(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}

func (o *LogOutOptions) loadConfig() error {
	data, err := utils.Asset("config/config.enc")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &o.Options)
	if err != nil {
		return err
	}

	o.Provider, err = authorize.NewTokenProvider(o.Options)
	if err != nil {
		return err
	}
	return nil
}
