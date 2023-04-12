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

const StorageClassPath = "cluster-resources/storage-classes.json"

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
	result := analyze.AnalyzeResult{
		Title: a.Title(),
	}

	storageClassesData, err := getFile(StorageClassPath)
	if err != nil {
		result.IsFail = true
		result.Message = fmt.Sprintf("get jsonfile failed, err:%v", err)
		return &result, err
	}

	var storageClasses storagev1beta1.StorageClassList
	if err := json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		result.IsFail = true
		result.Message = fmt.Sprintf("get jsonfile failed, err:%v", err)
		return &result, err
	}

	for _, storageClass := range storageClasses.Items {
		if storageClass.Parameters["type"] != analyzer.StorageClassType {
			continue
		}
		if storageClass.Provisioner == "" || (storageClass.Provisioner == analyzer.Provisioner) {
			for _, outcome := range analyzer.Outcomes {
				if outcome.Pass != nil {
					result.IsPass = true
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
				}
			}
			return &result, nil
		}
	}

	result.IsFail = true
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		}
	}
	return &result, nil
}
