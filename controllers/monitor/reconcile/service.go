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
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func Service(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	desired := []*corev1.Service{}

	if reqCtx.GetOteldInstance(monitorv1alpha1.ModeDaemonSet) != nil {
		svc, err := buildSvcForOtel(reqCtx.Config, reqCtx.GetOteldInstance(monitorv1alpha1.ModeDaemonSet), reqCtx.Namespace)
		if err != nil {
			return err
		}
		if svc != nil {
			desired = append(desired, svc)
		}
	}

	if reqCtx.GetOteldInstance(monitorv1alpha1.ModeDeployment) != nil {
		svc, _ := buildSvcForOtel(reqCtx.Config, reqCtx.GetOteldInstance(monitorv1alpha1.ModeDeployment), reqCtx.Namespace)
		if svc != nil {
			desired = append(desired, svc)
		}
	}

	if err := expectedService(reqCtx, params, desired); err != nil {
		return err
	}

	if err := deleteService(reqCtx, params, desired); err != nil {
		return err
	}

	return nil
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

func deleteService(reqCtx types.ReconcileCtx, params types.OTeldParams, desired []*corev1.Service) error {
	listopts := []client.ListOption{
		client.InNamespace(reqCtx.Namespace),
		client.MatchingLabels(map[string]string{
			constant.AppManagedByLabelKey: constant.AppName,
			constant.AppNameLabelKey:      OTeldName,
		}),
	}

	serviceList := &corev1.ServiceList{}
	if params.Client.List(reqCtx.Ctx, serviceList, listopts...) != nil {
		return nil
	}

	for _, configMap := range serviceList.Items {
		isdel := true
		for _, keep := range desired {
			if keep.Name == configMap.Name && keep.Namespace == configMap.Namespace {
				isdel = false
				break
			}
		}

		if isdel {
			if err := params.Client.Delete(reqCtx.Ctx, &configMap); err != nil {
				return fmt.Errorf("failed to delete: %w", err)
			}
		}
	}
	return nil
}
