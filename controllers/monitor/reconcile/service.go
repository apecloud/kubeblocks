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

package reconcile

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitorv1alpha1 "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
)

const (
	OteldServiceName = "oteld-service"
)

func Service(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	desired := make([]*corev1.Service, 0)

	if daemon := reqCtx.OteldCfgRef.GetOteldInstance(monitorv1alpha1.ModeDaemonSet); daemon != nil {
		svc, err := buildSvcForOtel(daemon.Oteld, reqCtx.OTeld.Namespace, monitorv1alpha1.ModeDaemonSet)
		if err != nil {
			return err
		}
		if svc != nil {
			desired = append(desired, svc)
		}
	}
	if deploy := reqCtx.OteldCfgRef.GetOteldInstance(monitorv1alpha1.ModeDeployment); deploy != nil {
		svc, _ := buildSvcForOtel(deploy.Oteld, reqCtx.OTeld.Namespace, monitorv1alpha1.ModeDeployment)
		if svc != nil {
			desired = append(desired, svc)
		}
	}

	return expectedService(reqCtx, params, desired)
}

func expectedService(reqCtx types.ReconcileCtx, params types.OTeldParams, desired []*corev1.Service) error {
	for _, svc := range desired {
		desired := svc

		existing := &corev1.Service{}
		getErr := params.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
		if getErr != nil && apierrors.IsNotFound(getErr) {
			if createErr := params.Client.Create(reqCtx.Ctx, desired); createErr != nil {
				if apierrors.IsAlreadyExists(createErr) {
					return nil
				}
				return fmt.Errorf("failed to create: %w", createErr)
			}
			reqCtx.Log.V(2).Info("created", "configmap.name", desired.Name, "configmap.namespace", desired.Namespace)
			continue
		} else if getErr != nil {
			return getErr
		}

		updated := existing.DeepCopy()
		if updated.Annotations == nil {
			updated.Annotations = map[string]string{}
		}
		if updated.Labels == nil {
			updated.Labels = map[string]string{}
		}

		updated.Spec = desired.Spec
		updated.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences

		for k, v := range desired.ObjectMeta.Annotations {
			updated.ObjectMeta.Annotations[k] = v
		}
		for k, v := range desired.ObjectMeta.Labels {
			updated.ObjectMeta.Labels[k] = v
		}

		patch := client.MergeFrom(existing)

		if err := params.Client.Patch(reqCtx.Ctx, updated, patch); err != nil {
			return fmt.Errorf("failed to apply changes: %w", err)
		}

		reqCtx.Log.V(2).Info("applied", "configmap.name", desired.Name, "configmap.namespace", desired.Namespace)
	}
	return nil
}
