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

package alert

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listSMTPServerExample = templates.Examples(`
		# list alert smtp servers config
		kbcli alert list-smtpserver`)
)

type listSMTPServerOptions struct {
	baseOptions
}

func newListSMTPServerCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &listSMTPServerOptions{baseOptions: baseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:     "list-smtpserver",
		Short:   "List alert smtp servers config.",
		Example: listSMTPServerExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *listSMTPServerOptions) run() error {
	data, err := getConfigData(o.alertConfigMap, alertConfigFileName)
	if err != nil {
		return err
	}

	// get global
	global := getGlobalFromData(data)

	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("IDENTITY", "PASSWORD", "USERNAME", "FROM", "SMARTHOST")
	tbl.AddRow(global["smtp_auth_identity"], global["smtp_auth_password"], global["smtp_auth_username"], global["smtp_from"], global["smtp_smarthost"])
	tbl.Print()
	return nil
}
