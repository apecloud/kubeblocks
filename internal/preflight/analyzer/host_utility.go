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

package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"

	preflightv1beta2 "github.com/apecloud/kubeblocks/apis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

type AnalyzeHostUtility struct {
	hostAnalyzer *preflightv1beta2.HostUtilityAnalyze
}

func (a *AnalyzeHostUtility) Title() string {
	return util.AnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Host Utility Info")
}

func (a *AnalyzeHostUtility) IsExcluded() (bool, error) {
	return util.IsExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostUtility) collectorName() string {
	if a.hostAnalyzer.CollectorName != "" {
		return a.hostAnalyzer.CollectorName
	}
	return "utility"
}

func (a *AnalyzeHostUtility) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*analyzer.AnalyzeResult, error) {
	fullPath := fmt.Sprintf(kbcollector.UtilityPathFormat, a.collectorName())
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get collected file name: %s", fullPath)
	}

	utilityInfo := kbcollector.HostUtilityInfo{}
	if err := json.Unmarshal(collected, &utilityInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal utility info")
	}
	// generate analyzer result
	result := analyzer.AnalyzeResult{
		Title: a.Title(),
	}
	for _, outcome := range a.hostAnalyzer.Outcomes {
		if outcome.Fail != nil && len(utilityInfo.Error) > 0 && len(utilityInfo.Path) == 0 {
			result.IsFail = true
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		} else if outcome.Pass != nil && len(utilityInfo.Error) == 0 && len(utilityInfo.Path) > 0 {
			result.IsPass = true
			result.Message = outcome.Pass.Message + fmt.Sprintf(". Utility %s Path is %s", utilityInfo.Name, utilityInfo.Path)
			result.URI = outcome.Pass.URI
		}
		// ignore warn outcome
	}
	return []*analyzer.AnalyzeResult{&result}, nil
}
