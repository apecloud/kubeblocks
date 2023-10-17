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

package configuration

import (
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type ConfigurationRevision struct {
	Revision    int64
	StrRevision string
	Phase       appsv1alpha1.ConfigurationPhase
}

const revisionHistoryLimit = 10

func GcConfigRevision(configObj *corev1.ConfigMap) {
	revisions := GcRevision(configObj.ObjectMeta.Annotations)
	if len(revisions) > 0 {
		for _, v := range revisions {
			delete(configObj.ObjectMeta.Annotations, core.GenerateRevisionPhaseKey(v.StrRevision))
		}
	}
}

func GcRevision(annotations map[string]string) []ConfigurationRevision {
	revisions := RetrieveRevision(annotations)
	if len(revisions) <= revisionHistoryLimit {
		return nil
	}

	return revisions[0 : len(revisions)-revisionHistoryLimit]
}

func RetrieveRevision(annotations map[string]string) []ConfigurationRevision {
	var revisions []ConfigurationRevision
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
		return revisions[i].Revision < revisions[j].Revision
	})
	return revisions
}

func parseRevision(revision string, phase string) (ConfigurationRevision, error) {
	v, err := strconv.ParseInt(revision, 10, 64)
	if err != nil {
		return ConfigurationRevision{}, err
	}
	return ConfigurationRevision{
		StrRevision: revision,
		Revision:    v,
		Phase:       appsv1alpha1.ConfigurationPhase(phase),
	}, nil
}

func GetCurrentRevision(annotations map[string]string) string {
	if len(annotations) == 0 {
		return ""
	}
	return annotations[constant.ConfigurationRevision]
}
