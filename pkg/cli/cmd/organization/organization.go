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

package organization

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/pkg/cli/printer"
)

var organizationExample = templates.Examples(`
	// Get the organization name currently used by the user.
	kbcli org current 
	// List all organizations the current user has joined.
	kbcli org list
	// Get the description information of organization org1.
	kbcli org describe org1
	// Switch to organization org2.
	kbcli org switch org2
`)

const (
	APIURL  = "https://cloudapi.apecloud.cn"
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
	getOrganization(name string) (*OrgItem, error)
	GetOrganizations() (*Organizations, error)
	switchOrganization(name string) (string, error)
	getCurrentOrganization() (string, error)
	addOrganization(body []byte) error
	deleteOrganization(name string) error
	IsValidOrganization(name string) (bool, error)
}

type OrganizationOption struct {
	Name         string
	OutputFormat string
	Organization Organization

	genericiooptions.IOStreams
}

func newOrganizationOption(streams genericiooptions.IOStreams) *OrganizationOption {
	return &OrganizationOption{
		IOStreams: streams,
	}
}

func NewOrganizationCmd(streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "org",
		Short:   "kbcli org is used to manage cloud organizations and is only suitable for interacting with cloud.",
		Example: organizationExample,
	}
	cmd.AddCommand(
		newOrgListCmd(streams),
		newOrgSwitchCmd(streams),
		newOrgCurrentCmd(streams),
		newOrgDescribeCmd(streams),
	)

	return cmd
}

func newOrgListCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := newOrganizationOption(streams)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all organizations you have joined.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runList())
		},
	}

	return cmd
}

func newOrgSwitchCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := newOrganizationOption(streams)

	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch to another organization you are already a member of.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runSwitch())
		},
	}

	return cmd
}

func newOrgCurrentCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := newOrganizationOption(streams)

	cmd := &cobra.Command{
		Use:   "current",
		Short: "Get current organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runCurrent())
		},
	}

	return cmd
}

func newOrgDescribeCmd(streams genericiooptions.IOStreams) *cobra.Command {
	o := newOrganizationOption(streams)

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Get the description information of an organization.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.validate(cmd))
			cmdutil.CheckErr(o.runDescribe())
		},
	}

	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "human", "Output format (human|yaml|json)")

	return cmd
}

func (o *OrganizationOption) validate(cmd *cobra.Command) error {
	if cmd.Name() == "switch" || cmd.Name() == "describe" {
		if o.Name == "" {
			return errors.New("Organization name is required.")
		}
	}
	return nil
}

func (o *OrganizationOption) complete(args []string) error {
	if len(args) > 0 {
		o.Name = args[0]
	}

	token, err := GetToken()
	if err != nil {
		return err
	}

	if o.Organization == nil {
		o.Organization = &CloudOrganization{
			Token:   token,
			APIURL:  APIURL,
			APIPath: APIPath,
		}
	}

	return nil
}

func (o *OrganizationOption) runList() error {
	organizations, err := o.Organization.GetOrganizations()
	if err != nil {
		return err
	}

	if len(organizations.Items) == 0 {
		fmt.Fprintln(o.Out, "you are currently not join in any organization")
		return nil
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

func (o *OrganizationOption) runSwitch() error {
	oldOrganizationName, err := o.Organization.switchOrganization(o.Name)
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "Successfully switched from %s to organization: %s\n", oldOrganizationName, o.Name)
	return nil
}

func (o *OrganizationOption) runCurrent() error {
	currentOrg, err := o.Organization.getCurrentOrganization()
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "Current organization: %s\n", currentOrg)
	return nil
}

func (o *OrganizationOption) runDescribe() error {
	orgItem, err := o.Organization.getOrganization(o.Name)
	if err != nil {
		return err
	}

	switch strings.ToLower(o.OutputFormat) {
	case "yaml":
		return o.printYAML(orgItem)
	case "json":
		return o.printJSON(orgItem)
	case "human":
		fallthrough
	default:
		return o.printTable(orgItem)
	}
}

func (o *OrganizationOption) printYAML(orgItem *OrgItem) error {
	body, err := yaml.Marshal(orgItem)
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "%s\n", body)
	return nil
}

func (o *OrganizationOption) printJSON(orgItem *OrgItem) error {
	body, err := json.MarshalIndent(orgItem, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "%s\n", body)
	return nil
}

func (o *OrganizationOption) printTable(orgItem *OrgItem) error {
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
