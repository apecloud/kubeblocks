package context

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

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

	genericclioptions.IOStreams
}

func NewContextCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Context command.",
	}
	cmd.AddCommand(
		NewContextListCmd(streams),
		NewContextUseCmd(streams),
		NewContextCurrentCmd(streams),
		NewContextDescribeCmd(streams),
	)
	return cmd
}

func NewContextListCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List context.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runList())
		},
	}
	return cmd
}

func NewContextCurrentCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "current",
		Short: "Current context.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runCurrent())
		},
	}
	return cmd
}

func NewContextDescribeCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe context.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runDescribe())
		},
	}

	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "human", "Output format (table|yaml|json)")

	return cmd
}

func NewContextUseCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &ContextOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "use",
		Short: "Use context.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runUse())
		},
	}
	return cmd
}

func (o *ContextOptions) validate(cmd *cobra.Command) error {
	if cmd.Name() != "list" && cmd.Name() != "current" {
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
			token := organization.GetToken()
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
