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

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/monitor/types"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	monitor "github.com/apecloud/kubeblocks/controllers/monitor/config"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

const OTeldName = "apecloudoteld"

func OTeld(reqCtx types.ReconcileCtx, params types.OTeldParams) error {
	namespace := viper.GetString("OTELD_NAMESPACE")
	k8sClient := params.Client
	recorder := params.Recorder

	newSecret, err := buildSecretFromConfig(reqCtx, k8sClient)
	if err != nil {
		reqCtx.Log.Error(err, "Failed to build secret")
		recorder.Eventf(newSecret, corev1.EventTypeWarning, "Failed to build secret", err.Error())
		return err
	}
	exitingSecret := &corev1.Secret{}
	if err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: OTeldName, Namespace: namespace}, exitingSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "Failed to find secret", "secret", exitingSecret.Name)
			recorder.Eventf(exitingSecret, corev1.EventTypeWarning, "Failed to find secret", err.Error())
			return err
		}
		if err := params.Client.Create(reqCtx.Ctx, newSecret); err != nil {
			reqCtx.Log.Error(err, "Failed to create secret", "secret", newSecret.Name)
			return err
		}
	} else if !reflect.DeepEqual(exitingSecret.Data, newSecret.Data) || !reflect.DeepEqual(exitingSecret.StringData, newSecret.StringData) {
		exitingSecret.Data = newSecret.Data
		exitingSecret.StringData = newSecret.StringData
		exitingSecret.Labels = newSecret.Labels
		reqCtx.Log.Info("updating existing secret", "secret", newSecret.Name)
		if err := k8sClient.Update(reqCtx.Ctx, newSecret); err != nil {
			reqCtx.Log.Error(err, "Failed to update secret", "secret", newSecret.Name)
		}
	}

	// TODO: For easier debugging, please remove it after sufficient testing
	configBytes, _ := buildOteldConfigYamlBytes(reqCtx, namespace, k8sClient)
	configmap := builder.NewConfigMapBuilder(namespace, OTeldName).
		SetData(map[string]string{"config.yaml": string(configBytes)}).
		GetObject()
	if err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: OTeldName, Namespace: namespace}, configmap); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := k8sClient.Create(reqCtx.Ctx, configmap); err != nil {
			return err
		}
	} else {
		reqCtx.Log.Info("updating existing secret", "secret", exitingSecret.Name)
		if err := k8sClient.Update(reqCtx.Ctx, configmap); err != nil {
			reqCtx.Log.Error(err, "Failed to find secret", "secret", exitingSecret.Name)
		}
	}

	svc := buildSvcForOtel(namespace)
	exitingSvc := &corev1.Service{}
	if err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: OTeldName, Namespace: namespace}, exitingSvc); err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "Failed to find secret", "secret", exitingSecret.Name)
			recorder.Eventf(exitingSvc, corev1.EventTypeWarning, "Failed to find secret", err.Error())
			return err
		}
		if err := k8sClient.Create(reqCtx.Ctx, svc); err != nil {
			reqCtx.Log.Error(err, "Failed to create svc", "svc", svc.Name)
			recorder.Eventf(svc, corev1.EventTypeWarning, "Failed to create svc", err.Error())
			return err
		}
	} else {
		reqCtx.Log.Info("updating existing secret", "secret", exitingSecret.Name)
		if err := k8sClient.Update(reqCtx.Ctx, svc); err != nil {
			reqCtx.Log.Error(err, "Failed to find secret", "secret", exitingSecret.Name)
		}
	}

	return createOrUpdateOteld(reqCtx, recorder, namespace, k8sClient)
}

func createOrUpdateOteld(reqCtx types.ReconcileCtx, recorder record.EventRecorder, namespace string, k8sClient client.Client) error {

	oteldDaemonset := buildDaemonsetForOtel(reqCtx.Config, namespace)

	existingDaemonset := &appsv1.DaemonSet{}
	if err := k8sClient.Get(reqCtx.Ctx, client.ObjectKey{Name: OTeldName, Namespace: namespace}, existingDaemonset); err != nil {
		if !apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "Failed to find secret", "daemonset", existingDaemonset.Name)
			recorder.Eventf(existingDaemonset, corev1.EventTypeWarning, "Failed to find secret", err.Error())
			return err
		}
		if err := k8sClient.Create(reqCtx.Ctx, oteldDaemonset); err != nil {
			reqCtx.Log.Error(err, "Failed to create svc", "daemonset", oteldDaemonset.Name)
			recorder.Eventf(oteldDaemonset, corev1.EventTypeWarning, "Failed to create svc", err.Error())
			return err
		}
	} else {
		if reflect.DeepEqual(existingDaemonset.Spec, oteldDaemonset.Spec) {
			return nil
		}
		existingDaemonset.Spec = oteldDaemonset.Spec
		existingDaemonset.Labels = oteldDaemonset.Labels
		reqCtx.Log.Info("updating existing daemonset", "daemonset", oteldDaemonset.Name)
		if err := k8sClient.Update(reqCtx.Ctx, oteldDaemonset); err != nil {
			reqCtx.Log.Error(err, "Failed to update daemonset", "daemonset", oteldDaemonset.Name)
		}
	}
	return nil
}

func buildDaemonsetForOtel(config *types.Config, namespace string) *appsv1.DaemonSet {
	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  OTeldName,
		constant.MonitorManagedByKey:  "agamotto",
	}

	labelSelector := &v1.LabelSelector{
		MatchLabels: commonLabels,
	}

	podSpec := buildPodSpecForOteld(config)

	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels)

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *podSpec,
	}

	return builder.NewDaemonSetBuilder(namespace, OTeldName).
		SetTemplate(podTemplate).
		AddLabelsInMap(commonLabels).
		AddMatchLabelsInMap(commonLabels).
		SetSelector(labelSelector).
		GetObject()
}

func buildSecretFromConfig(ctx types.ReconcileCtx, k8sClient client.Client) (*corev1.Secret, error) {
	name := OTeldName
	namespace := ctx.Req.Namespace

	configBytes, err := buildOteldConfigYamlBytes(ctx, namespace, k8sClient)
	if err != nil {
		return nil, err
	}

	secret := builder.NewSecretBuilder(namespace, name).
		AddLabels(constant.AppNameLabelKey, name).
		AddLabels(constant.KBManagedByKey, constant.AppName).
		SetStringData(map[string]string{}).PutData("config.yaml", configBytes).
		GetObject()
	return secret, nil
}

func buildOteldConfigYamlBytes(ctx types.ReconcileCtx, namespace string, k8sClient client.Client) ([]byte, error) {
	config := ctx.Config

	metricsExporters := &v1alpha1.MetricsExporterSinkList{}
	if err := k8sClient.List(ctx.Ctx, metricsExporters, client.InNamespace(namespace)); err != nil {
		ctx.Log.Error(err, "Failed to find metrics metricsExporters", "secret")
		return nil, err
	}

	logsExporters := &v1alpha1.LogsExporterSinkList{}
	if err := k8sClient.List(ctx.Ctx, logsExporters, client.InNamespace(namespace)); err != nil {
		ctx.Log.Error(err, "Failed to find metrics logsExporters", "secret")
		return nil, err
	}

	datasources := &v1alpha1.CollectorDataSourceList{}
	if err := k8sClient.List(ctx.Ctx, datasources, client.InNamespace(namespace)); err != nil {
		ctx.Log.Error(err, "Failed to find datasources", "secret")
		return nil, err
	}

	gc, err := monitor.NewConfigGenerator(config)
	if err != nil {
		return nil, err
	}
	otelConfigBytes, err := gc.GenerateOteldConfiguration(datasources, metricsExporters, logsExporters)
	if err != nil {
		return nil, err
	}
	return otelConfigBytes, nil
}
