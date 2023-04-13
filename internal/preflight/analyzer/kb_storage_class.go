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

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	storagev1beta1 "k8s.io/api/storage/v1beta1"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const (
	StorageClassPath      = "cluster-resources/storage-classes.json"
	MissingOutcomeMessage = "there is a missing outcome message"
	PassType              = "Pass"
	WarnType              = "Warn"
	FailType              = "Fail"
)

type AnalyzeStorageClassByKb struct {
	analyzer *preflightv1beta2.KbStorageClassAnalyze
}

func (a *AnalyzeStorageClassByKb) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Kubeblocks Storage Class")
}

func (a *AnalyzeStorageClassByKb) IsExcluded() (bool, error) {
	return util.IsExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeStorageClassByKb) Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyze.AnalyzeResult, error) {
	result, err := a.analyzeStorageClass(a.analyzer, getFile, findFiles)
	if err != nil {
		return []*analyze.AnalyzeResult{result}, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*analyze.AnalyzeResult{result}, nil
}

func (a *AnalyzeStorageClassByKb) analyzeStorageClass(analyzer *preflightv1beta2.KbStorageClassAnalyze, getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) (*analyze.AnalyzeResult, error) {
	storageClassesData, err := getFile(StorageClassPath)
	if err != nil {
		return a.FailedResultWithMessage(fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}
	var storageClasses storagev1beta1.StorageClassList
	if err = json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		return a.FailedResultWithMessage(fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}

	for _, storageClass := range storageClasses.Items {
		if storageClass.Parameters["type"] != analyzer.StorageClassType {
			continue
		}
		if storageClass.Provisioner == "" || (storageClass.Provisioner == analyzer.Provisioner) {
			return a.GenerateResult(PassType), nil
		}
	}
	return a.GenerateResult(FailType), nil
}

func (a *AnalyzeStorageClassByKb) GenerateResult(resultType string) *analyze.AnalyzeResult {
	result := analyze.AnalyzeResult{
		Title: a.Title(),
	}
	for _, outcome := range a.analyzer.Outcomes {
		if outcome == nil {
			continue
		}
		switch resultType {
		case PassType:
			if outcome.Pass != nil {
				result.IsPass = true
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI
				return &result
			}
		case WarnType:
			if outcome.Warn != nil {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI
				return &result
			}
		case FailType:
			if outcome.Fail != nil {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI
				return &result
			}
		default:
			break
		}
	}
	result.IsFail = true
	result.Message = MissingOutcomeMessage
	return &result
}

func (a *AnalyzeStorageClassByKb) FailedResultWithMessage(message string) *analyze.AnalyzeResult {
	return &analyze.AnalyzeResult{
		Title:   a.Title(),
		Message: message,
		IsFail:  true,
	}
}

var _ KBAnalyzer = &AnalyzeStorageClassByKb{}
