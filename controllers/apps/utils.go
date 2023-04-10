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
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// default reconcile requeue after duration
var requeueDuration = time.Millisecond * 100

func getEnvReplacementMapForAccount(name, passwd string) map[string]string {
	return map[string]string{
		"$(USERNAME)": name,
		"$(PASSWD)":   passwd,
	}
}

// notifyClusterStatusChange notifies cluster changes occurred and triggers it to reconcile.
func notifyClusterStatusChange(ctx context.Context, cli client.Client, recorder record.EventRecorder, obj client.Object, event *corev1.Event) error {
	if obj == nil || !intctrlutil.WorkloadFilterPredicate(obj) {
		return nil
	}

	cluster, ok := obj.(*appsv1alpha1.Cluster)
	if !ok {
		var err error
		if cluster, err = util.GetClusterByObject(ctx, cli, obj); err != nil {
			return err
		}
	}

	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	if err := cli.Patch(ctx, cluster, patch); err != nil {
		return err
	}

	if event != nil {
		recorder.Eventf(cluster, corev1.EventTypeWarning, event.Reason, getFinalEventMessageForRecorder(event))
	}
	return nil
}

// getFinalEventMessageForRecorder gets final event message by event involved object kind for recorded it
func getFinalEventMessageForRecorder(event *corev1.Event) string {
	if event.InvolvedObject.Kind == constant.PodKind {
		return fmt.Sprintf("Pod %s: %s", event.InvolvedObject.Name, event.Message)
	}
	return event.Message
}
