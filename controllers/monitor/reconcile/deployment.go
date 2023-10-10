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
	monitorv1alpha1 "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	monitortypes "github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OTeldAPIServerName = "grafana"

func Deployment(reqCtx monitortypes.ReconcileCtx, params monitortypes.OTeldParams) error {
	var (
		k8sClient = params.Client
		namespace = viper.GetString(constant.MonitorNamespaceEnvName)
	)

	instance := reqCtx.GetOteldInstance(monitorv1alpha1.ModeDeployment)

	oteldDeployment := buildDeploymentForOteld(reqCtx.Config, instance, namespace, OTeldName)

	existingDeployment := &appsv1.Deployment{}
	err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: OTeldName, Namespace: namespace}, existingDeployment)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "Failed to find daemonset", "daemonset", existingDeployment.Name)
			params.Recorder.Eventf(existingDeployment, corev1.EventTypeWarning, "Failed to find secret", err.Error())
			return err
		}
		if oteldDeployment == nil {
			return nil
		}
		return k8sClient.Create(reqCtx.Ctx, oteldDeployment)
	}

	if oteldDeployment == nil {
		return k8sClient.Delete(reqCtx.Ctx, existingDeployment)
	}

	if reflect.DeepEqual(existingDeployment.Spec, oteldDeployment.Spec) {
		return nil
	}

	updatedDeployment := existingDeployment.DeepCopy()
	updatedDeployment.Spec = oteldDeployment.Spec
	updatedDeployment.Labels = oteldDeployment.Labels
	updatedDeployment.Annotations = oteldDeployment.Annotations
	reqCtx.Log.Info("updating existing daemonset", "daemonset", client.ObjectKeyFromObject(updatedDeployment))
	return k8sClient.Update(reqCtx.Ctx, oteldDeployment)
}

func buildDeploymentForOteld(config *monitortypes.Config, instance *monitortypes.OteldInstance, namespace, name string) *appsv1.Deployment {
	if instance == nil || instance.OteldTemplate == nil {
		return nil
	}

	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  name,
		constant.MonitorManagedByKey:  "agamotto",
	}

	labelSelector := &metav1.LabelSelector{
		MatchLabels: commonLabels,
	}

	template := instance.OteldTemplate
	podSpec := buildPodSpecForOteld(config, template)

	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels)
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *podSpec,
	}

	return builder.NewDeploymentBuilder(namespace, name).
		SetTemplate(podTemplate).
		AddLabelsInMap(commonLabels).
		AddMatchLabelsInMap(commonLabels).
		SetSelector(labelSelector).
		SetOwnerReferences(template.APIVersion, template.Kind, template).
		GetObject()
}
