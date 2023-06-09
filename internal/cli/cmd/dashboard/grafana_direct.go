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

package dashboard

import (
	"fmt"
	"strings"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/spf13/cobra"
)

var (
	clusterType string
	// Todo: availableTypes is hard code, better to do a dynamic query but where the source from?
	availableTypes = [...]string{
		"mysql",
		"cadvisor",
		"jmx",
		"kafka",
		"mongodb",
		"node",
		"postgresql",
		"redis",
		"weaviate",
	}
	usage = `the cluster type opened directly in dashboard, support 'mysql','cadvisor','jmx','kafka','mongodb','node','postgresql','redis' and 'weaviate'`
)

func addCharacterFlag(cmd *cobra.Command, clusterType *string) {
	cmd.Flags().StringVar(clusterType, "cluster-type", "", usage)
	util.CheckErr(cmd.RegisterFlagCompletionFunc("cluster-type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var name []string
		for i := range availableTypes {
			if strings.HasPrefix(availableTypes[i], toComplete) {
				name = append(name, availableTypes[i])
			}
		}
		return name, cobra.ShellCompDirectiveNoFileComp
	}))
}

func buildGrafanaDirectURL(url *string, targetType string) error {
	if targetType == "" {
		return nil
	}
	for i := range availableTypes {
		if targetType == availableTypes[i] {
			*url += "/d/" + availableTypes[i]
			return nil
		}
	}
	return fmt.Errorf("input an invalid cluster type, please check your input")
}
