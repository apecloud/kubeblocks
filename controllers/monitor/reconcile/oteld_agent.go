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
	"reflect"

	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitorv1alpha1 "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

const OTeldAgentName = "oteld-agent"

func OTeldAgent(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	var (
		k8sClient = params.Client
		name      = reqCtx.OTeld.Name
		namespace = reqCtx.OTeld.Namespace
	)

	instance := reqCtx.OteldCfgRef.GetOteldInstance(monitorv1alpha1.ModeDaemonSet)
	if instance == nil {
		return cfgcore.MakeError("oteld instance is nil")
	}
	oteldDaemonSet := buildDaemonSetForOteld(instance, namespace, name)

	existingDaemonset := &appsv1.DaemonSet{}
	if err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: name, Namespace: namespace}, existingDaemonset); err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "Failed to find secret", "daemonset", existingDaemonset.Name)
			params.Recorder.Eventf(existingDaemonset, corev1.EventTypeWarning, "Failed to find secret", err.Error())
			return err
		}
		return k8sClient.Create(reqCtx.Ctx, oteldDaemonSet)
	}

	if reflect.DeepEqual(existingDaemonset.Spec, oteldDaemonSet.Spec) {
		return nil
	}

	updatedDeamonset := existingDaemonset.DeepCopy()
	updatedDeamonset.Spec = oteldDaemonSet.Spec
	updatedDeamonset.Labels = oteldDaemonSet.Labels
	updatedDeamonset.Annotations = oteldDaemonSet.Annotations
	reqCtx.Log.Info("updating existing daemonset", "daemonset", client.ObjectKeyFromObject(updatedDeamonset))
	return k8sClient.Update(reqCtx.Ctx, oteldDaemonSet)
}

func buildDaemonSetForOteld(instance *types.OteldInstance, namespace string, name string) *appsv1.DaemonSet {
	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  name,
		constant.MonitorManagedByKey:  "oteld",
	}

	labelSelector := &metav1.LabelSelector{
		MatchLabels: commonLabels,
	}

	template := instance.Oteld
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: commonLabels,
		},
		Spec: buildPodSpecForOteld(template),
	}

	return builder.NewDaemonSetBuilder(namespace, name).
		SetTemplate(podTemplate).
		AddLabelsInMap(commonLabels).
		AddMatchLabelsInMap(commonLabels).
		SetSelector(labelSelector).
		SetOwnerReferences(template.APIVersion, template.Kind, template).
		GetObject()
}
