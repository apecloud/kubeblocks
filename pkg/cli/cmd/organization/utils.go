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
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/auth/authorize"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

const (
	CloudContextDir          = "cloud_context"
	CurrentOrgAndContextFile = "current.json"
	ContextFile              = "context.json"
)

func GetCurrentOrgAndContextFilePath() (string, error) {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(cliHomeDir, CloudContextDir, CurrentOrgAndContextFile)
	return filePath, nil
}

func GetContextFilePath() (string, error) {
	cliHomeDir, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	filePath := filepath.Join(cliHomeDir, CloudContextDir, ContextFile)
	return filePath, nil
}

func GetToken() (string, error) {
	v := os.Getenv("TEST_ENV")
	if v == "true" {
		return "test_token", nil
	}

	cached := authorize.NewKeyringCachedTokenProvider(nil)
	tokenRes, err := cached.GetTokens()
	if err != nil {
		return "", err
	}
	if tokenRes != nil {
		return tokenRes.IDToken, nil
	}
	return "", errors.New("Failed to get token")
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
