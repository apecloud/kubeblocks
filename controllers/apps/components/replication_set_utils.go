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

package components

import (
	"context"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// updateObjRoleChangedInfo updates the value of the role label and annotation of the object.
func updateObjRoleChangedInfo[T generics.Object, PT generics.PObject[T]](
	ctx context.Context, cli client.Client, event *corev1.Event, obj T, role string) error {
	pObj := PT(&obj)
	patch := client.MergeFrom(PT(pObj.DeepCopy()))
	pObj.GetLabels()[constant.RoleLabelKey] = role
	if pObj.GetAnnotations() == nil {
		pObj.SetAnnotations(map[string]string{})
	}
	pObj.GetAnnotations()[constant.LastRoleChangedEventTimestampAnnotationKey] = event.EventTime.Time.Format(time.RFC3339Nano)
	if err := cli.Patch(ctx, pObj, patch); err != nil {
		return err
	}
	return nil
}

// HandleReplicationSetRoleChangeEvent handles the role change event of the replication workload when switchPolicy is Noop.
func HandleReplicationSetRoleChangeEvent(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	event *corev1.Event,
	cluster *appsv1alpha1.Cluster,
	compName string,
	pod *corev1.Pod,
	newRole string) error {
	reqCtx.Log.Info("receive role change event", "podName", pod.Name, "current pod role label", pod.Labels[constant.RoleLabelKey], "new role", newRole)
	// if newRole is not Primary or Secondary, ignore it.
	if !slices.Contains([]string{constant.Primary, constant.Secondary}, newRole) {
		reqCtx.Log.Info("replicationSet new role is invalid, please check", "new role", newRole)
		return nil
	}

	// if switchPolicy is not Noop, return
	clusterCompSpec := getClusterComponentSpecByName(*cluster, compName)
	if clusterCompSpec == nil || clusterCompSpec.SwitchPolicy == nil || clusterCompSpec.SwitchPolicy.Type != appsv1alpha1.Noop {
		reqCtx.Log.Info("cluster switchPolicy is not Noop, does not support handling role change event", "cluster", cluster.Name)
		return nil
	}

	// update pod role label with newRole
	if err := updateObjRoleChangedInfo(reqCtx.Ctx, cli, event, *pod, newRole); err != nil {
		reqCtx.Log.Info("failed to update pod role label", "podName", pod.Name, "newRole", newRole, "err", err)
		return err
	}
	reqCtx.Log.Info("succeed to update pod role label", "podName", pod.Name, "newRole", newRole)
	return nil
}
