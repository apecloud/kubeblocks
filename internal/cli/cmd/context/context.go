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

package context

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

var contextExample = templates.Examples(`
	// Get the context name currently used by the user.
	kbcli context current 
	// List all contexts created by the current user.
	kbcli context list
	// Get the description information of context context1.
	kbcli context describe context1
	// Switch to context context2.
	kbcli context use context2
`)

const (
	localContext = "local"
)

type Context interface {
	showContext() error
	showContexts() error
	showCurrentContext() error
	showUseContext() error
	showRemoveContext() error
}

type ContextOptions struct {
	ContextName  string
	Context      Context
	OutputFormat string

	genericiooptions.IOStreams
}

func NewContextCmd(streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use: "context",
		Short: "kbcli context allows you to manage cloud context. This command is currently only applicable to cloud," +
			" and currently does not support switching the context of the local k8s cluster.",
		Example: contextExample,
	}
	cmd.AddCommand(
		newContextListCmd(streams),
		newContextUseCmd(streams),
		newContextCurrentCmd(streams),
		newContextDescribeCmd(streams),
	)
	return cmd
}

func newContextListCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all created contexts.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runList())
		},
	}
	return cmd
}

func newContextCurrentCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "current",
		Short: "Get the currently used context.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runCurrent())
		},
	}
	return cmd
}

func newContextDescribeCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Get the description information of a context.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runDescribe())
		},
	}

	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "human", "Output format (table|yaml|json)")

	return cmd
}

func newContextUseCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "use",
		Short: "Use another context that you have already created.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runUse())
		},
	}

	return cmd
}

func (o *ContextOptions) validate(cmd *cobra.Command) error {
	if cmd.Name() == "describe" || cmd.Name() == "use" {
		if o.ContextName == "" {
			return errors.New("context name is required")
		}
	}

	return nil
}

func (o *ContextOptions) complete(args []string) error {
	if len(args) > 0 {
		o.ContextName = args[0]
	}

	currentOrgAndContext, err := organization.GetCurrentOrgAndContext()
	if err != nil {
		return err
	}

	if o.Context == nil {
		if currentOrgAndContext.CurrentContext != localContext {
			token, err := organization.GetToken()
			if err != nil {
				return err
			}
			o.Context = &CloudContext{
				ContextName:  o.ContextName,
				Token:        token,
				OrgName:      currentOrgAndContext.CurrentOrganization,
				IOStreams:    o.IOStreams,
				APIURL:       organization.APIURL,
				APIPath:      organization.APIPath,
				OutputFormat: o.OutputFormat,
			}
		}
	}

	return nil
}

func (o *ContextOptions) runList() error {
	return o.Context.showContexts()
}

func (o *ContextOptions) runCurrent() error {
	return o.Context.showCurrentContext()
}

func (o *ContextOptions) runDescribe() error {
	return o.Context.showContext()
}

func (o *ContextOptions) runUse() error {
	return o.Context.showUseContext()
}
