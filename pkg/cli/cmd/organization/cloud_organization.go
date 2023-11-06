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
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const apiName = "organization"

type CloudOrganization struct {
	Token   string
	APIURL  string
	APIPath string
}

func (o *CloudOrganization) getOrganization(name string) (*OrgItem, error) {
	path := strings.Join([]string{o.APIURL, o.APIPath, apiName, name}, "/")
	response, err := NewRequest(http.MethodGet, path, o.Token, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get organization.")
	}

	var orgItem OrgItem
	err = json.Unmarshal(response, &orgItem)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid organization format.")
	}

	return &orgItem, nil
}

func (o *CloudOrganization) GetOrganizations() (*Organizations, error) {
	path := strings.Join([]string{o.APIURL, o.APIPath, apiName}, "/")
	response, err := NewRequest(http.MethodGet, path, o.Token, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get organizations.")
	}

	var organizations Organizations
	err = json.Unmarshal(response, &organizations)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid organizations format.")
	}

	return &organizations, nil
}

func (o *CloudOrganization) switchOrganization(name string) (string, error) {
	if ok, err := o.IsValidOrganization(name); !ok {
		return "", err
	}

	currentOrgAndContext, err := GetCurrentOrgAndContext()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get current organization.")
	}
	oldOrganizationName := currentOrgAndContext.CurrentOrganization
	currentOrgAndContext.CurrentOrganization = name
	if err = SetCurrentOrgAndContext(currentOrgAndContext); err != nil {
		return "", errors.Wrapf(err, "Failed to switch organization to %s.", name)
	}
	return oldOrganizationName, nil
}

func (o *CloudOrganization) getCurrentOrganization() (string, error) {
	currentOrg, err := getCurrentOrganization()
	if err != nil {
		return "", err
	}

	if ok, err := o.IsValidOrganization(currentOrg); !ok {
		return "", err
	}
	return currentOrg, nil
}

func (o *CloudOrganization) IsValidOrganization(name string) (bool, error) {
	organizations, err := o.GetOrganizations()
	if err != nil {
		return false, errors.Wrap(err, "Failed to get organizations.")
	}
	for _, item := range organizations.Items {
		if item.Name == name {
			return true, nil
		}
	}
	return false, errors.Errorf("Organization %s not found.", name)
}

func (o *CloudOrganization) addOrganization(body []byte) error {
	path := strings.Join([]string{o.APIURL, o.APIPath, apiName}, "/")
	_, err := NewRequest(http.MethodPost, path, o.Token, body)
	if err != nil {
		return errors.Wrap(err, "Failed to add organization.")
	}

	return nil
}

func (o *CloudOrganization) deleteOrganization(name string) error {
	path := strings.Join([]string{o.APIURL, o.APIPath, apiName, name}, "/")
	_, err := NewRequest(http.MethodDelete, path, o.Token, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to delete organization.")
	}

	return nil
}

// SetCurrentOrgAndContext TODO:Check whether the newly set context and org exist.
func SetCurrentOrgAndContext(currentOrgAndContext *CurrentOrgAndContext) error {
	data, err := json.MarshalIndent(currentOrgAndContext, "", "    ")
	if err != nil {
		return errors.Wrap(err, "Failed to marshal current organization and context.")
	}

	filePath, err := GetCurrentOrgAndContextFilePath()
	if err != nil {
		return errors.Wrap(err, "Failed to get current organization and context.")
	}

	// Create the necessary folders and file if they don't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return errors.Wrap(err, "Failed to create necessary folders.")
	}

	file, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "Failed to create or open current organization and context file.")
	}

	_, err = file.Write(data)
	if err != nil {
		return errors.Wrap(err, "Failed to write current organization and context.")
	}
	defer file.Close()

	return nil
}

func getCurrentOrganization() (string, error) {
	currentOrgAndContext, err := GetCurrentOrgAndContext()
	if err != nil {
		return "", err
	}

	if currentOrgAndContext.CurrentOrganization == "" {
		return "", errors.New("No organization available, please join an organization.")
	}
	return currentOrgAndContext.CurrentOrganization, nil
}
