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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

type CloudContext struct {
	ContextName  string
	Token        string
	OrgName      string
	APIURL       string
	APIPath      string
	OutputFormat string

	genericiooptions.IOStreams
}

type Metadata struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Project      string `json:"project"`
	Organization string `json:"organization"`
	Partner      string `json:"partner"`
	ID           string `json:"id"`
	CreatedAt    struct {
		Seconds int `json:"seconds"`
		Nanos   int `json:"nanos"`
	} `json:"createdAt"`
	ModifiedAt struct {
		Seconds int `json:"seconds"`
		Nanos   int `json:"nanos"`
	} `json:"modifiedAt"`
}

type Params struct {
	KubernetesProvider   string `json:"kubernetesProvider"`
	ProvisionEnvironment string `json:"provisionEnvironment"`
	ProvisionType        string `json:"provisionType"`
	State                string `json:"state"`
}

type ClusterStatus struct {
	Conditions []struct {
		Type        string `json:"type"`
		LastUpdated struct {
			Seconds int `json:"seconds"`
			Nanos   int `json:"nanos"`
		} `json:"lastUpdated"`
		Reason string `json:"reason"`
	} `json:"conditions"`
	Token              string `json:"token"`
	PublishedBlueprint string `json:"publishedBlueprint"`
}

type ClusterData struct {
	ClusterBlueprint string `json:"cluster_blueprint"`
	Projects         []struct {
		ProjectID string `json:"projectID"`
		ClusterID string `json:"clusterID"`
	} `json:"projects"`
	ClusterStatus ClusterStatus `json:"cluster_status"`
}

type ClusterSpec struct {
	ClusterType      string      `json:"clusterType"`
	Metro            struct{}    `json:"metro"`
	OverrideSelector string      `json:"overrideSelector"`
	Params           Params      `json:"params"`
	ProxyConfig      struct{}    `json:"proxyConfig"`
	ClusterData      ClusterData `json:"clusterData"`
}

type ClusterItem struct {
	Metadata Metadata    `json:"metadata"`
	Spec     ClusterSpec `json:"spec"`
}

type CloudContextsResponse struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Count int `json:"count"`
		Limit int `json:"limit"`
	} `json:"metadata"`
	Items []ClusterItem `json:"items"`
}

type Status struct {
	ConditionStatus int `json:"conditionStatus"`
}

type CloudContextResponse struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   Metadata    `json:"metadata"`
	Spec       ClusterSpec `json:"spec"`
	Status     Status      `json:"status"`
}

func (c *CloudContext) showContext() error {
	cloudContext, err := c.GetContext()
	if err != nil {
		return errors.Wrapf(err, "Failed to get context %s", c.ContextName)
	}

	switch strings.ToLower(c.OutputFormat) {
	case "yaml":
		return c.printYAML(cloudContext)
	case "json":
		return c.printJSON(cloudContext)
	case "human":
		fallthrough
	default:
		return c.printTable(cloudContext)
	}
}

func (c *CloudContext) printYAML(ctxRes *CloudContextResponse) error {
	yamlData, err := yaml.Marshal(ctxRes)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.Out, "%s", string(yamlData))
	return nil
}

func (c *CloudContext) printJSON(ctxRes *CloudContextResponse) error {
	jsonData, err := json.MarshalIndent(ctxRes, "", "    ")
	if err != nil {
		return err
	}

	fmt.Fprintf(c.Out, "%s", string(jsonData))
	return nil
}

func (c *CloudContext) printTable(ctxRes *CloudContextResponse) error {
	tbl := printer.NewTablePrinter(c.Out)
	tbl.Tbl.SetColumnConfigs([]table.ColumnConfig{
		{Number: 8, WidthMax: 120},
	})
	tbl.SetHeader(
		"NAME",
		"Description",
		"Project",
		"Organization",
		"Partner",
		"ID",
		"CreatedAt",
		"ModifiedAt",
	)

	createAt := convertTimestampToHumanReadable(
		ctxRes.Metadata.CreatedAt.Seconds,
		ctxRes.Metadata.CreatedAt.Nanos,
	)
	modifiedAt := convertTimestampToHumanReadable(
		ctxRes.Metadata.ModifiedAt.Seconds,
		ctxRes.Metadata.ModifiedAt.Nanos,
	)
	tbl.AddRow(
		ctxRes.Metadata.Name,
		ctxRes.Metadata.Description,
		ctxRes.Metadata.Project,
		ctxRes.Metadata.Organization,
		ctxRes.Metadata.Partner,
		ctxRes.Metadata.ID,
		createAt,
		modifiedAt,
	)
	tbl.Print()

	return nil
}

func (c *CloudContext) showContexts() error {
	cloudContexts, err := c.GetContexts()
	if err != nil {
		return errors.Wrapf(err, "Failed to get contexts, please check your organization name")
	}

	tbl := printer.NewTablePrinter(c.Out)
	tbl.Tbl.SetColumnConfigs([]table.ColumnConfig{
		{Number: 8, WidthMax: 120},
	})
	tbl.SetHeader(
		"NAME",
		"Description",
		"Project",
		"Organization",
		"Partner",
		"ID",
		"CreatedAt",
		"ModifiedAt",
	)

	for _, orgItem := range cloudContexts.Items {
		createAt := convertTimestampToHumanReadable(
			orgItem.Metadata.CreatedAt.Seconds,
			orgItem.Metadata.CreatedAt.Nanos,
		)
		modifiedAt := convertTimestampToHumanReadable(
			orgItem.Metadata.ModifiedAt.Seconds,
			orgItem.Metadata.ModifiedAt.Nanos,
		)
		tbl.AddRow(
			orgItem.Metadata.Name,
			orgItem.Metadata.Description,
			orgItem.Metadata.Project,
			orgItem.Metadata.Organization,
			orgItem.Metadata.Partner,
			orgItem.Metadata.ID,
			createAt,
			modifiedAt,
		)
	}
	tbl.Print()

	if ok := writeContexts(cloudContexts); ok != nil {
		return errors.Wrapf(err, "Failed to write contexts.")
	}
	return nil
}

func (c *CloudContext) showCurrentContext() error {
	currentContext, err := c.getCurrentContext()
	if err != nil {
		return errors.Wrapf(err, "Failed to get current context.")
	}

	fmt.Fprintf(c.Out, "Current context: %s\n", currentContext)
	return nil
}

func (c *CloudContext) showUseContext() error {
	oldContextName, err := c.useContext(c.ContextName)
	if err != nil {
		return errors.Wrapf(err, "Failed to switch context to %s.", c.ContextName)
	}

	fmt.Fprintf(c.Out, "Successfully switched from %s to context %s.\n", oldContextName, c.ContextName)
	return nil
}

func (c *CloudContext) showRemoveContext() error {
	if err := c.removeContext(); err != nil {
		return errors.Wrapf(err, "Failed to remove context %s.", c.ContextName)
	}

	fmt.Fprintf(c.Out, "Context %s removed.\n", c.ContextName)
	return nil
}

func (c *CloudContext) GetContext() (*CloudContextResponse, error) {
	path := strings.Join([]string{c.APIURL, c.APIPath, "organizations", c.OrgName, "contexts", c.ContextName}, "/")
	response, err := organization.NewRequest(http.MethodGet, path, c.Token, nil)
	if err != nil {
		return nil, err
	}

	var context CloudContextResponse
	err = json.Unmarshal(response, &context)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal context %s.", c.ContextName)
	}

	return &context, nil
}

func (c *CloudContext) GetContexts() (*CloudContextsResponse, error) {
	path := strings.Join([]string{c.APIURL, c.APIPath, "organizations", c.OrgName, "contexts"}, "/")
	response, err := organization.NewRequest(http.MethodGet, path, c.Token, nil)
	if err != nil {
		return nil, err
	}

	var contexts CloudContextsResponse
	err = json.Unmarshal(response, &contexts)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal contexts.")
	}

	return &contexts, nil
}

func (c *CloudContext) getCurrentContext() (string, error) {
	currentOrgAndContext, err := organization.GetCurrentOrgAndContext()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get current context.")
	}

	if ok, err := c.isValidContext(currentOrgAndContext.CurrentContext); !ok {
		return "", err
	}

	return currentOrgAndContext.CurrentContext, nil
}

func (c *CloudContext) useContext(contextName string) (string, error) {
	if ok, err := c.isValidContext(contextName); !ok {
		return "", err
	}

	currentOrgAndContext, err := organization.GetCurrentOrgAndContext()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get current context.")
	}

	oldContextName := currentOrgAndContext.CurrentContext
	currentOrgAndContext.CurrentContext = contextName
	if err = organization.SetCurrentOrgAndContext(currentOrgAndContext); err != nil {
		return "", errors.Wrapf(err, "Failed to switch context to %s.", contextName)
	}

	return oldContextName, nil
}

// RemoveContext TODO: By the way, delete the context stored locally.
func (c *CloudContext) removeContext() error {
	path := strings.Join([]string{c.APIURL, c.APIPath, "organizations", c.OrgName, "contexts", c.ContextName}, "/")
	_, err := organization.NewRequest(http.MethodDelete, path, c.Token, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *CloudContext) isValidContext(contextName string) (bool, error) {
	cloudContexts, err := c.GetContexts()
	if err != nil {
		return false, errors.Wrap(err, "Failed to get contexts.")
	}

	if cloudContexts == nil || len(cloudContexts.Items) == 0 {
		return false, errors.Wrap(err, "No context found, please create a context on cloud.")
	}

	for _, item := range cloudContexts.Items {
		if item.Metadata.Name == contextName {
			return true, nil
		}
	}

	return false, errors.Errorf("Context %s does not exist.", contextName)
}

func writeContexts(contexts *CloudContextsResponse) error {
	jsonData, err := json.MarshalIndent(contexts, "", "    ")
	if err != nil {
		return err
	}

	filePath, err := organization.GetContextFilePath()
	if err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}
	return nil
}

func convertTimestampToHumanReadable(seconds int, nanos int) string {
	// Convert seconds and nanoseconds to a time.Time object
	secondsDuration := time.Duration(seconds) * time.Second
	nanosDuration := time.Duration(nanos) * time.Nanosecond
	timestamp := time.Unix(0, int64(secondsDuration+nanosDuration))

	// Format the time to a human-readable layout
	return timestamp.Format("2006-01-02 15:04:05")
}
