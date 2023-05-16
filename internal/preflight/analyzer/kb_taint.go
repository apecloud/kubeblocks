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
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"helm.sh/helm/v3/pkg/cli/values"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const (
	NodesPath   = "cluster-resources/nodes.json"
	Tolerations = "tolerations"
	KubeBlocks  = "kubeblocks"
)

type AnalyzeTaintClassByKb struct {
	analyzer *preflightv1beta2.KBTaintAnalyze
	HelmOpts *values.Options
}

func (a *AnalyzeTaintClassByKb) Title() string {
	return util.TitleOrDefault(a.analyzer.AnalyzeMeta, "Kubeblocks Taints")
}

func (a *AnalyzeTaintClassByKb) GetAnalyzer() *preflightv1beta2.KBTaintAnalyze {
	return a.analyzer
}

func (a *AnalyzeTaintClassByKb) IsExcluded() (bool, error) {
	return util.IsExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeTaintClassByKb) Analyze(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) ([]*analyze.AnalyzeResult, error) {
	result, err := a.analyzeTaint(getFile, findFiles)
	if err != nil {
		return []*analyze.AnalyzeResult{result}, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*analyze.AnalyzeResult{result}, nil
}

func (a *AnalyzeTaintClassByKb) analyzeTaint(getFile GetCollectedFileContents, findFiles GetChildCollectedFileContents) (*analyze.AnalyzeResult, error) {
	nodesData, err := getFile(NodesPath)
	if err != nil {
		return newFailedResultWithMessage(a.Title(), fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}
	var nodes v1.NodeList
	if err = json.Unmarshal(nodesData, &nodes); err != nil {
		return newFailedResultWithMessage(a.Title(), fmt.Sprintf("get jsonfile failed, err:%v", err)), err
	}
	err = a.generateTolerations()
	if err != nil {
		return newFailedResultWithMessage(a.Title(), fmt.Sprintf("get tolerations failed, err:%v", err)), err
	}
	return a.doAnalyzeTaint(nodes)
}

func (a *AnalyzeTaintClassByKb) doAnalyzeTaint(nodes v1.NodeList) (*analyze.AnalyzeResult, error) {
	taintFailResult := []string{}
	for _, node := range nodes.Items {
		if node.Spec.Taints == nil || len(node.Spec.Taints) == 0 {
			return newAnalyzeResult(a.Title(), PassType, a.analyzer.Outcomes), nil
		}
	}

	if a.analyzer.TolerationsMap == nil || len(a.analyzer.TolerationsMap) == 0 {
		return newAnalyzeResult(a.Title(), FailType, a.analyzer.Outcomes), nil
	}

	for k, tolerations := range a.analyzer.TolerationsMap {
		count := 0
		for _, node := range nodes.Items {
			if isTolerableTaints(node.Spec.Taints, tolerations) {
				count++
			}
		}
		if count <= 0 {
			taintFailResult = append(taintFailResult, k)
		}
	}
	if len(taintFailResult) > 0 {
		result := newAnalyzeResult(a.Title(), FailType, a.analyzer.Outcomes)
		result.Message += fmt.Sprintf(" Taint check failed components: %s", strings.Join(taintFailResult, ", "))
		return result, nil
	}
	return newAnalyzeResult(a.Title(), PassType, a.analyzer.Outcomes), nil
}

func (a *AnalyzeTaintClassByKb) generateTolerations() error {
	tolerations := map[string][]v1.Toleration{}
	if a.HelmOpts != nil {
		optsMap, err := a.getHelmValues()
		if err != nil {
			return err
		}
		getTolerationsMap(optsMap, "", tolerations)
	}
	a.analyzer.TolerationsMap = tolerations
	return nil
}

func (a *AnalyzeTaintClassByKb) getHelmValues() (map[string]interface{}, error) {
	settings := cli.New()
	p := getter.All(settings)
	vals, err := a.HelmOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}
	return vals, nil
}

func getTolerationsMap(tolerationData map[string]interface{}, addonName string, tolerationsMap map[string][]v1.Toleration) {
	var tmpTolerationList []v1.Toleration
	var tmpToleration v1.Toleration

	for k, v := range tolerationData {
		if k == Tolerations {
			tolerationList := v.([]interface{})
			tmpTolerationList = []v1.Toleration{}
			for _, t := range tolerationList {
				toleration := t.(map[string]interface{})
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(toleration, &tmpToleration); err != nil {
					continue
				}
				tmpTolerationList = append(tmpTolerationList, tmpToleration)
			}
			if addonName == "" {
				addonName = KubeBlocks
			}
			tolerationsMap[addonName] = tmpTolerationList
			continue
		}

		switch v := v.(type) {
		case map[string]interface{}:
			if addonName != "" {
				addonName += "."
			}
			addonName += k
			getTolerationsMap(v, addonName, tolerationsMap)
		default:
			continue
		}
	}
}

func isTolerableTaints(taints []v1.Taint, tolerations []v1.Toleration) bool {
	tolerableCount := 0
	for _, taint := range taints {
		// check only on taints that have effect NoSchedule
		if taint.Effect != v1.TaintEffectNoSchedule {
			continue
		}
		for _, toleration := range tolerations {
			if toleration.ToleratesTaint(&taint) {
				tolerableCount++
				break
			}
		}
	}
	return tolerableCount >= len(taints)
}
