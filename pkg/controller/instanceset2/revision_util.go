/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instanceset2

import (
	"fmt"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/util/dump"
	"k8s.io/apimachinery/pkg/util/rand"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type instanceRevisionIntent struct {
	Labels      map[string]string
	Annotations map[string]string
	Spec        workloads.InstanceSpec
}

func buildInstanceRevision(inst *workloads.Instance) string {
	return buildRevisionIntentHash(copyRevisionLabels(inst.Labels), copyRevisionAnnotations(inst.Annotations), *inst.Spec.DeepCopy())
}

func buildCurrentInstanceRevision(inst, desired *workloads.Instance) string {
	if desired == nil {
		return buildInstanceRevision(inst)
	}
	return buildRevisionIntentHash(
		copyRevisionLabelsByKeys(inst.Labels, desired.Labels),
		copyRevisionAnnotationsByKeys(inst.Annotations, desired.Annotations),
		*inst.Spec.DeepCopy())
}

func buildRevisionIntentHash(labels, annotations map[string]string, spec workloads.InstanceSpec) string {
	intent := instanceRevisionIntent{
		Labels:      labels,
		Annotations: annotations,
		Spec:        spec,
	}
	hasher := fnv.New32()
	fmt.Fprintf(hasher, "%v", dump.ForHash(intent))
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

func copyRevisionLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	return copied
}

func copyRevisionLabelsByKeys(labels, keys map[string]string) map[string]string {
	if len(labels) == 0 || len(keys) == 0 {
		return nil
	}
	copied := make(map[string]string, len(keys))
	for k := range keys {
		if v, ok := labels[k]; ok {
			copied[k] = v
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func copyRevisionAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}
	copied := make(map[string]string, len(annotations))
	for k, v := range annotations {
		if isNonRevisionAnnotation(k) {
			continue
		}
		copied[k] = v
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func copyRevisionAnnotationsByKeys(annotations, keys map[string]string) map[string]string {
	if len(annotations) == 0 || len(keys) == 0 {
		return nil
	}
	copied := make(map[string]string, len(keys))
	for k := range keys {
		if isNonRevisionAnnotation(k) {
			continue
		}
		if v, ok := annotations[k]; ok {
			copied[k] = v
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func isNonRevisionAnnotation(key string) bool {
	switch key {
	case constant.KubeBlocksGenerationKey:
		return true
	default:
		return false
	}
}
