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
	IncorrectOutcomeType  = "there is an incorrect outcome type"
	PassType              = "Pass"
	WarnType              = "Warn"
	FailType              = "Fail"
)

type AnalyzeStorageClassByKb struct {
	analyzer *preflightv1beta2.KBStorageClassAnalyze
}

func (a *AnalyzeStorageClassByKb) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Kubeblocks Storage Class")
}

func (a *AnalyzeStorageClassByKb) GetAnalyzer() *preflightv1beta2.KBStorageClassAnalyze {
	return a.analyzer
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

func (a *AnalyzeStorageClassByKb) analyzeStorageClass(analyzer *preflightv1beta2.KBStorageClassAnalyze, getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) (*analyze.AnalyzeResult, error) {
	storageClassesData, err := getFile(StorageClassPath)
	if err != nil {
		return NewFailedResultWithMessage(a.Title(), fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}
	var storageClasses storagev1beta1.StorageClassList
	if err = json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		return NewFailedResultWithMessage(a.Title(), fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}

	for _, storageClass := range storageClasses.Items {
		if storageClass.Parameters["type"] != analyzer.StorageClassType {
			continue
		}
		if storageClass.Provisioner == "" || (storageClass.Provisioner == analyzer.Provisioner) {
			return NewAnalyzeResult(a.Title(), PassType, a.analyzer.Outcomes), nil
		}
	}
	return NewAnalyzeResult(a.Title(), FailType, a.analyzer.Outcomes), nil
}

var _ KBAnalyzer = &AnalyzeStorageClassByKb{}
