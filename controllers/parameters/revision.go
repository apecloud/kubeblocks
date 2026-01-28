/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

const (
	revisionHistoryLimit = 10
)

type configurationRevision struct {
	revision    int64
	strRevision string
	phase       parametersv1alpha1.ParameterPhase
	result      parameters.Result
}

func gcConfigRevision(configObj *corev1.ConfigMap) {
	revisions := gcRevision(configObj.Annotations)
	if len(revisions) > 0 {
		for _, v := range revisions {
			delete(configObj.Annotations, core.GenerateRevisionPhaseKey(v.strRevision))
		}
	}
}

func gcRevision(annotations map[string]string) []configurationRevision {
	revisions := retrieveRevision(annotations)
	if len(revisions) <= revisionHistoryLimit {
		return nil
	}

	return revisions[0 : len(revisions)-revisionHistoryLimit]
}

func retrieveRevision(annotations map[string]string) []configurationRevision {
	var revisions []configurationRevision
	var revisionPrefix = constant.LastConfigurationRevisionPhase + "-"

	for key, value := range annotations {
		if !strings.HasPrefix(key, revisionPrefix) {
			continue
		}
		revision, err := parseRevision(strings.TrimPrefix(key, revisionPrefix), value)
		if err == nil {
			revisions = append(revisions, revision)
		}
	}

	// for sort
	sort.SliceStable(revisions, func(i, j int) bool {
		return revisions[i].revision < revisions[j].revision
	})
	return revisions
}

func parseRevision(revision string, data string) (configurationRevision, error) {
	v, err := strconv.ParseInt(revision, 10, 64)
	if err != nil {
		return configurationRevision{}, err
	}
	result := parseResult(data, revision)
	return configurationRevision{
		revision:    v,
		strRevision: revision,
		phase:       result.Phase,
		result:      result,
	}, nil
}

func parseResult(data string, revision string) parameters.Result {
	result := parameters.Result{
		Revision: revision,
	}
	data = strings.TrimSpace(data)
	if data == "" {
		return result
	}
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		result.Phase = parametersv1alpha1.ParameterPhase(data)
	}
	return result
}
