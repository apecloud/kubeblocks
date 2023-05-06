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
	"os"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/apimachinery/pkg/util/yaml"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"helm.sh/helm/v3/pkg/cli/values"
	v1 "k8s.io/api/core/v1"
	v1helper "k8s.io/component-helpers/scheduling/corev1"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

const (
	NodesPath   = "cluster-resources/nodes.json"
	Tolerations = "tolerations"
	KubeBlock   = "kubeblock"
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
	if a.analyzer.TolerationsMap == nil {
		return newAnalyzeResult(a.Title(), PassType, a.analyzer.Outcomes), nil
	}
	taintFailResult := []string{}
	for k, tolerations := range a.analyzer.TolerationsMap {
		count := 0
		for _, node := range nodes.Items {
			count += countTolerableTaints(node.Spec.Taints, tolerations)
		}
		if count <= 0 {
			taintFailResult = append(taintFailResult, k)
		}
	}
	if len(taintFailResult) > 0 {
		return newAnalyzeResult(a.Title(), FailType, a.analyzer.Outcomes), nil
	}
	return newAnalyzeResult(a.Title(), PassType, a.analyzer.Outcomes), nil
}

func (a *AnalyzeTaintClassByKb) generateTolerations() error {
	if a.HelmOpts == nil {
		return nil
	}
	optsMap, err := MergeValues(a.HelmOpts)
	if err != nil {
		return err
	}

	tolerations := map[string][]v1.Toleration{}
	getTolerationsMap(optsMap, "", tolerations)
	a.analyzer.TolerationsMap = tolerations
	return nil
}

func getTolerationsMap(tolerationData map[string]interface{}, addonName string, tolerationsMap map[string][]v1.Toleration) {
	var tmpTolerationList []v1.Toleration
	var tmpToleration v1.Toleration
	for k, v := range tolerationData {
		if addonName != "" {
			addonName += "."
		}
		addonName += k

		if k == Tolerations {
			tolerationList := v.([]interface{})
			tmpTolerationList = []v1.Toleration{}
			for _, t := range tolerationList {
				toleration := t.(map[string]interface{})
				tmpToleration.Key = toleration["key"].(string)
				tmpToleration.Value = toleration["value"].(string)
				tmpToleration.Operator = v1.TolerationOperator(toleration["operator"].(string))
				tmpToleration.Effect = v1.TaintEffect(toleration["effect"].(string))
				tmpTolerationList = append(tmpTolerationList, tmpToleration)
			}
			if addonName == "" {
				addonName = KubeBlock
			}
			tolerationsMap[addonName] = tmpTolerationList
			return
		}

		switch v := v.(type) {
		case map[string]interface{}:
			getTolerationsMap(v, addonName, tolerationsMap)
		default:
			return
		}
	}
}

// MergeValues merges values from files specified via -f/--values and directly
// via --set-json, --set, --set-string, or --set-file, marshaling them to YAML
func MergeValues(opts *values.Options) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range opts.ValueFiles {
		currentMap := map[string]interface{}{}

		bytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", filePath)
		}
		// Merge with the previous map
		base = mergeMaps(base, currentMap)
	}

	// User specified a value via --set-json
	for _, value := range opts.JSONValues {
		if err := strvals.ParseJSON(value, base); err != nil {
			return nil, errors.Errorf("failed parsing --set-json data %s", value)
		}
	}

	// User specified a value via --set
	for _, value := range opts.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set data")
		}
	}

	// User specified a value via --set-string
	for _, value := range opts.StringValues {
		if err := strvals.ParseIntoString(value, base); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set-string data")
		}
	}

	// User specified a value via --set-file
	for _, value := range opts.FileValues {
		reader := func(rs []rune) (interface{}, error) {
			bytes, err := os.ReadFile(string(rs))
			if err != nil {
				return nil, err
			}
			return string(bytes), err
		}
		if err := strvals.ParseIntoFile(value, base, reader); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set-file data")
		}
	}

	return base, nil
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func countTolerableTaints(taints []v1.Taint, tolerations []v1.Toleration) int {
	tolerableTaints := 0
	for _, taint := range taints {
		// check only on taints that have effect NoSchedule
		if taint.Effect != v1.TaintEffectNoSchedule {
			continue
		}

		if v1helper.TolerationsTolerateTaint(tolerations, &taint) {
			tolerableTaints++
		}
	}
	return tolerableTaints
}
