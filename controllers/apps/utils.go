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

package apps

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// default reconcile requeue after duration
var requeueDuration = time.Millisecond * 1000

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
		if cluster, err = GetClusterByObject(ctx, cli, obj); err != nil {
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

	if recorder != nil && event != nil {
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

func boolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func mergeMap(dst, src map[string]string) {
	for key, val := range src {
		dst[key] = val
	}
}
