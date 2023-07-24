package organization

import (
	"encoding/json"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

const (
	APIURL  = "http://a8ff89cbeec444c82b90c5f83a117b39-16361bbd933bde33.elb.cn-northwest-1.amazonaws.com.cn:8086"
	APIPath = "api/v1"
)

type Organizations struct {
	Items []OrgItem `json:"items"`
}

type OrgItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type CurrentOrgAndContext struct {
	CurrentOrganization string `json:"currentOrganization"`
	CurrentContext      string `json:"currentContext"`
}

type Organization interface {
	getOrganization(token string, name string) (*OrgItem, error)
	GetOrganizations(token string) (*Organizations, error)
	switchOrganization(token string, name string) (string, error)
	addOrganization(token string, body []byte) error
	deleteOrganization(token string, name string) error
	IsValidOrganization(token string, name string) (bool, error)
}

type OrganizationOption struct {
	Name        string
	Description string
	DisplayName string

	Organization Organization

	genericclioptions.IOStreams
}

func NewOrganizationCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Organization command.",
	}
	cmd.AddCommand(
		newOrgListCmd(streams),
		newOrgSwitchCmd(streams),
		NewOrgCurrentCmd(streams),
		newOrgDescribeCmd(streams),
		newOrgAddCmd(streams),
		newOrgDeleteCmd(streams),
	)

	return cmd
}

func newOrgListCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &OrganizationOption{
		IOStreams: streams,
		Organization: &CloudOrganization{
			APIURL:  APIPath,
			APIPath: APIPath,
		},
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List organizations.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runList())
		},
	}

	return cmd
}

func newOrgSwitchCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &OrganizationOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runSwitch())
		},
	}

	return cmd
}

func NewOrgCurrentCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &OrganizationOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Current organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runCurrent())
		},
	}

	return cmd
}

func newOrgDescribeCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &OrganizationOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runDescribe())
		},
	}

	return cmd
}

func newOrgAddCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &OrganizationOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runAdd())
		},
	}
	cmd.Flags().StringVarP(&o.Description, "description", "n", "", "organization description")
	cmd.Flags().StringVarP(&o.DisplayName, "display-name", "d", "", "organization display name")

	return cmd
}

func newOrgDeleteCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &OrganizationOption{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runDelete())
		},
	}

	return cmd
}

func (o *OrganizationOption) validate() error {
	return nil
}

func (o *OrganizationOption) complete(args []string) error {
	if len(args) > 0 {
		o.Name = args[0]
	}
	return nil
}

// TODO: print organization list in a table format.
func (o *OrganizationOption) runList() error {
	token := GetToken()
	organizations, err := o.Organization.GetOrganizations(token)
	if err != nil {
		return err
	}

	tbl := printer.NewTablePrinter(o.Out)
	tbl.Tbl.SetColumnConfigs([]table.ColumnConfig{
		{Number: 5, WidthMax: 120},
	})
	tbl.SetHeader("NAME", "DISPLAYNAME", "DESCRIPTION", "ROLE", "ORGID")

	for _, item := range organizations.Items {
		tbl.AddRow(item.Name, item.DisplayName, item.Description, item.Role, item.ID)
	}

	tbl.Print()
	return nil
}

// TODO:check whether the newly set organization exists.
func (o *OrganizationOption) runSwitch() error {
	token := GetToken()
	oldOrganizationName, err := o.Organization.switchOrganization(token, o.Name)
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "From %s switched to organization: %s\n", oldOrganizationName, o.Name)
	return nil
}

func (o *OrganizationOption) runCurrent() error {
	currentOrgAndContext, err := GetCurrentOrgAndContext()
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "Current organization: %s\n", currentOrgAndContext.CurrentOrganization)
	return nil
}

// TODO: print organization in a table format.
func (o *OrganizationOption) runDescribe() error {
	token := GetToken()
	orgItem, err := o.Organization.getOrganization(token, o.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to get organization.")
	}

	tbl := printer.NewTablePrinter(o.Out)
	tbl.Tbl.SetColumnConfigs([]table.ColumnConfig{
		{Number: 5, WidthMax: 120},
	})
	tbl.SetHeader(
		"NAME",
		"DISPLAYNAME",
		"CREATEDAT",
		"UPDATEDAT",
		"DESCRIPTION",
		"ORGID",
		"ROLE",
	)

	tbl.AddRow(
		orgItem.Name,
		orgItem.DisplayName,
		orgItem.CreatedAt,
		orgItem.UpdatedAt,
		orgItem.Description,
		orgItem.ID,
		orgItem.Role,
	)

	tbl.Print()

	return nil
}

func (o *OrganizationOption) runAdd() error {
	token := GetToken()

	organization := OrganizationOption{
		Name:        o.Name,
		Description: o.Description,
		DisplayName: o.DisplayName,
	}
	body, err := json.Marshal(organization)
	if err != nil {
		return err
	}

	err = o.Organization.addOrganization(token, body)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Organization %s added.\n", o.Name)
	return nil
}

func (o *OrganizationOption) runDelete() error {
	token := GetToken()
	err := o.Organization.deleteOrganization(token, o.Name)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Organization %s deleted.\n", o.Name)
	return nil
}
