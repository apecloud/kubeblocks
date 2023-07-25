package organization

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type CloudOrganization struct {
	APIURL  string
	APIPath string
}

func (o *CloudOrganization) getOrganization(token string, name string) (*OrgItem, error) {
	path := strings.Join([]string{o.APIURL, o.APIPath, "organizations", name}, "/")
	response, err := NewRequest(http.MethodGet, path, token, nil)
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

func (o *CloudOrganization) GetOrganizations(token string) (*Organizations, error) {
	path := strings.Join([]string{o.APIURL, o.APIPath, "organizations"}, "/")
	response, err := NewRequest(http.MethodGet, path, token, nil)
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

func (o *CloudOrganization) switchOrganization(token string, name string) (string, error) {
	if ok, err := o.IsValidOrganization(token, name); !ok {
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

func (o *CloudOrganization) IsValidOrganization(token string, name string) (bool, error) {
	organizations, err := o.GetOrganizations(token)
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

func (o *CloudOrganization) addOrganization(token string, body []byte) error {
	path := strings.Join([]string{o.APIURL, o.APIPath, "organizations"}, "/")
	_, err := NewRequest(http.MethodPost, path, token, body)
	if err != nil {
		return errors.Wrap(err, "Failed to add organization.")
	}

	return nil
}

func (o *CloudOrganization) deleteOrganization(token string, name string) error {
	path := strings.Join([]string{o.APIURL, o.APIPath, "organizations", name}, "/")
	_, err := NewRequest(http.MethodDelete, path, token, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to delete organization.")
	}

	return nil
}

func GetToken() string {
	filePath, err := GetTokenFilePath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	var token map[string]string
	err = json.Unmarshal(data, &token)
	if err != nil {
		return ""
	}

	return token["token"]
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

func GetCurrentOrgAndContext() (*CurrentOrgAndContext, error) {
	filePath, err := GetCurrentOrgAndContextFilePath()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get current organization and context file: %s", filePath)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read current organization and context file: %s", filePath)
	}

	var currentOrgAndContext CurrentOrgAndContext
	err = json.Unmarshal(data, &currentOrgAndContext)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid current organization and context format.")
	}

	return &currentOrgAndContext, nil
}
