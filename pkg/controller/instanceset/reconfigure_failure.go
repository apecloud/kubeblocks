/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instanceset

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type reconfigureActionFailureRecord struct {
	ConfigHash string `json:"configHash,omitempty"`
	Message    string `json:"message,omitempty"`
}

func setReconfigureActionFailure(pod *corev1.Pod, config workloads.ConfigTemplate, message string) error {
	failures := getReconfigureActionFailures(pod)
	if failures == nil {
		failures = map[string]reconfigureActionFailureRecord{}
	}
	failures[config.Name] = reconfigureActionFailureRecord{
		ConfigHash: ptr.Deref(config.ConfigHash, ""),
		Message:    message,
	}
	return setReconfigureActionFailures(pod, failures)
}

func clearReconfigureActionFailure(pod *corev1.Pod, config workloads.ConfigTemplate) error {
	failures := getReconfigureActionFailures(pod)
	if len(failures) == 0 {
		return nil
	}
	delete(failures, config.Name)
	return setReconfigureActionFailures(pod, failures)
}

func getReconfigureActionFailures(pod *corev1.Pod) map[string]reconfigureActionFailureRecord {
	annotations := pod.GetAnnotations()
	if len(annotations) == 0 || annotations[constant.ReconfigureActionFailureAnnotationKey] == "" {
		return nil
	}
	failures := map[string]reconfigureActionFailureRecord{}
	if err := json.Unmarshal([]byte(annotations[constant.ReconfigureActionFailureAnnotationKey]), &failures); err != nil {
		return nil
	}
	return failures
}

func setReconfigureActionFailures(pod *corev1.Pod, failures map[string]reconfigureActionFailureRecord) error {
	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if len(failures) == 0 {
		delete(annotations, constant.ReconfigureActionFailureAnnotationKey)
		pod.SetAnnotations(annotations)
		return nil
	}
	data, err := json.Marshal(failures)
	if err != nil {
		return err
	}
	annotations[constant.ReconfigureActionFailureAnnotationKey] = string(data)
	pod.SetAnnotations(annotations)
	return nil
}
