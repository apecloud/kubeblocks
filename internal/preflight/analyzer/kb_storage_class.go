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
	StorageClassPath = "cluster-resources/storage-classes.json"
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
		return newFailedResultWithMessage(a.Title(), fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}
	var storageClasses storagev1beta1.StorageClassList
	if err = json.Unmarshal(storageClassesData, &storageClasses); err != nil {
		return newFailedResultWithMessage(a.Title(), fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}

	for _, storageClass := range storageClasses.Items {
		if storageClass.Parameters["type"] != analyzer.StorageClassType {
			continue
		}
		if storageClass.Provisioner == "" || (storageClass.Provisioner == analyzer.Provisioner) {
			return newAnalyzeResult(a.Title(), PassType, a.analyzer.Outcomes), nil
		}
	}
	return newAnalyzeResult(a.Title(), FailType, a.analyzer.Outcomes), nil
}

var _ KBAnalyzer = &AnalyzeStorageClassByKb{}
