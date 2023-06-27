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

package cluster

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type UpdateEngineOptions struct {
	// engine is the type of the engine to create.
	engine cluster.EngineType

	// values is used to render the cluster helm chart.
	values map[string]interface{}

	// schema is the cluster helm chart schema, used to render the command flag
	// and validate the values.
	schema *spec.Schema

	*patch.Options
}

func buildUpdateEngineCmds(patchOptions *patch.Options) []*cobra.Command {
	var (
		err  error
		cmds []*cobra.Command
	)

	for _, e := range cluster.SupportedEngines() {
		o := &UpdateEngineOptions{
			engine:  e,
			Options: patchOptions,
		}

		// get engine schema
		o.schema, err = cluster.GetEngineSchema(e)
		util.CheckErr(err)

		cmd := &cobra.Command{
			Use:   strings.ToLower(e.String()) + " NAME",
			Short: fmt.Sprintf("Update a %s cluster.", e),
			Run: func(cmd *cobra.Command, args []string) {
				cmdutil.CheckErr(o.complete(cmd, args))
				cmdutil.CheckErr(o.validate())
				cmdutil.CheckErr(o.run(cmd))
			},
		}

		util.CheckErr(addEngineFlags(cmd, o.Factory, o.schema))

		cmds = append(cmds, cmd)
	}
	return cmds
}

func (o *UpdateEngineOptions) complete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return makeMissingClusterNameErr()
	}
	if len(args) > 1 {
		return fmt.Errorf("only support to update one cluster")
	}
	o.Names = args

	// record the flags that been set by user
	fs := flag.NewFlagSet(cmd.Name(), flag.ContinueOnError)
	cmd.Flags().Visit(func(flag *flag.Flag) {
		fs.AddFlag(flag)
	})

	// not any flags been set, nothing to do
	if !fs.HasFlags() {
		return nil
	}

	// get values from flags
	o.values = getValuesFromFlags(fs)
	return nil
}

func (o *UpdateEngineOptions) validate() error {
	return cluster.ValidateValues(o.schema, o.values)
}

func (o *UpdateEngineOptions) run(cmd *cobra.Command) error {
	if err := o.buildPatch(); err != nil {
		return err
	}
	return o.Run(cmd)
}

func (o *UpdateEngineOptions) buildPatch() error {

}
