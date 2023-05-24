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

package sync2foxlake

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	AddressEndpointType     = "address"
	ClusterNameEndpointType = "clustername"
)

type EndpointModel struct {
	EndpointType          string
	EndpointCharacterType string
	Endpoint              string
	UserName              string
	Password              string
	Host                  string
	Port                  string
}

func (e *EndpointModel) complete(endpointName string) error {
	if endpointName == "" {
		return fmt.Errorf("endpointName is empty")
	}
	endpointName = strings.TrimSpace(endpointName)
	accountURLPair := strings.Split(endpointName, "@")
	switch len(accountURLPair) {
	case 1:
		e.EndpointType = ClusterNameEndpointType
		e.Endpoint = accountURLPair[0]
	case 2:
		e.EndpointType = AddressEndpointType
		accountPair := strings.Split(accountURLPair[0], ":")
		if len(accountPair) != 2 {
			return fmt.Errorf("the account info in endpoint is invalid, should be like \"user:123456\"")
		}
		e.UserName = accountPair[0]
		e.Password = accountPair[1]
		addressPair := strings.Split(accountURLPair[1], ":")
		if len(addressPair) != 2 {
			return fmt.Errorf("the address info in endpoint is invalid, should be like \"127.0.0.1:3306\"")
		}
		e.Endpoint = accountURLPair[1]
		e.Host = addressPair[0]
		e.Port = addressPair[1]
	default:
		return fmt.Errorf("the endpoint is invalid")
	}
	return nil
}

func NewSync2FoxLakeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync2foxlake",
		Short: "Sync data from other database to FoxLake",
	}

	groups := templates.CommandGroups{
		{
			Message: "Basic Sync2FoxLake Commands:",
			Commands: []*cobra.Command{
				NewSync2FoxLakeCreateCmd(f, streams),
				NewSync2FoxLakeListCmd(f, streams),
				NewSync2FoxLakeTerminateCmd(f, streams),
			},
		},
		{
			Message: "Sync2FoxLake Operation Commands:",
			Commands: []*cobra.Command{
				NewSync2FoxLakePauseCmd(f, streams),
				NewSync2FoxLakeResumeCmd(f, streams),
				NewSync2FoxLakeDescribeCmd(f, streams),
			},
		},
	}

	// add subcommands
	groups.Add(cmd)
	templates.ActsAsRootCommand(cmd, nil, groups...)

	return cmd
}
