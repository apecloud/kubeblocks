package login

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/context"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

type LoginOptions struct {
	OrgName     string
	ContextName string

	genericclioptions.IOStreams
}

func NewLoginCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &LoginOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login command.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVarP(&o.OrgName, "org", "o", "", "Organization name.")
	cmd.Flags().StringVarP(&o.ContextName, "context", "c", "", "Context name.")
	return cmd
}

func (o *LoginOptions) run() error {
	if o.OrgName != "" {
		if o.ContextName != "" {
			return o.loginWithOrgAndContext()
		}
		return o.loginWithOrg()
	}

	if o.ContextName != "" {
		return o.loginWithContext()
	}

	return o.loginWithDefault()
}

func (o *LoginOptions) loginWithOrg() error {
	token, err := o.login()
	if err != nil {
		return err
	}

	org := &organization.OrganizationOption{}
	if ok, err := org.Organization.IsValidOrganization(token, o.OrgName); !ok {
		return err
	}

	firstContextName, err := getFirstContext(token, o.OrgName)
	if err != nil {
		return err
	}

	return o.setCurrentConfig(o.OrgName, firstContextName)
}

func (o *LoginOptions) loginWithContext() error {
	token, err := o.login()
	if err != nil {
		return err
	}

	firstOrgName, err := getFirstOrg(token)
	if err != nil {
		return err
	}

	return o.setCurrentConfig(firstOrgName, o.ContextName)
}

func (o *LoginOptions) loginWithOrgAndContext() error {
	_, err := o.login()
	if err != nil {
		return err
	}
	return o.setCurrentConfig(o.OrgName, o.ContextName)
}

func (o *LoginOptions) loginWithDefault() error {
	token, err := o.login()
	if err != nil {
		return err
	}

	firstOrgName, err := getFirstOrg(token)
	if err != nil {
		return err
	}

	firstContextName, err := getFirstContext(token, firstOrgName)
	if err != nil {
		return err
	}

	return o.setCurrentConfig(firstOrgName, firstContextName)
}

func (o *LoginOptions) setCurrentConfig(orgName, contextName string) error {
	currentOrgAndContext := organization.CurrentOrgAndContext{
		CurrentOrganization: orgName,
		CurrentContext:      contextName,
	}

	err := organization.SetCurrentOrgAndContext(&currentOrgAndContext)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "You are now logged in to the organization: %s and the context: %s.\n", orgName, contextName)
	return nil
}

func (o *LoginOptions) login() (string, error) {
	return organization.GetToken(), nil
}

func getFirstOrg(token string) (string, error) {
	org := &organization.OrganizationOption{}
	organizations, err := org.Organization.GetOrganizations(token)
	if err != nil {
		return "", err
	}

	if organizations != nil {
		return organizations.Items[0].Name, nil
	}
	return "", errors.New("no organization")
}

func getFirstContext(token string, orgName string) (string, error) {
	c := &context.CloudContext{
		OrgName: orgName,
		Token:   token,
	}
	contexts, err := c.GetContexts()
	if err != nil {
		return "", err
	}

	if contexts != nil {
		return contexts.Items[0].Metadata.Name, nil
	}
	return "", errors.New("no context")
}
