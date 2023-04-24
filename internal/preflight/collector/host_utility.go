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

package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	pkgcollector "github.com/replicatedhq/troubleshoot/pkg/collect"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const UtilityPathFormat = "host-collectors/utility/%s.json"
const DefaultHostUtilityName = "Host Utility"
const DefaultHostUtilityPath = "utility"

type HostUtilityInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Error string `json:"error"`
}

type CollectHostUtility struct {
	HostCollector *preflightv1beta2.HostUtility
	BundlePath    string
}

func (c *CollectHostUtility) Title() string {
	return util.TitleOrDefault(c.HostCollector.HostCollectorMeta, DefaultHostUtilityName)
}

func (c *CollectHostUtility) IsExcluded() (bool, error) {
	return util.IsExcluded(c.HostCollector.Exclude)
}

func (c *CollectHostUtility) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	hostCollector := c.HostCollector

	path, err := exec.LookPath(hostCollector.UtilityName)
	utilityInfo := HostUtilityInfo{
		Name: hostCollector.UtilityName,
		Path: path,
	}
	if err != nil {
		utilityInfo.Error = err.Error()
	}
	b, err := json.Marshal(utilityInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal host utility info")
	}

	resPath := util.TitleOrDefault(hostCollector.HostCollectorMeta, DefaultHostUtilityPath)

	output := pkgcollector.NewResult()
	_ = output.SaveResult(c.BundlePath, fmt.Sprintf(UtilityPathFormat, resPath), bytes.NewBuffer(b))
	return output, nil
}
