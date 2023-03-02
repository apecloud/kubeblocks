/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
