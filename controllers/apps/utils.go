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

package apps

import (
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/constant"
)

// default reconcile requeue after duration
var requeueDuration time.Duration = time.Millisecond * 100

func getEnvReplacementMapForAccount(name, passwd string) map[string]string {
	return map[string]string{
		"$(USERNAME)": name,
		"$(PASSWD)":   passwd,
	}
}

// parseCustomLabelPattern parses the custom label pattern to GroupVersionKind.
func parseCustomLabelPattern(pattern string) (schema.GroupVersionKind, error) {
	patterns := strings.Split(pattern, "/")
	switch len(patterns) {
	case 2:
		return schema.GroupVersionKind{
			Group:   "",
			Version: patterns[0],
			Kind:    patterns[1],
		}, nil
	case 3:
		return schema.GroupVersionKind{
			Group:   patterns[0],
			Version: patterns[1],
			Kind:    patterns[2],
		}, nil
	}
	return schema.GroupVersionKind{}, fmt.Errorf("invalid pattern %s", pattern)
}

// getCustomLabelSupportKind returns the kinds that support custom label.
func getCustomLabelSupportKind() []string {
	return []string{
		constant.CronJob,
		constant.StatefulSetKind,
		constant.DeploymentKind,
		constant.ReplicaSet,
		constant.ServiceKind,
		constant.ConfigMapKind,
		constant.PodKind,
	}
}

// getObjectListMapOfResourceKind returns the mapping of resource kind and its object list.
func getObjectListMapOfResourceKind() map[string]client.ObjectList {
	return map[string]client.ObjectList{
		constant.CronJob:         &batchv1.CronJobList{},
		constant.StatefulSetKind: &appsv1.StatefulSetList{},
		constant.DeploymentKind:  &appsv1.DeploymentList{},
		constant.ReplicaSet:      &appsv1.ReplicaSetList{},
		constant.ServiceKind:     &corev1.ServiceList{},
		constant.ConfigMapKind:   &corev1.ConfigMapList{},
		constant.PodKind:         &corev1.PodList{},
	}
}
