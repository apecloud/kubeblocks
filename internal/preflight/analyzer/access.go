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

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const ClusterVersionPath = "cluster-info/cluster_version.json"

type AnalyzeClusterAccess struct {
	analyzer *preflightv1beta2.ClusterAccessAnalyze
}

func (a *AnalyzeClusterAccess) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Cluster Access")
}

func (a *AnalyzeClusterAccess) IsExcluded() (bool, error) {
	return util.IsExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterAccess) Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyze.AnalyzeResult, error) {
	isAccess := true
	collected, err := getFile(ClusterVersionPath)
	if err != nil {
		isAccess = false
	} else {
		if err := json.Unmarshal(collected, &collect.ClusterVersion{}); err != nil {
			isAccess = false
		}
	}

	result := analyze.AnalyzeResult{
		Title: a.Title(),
	}
	for _, outcome := range a.analyzer.Outcomes {
		if outcome.Fail != nil && !isAccess {
			result.IsFail = true
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		} else if outcome.Pass != nil && isAccess {
			result.IsPass = true
			result.Message = outcome.Pass.Message
			result.URI = outcome.Pass.URI
		}
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*analyze.AnalyzeResult{&result}, nil
}
