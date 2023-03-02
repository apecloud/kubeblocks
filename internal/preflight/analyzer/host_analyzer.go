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
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

func GetHostAnalyzer(analyzer *preflightv1beta2.ExtendHostAnalyze) (analyze.HostAnalyzer, bool) {
	switch {
	case analyzer.HostUtility != nil:
		return &AnalyzeHostUtility{analyzer.HostUtility}, true
	default:
		return nil, false
	}
}
