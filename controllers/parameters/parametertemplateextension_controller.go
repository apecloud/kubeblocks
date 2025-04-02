/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

// ParameterTemplateExtensionReconciler reconciles a ParameterTemplateExtension object
type ParameterTemplateExtensionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ParameterTemplateExtension object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ParameterTemplateExtensionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ParameterExtensionReconciler").
			WithValues("Namespace", req.Namespace, "ParameterExtension", req.Name),
	}

	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if cluster.IsDeleting() {
		return intctrlutil.Reconciled()
	}
	if !intctrlutil.ObjectAPIVersionSupported(cluster) {
		return intctrlutil.Reconciled()
	}
	return r.reconcile(reqCtx, cluster)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ParameterTemplateExtensionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Cluster{}).
		Watches(&parametersv1alpha1.ComponentParameter{}, handler.EnqueueRequestsFromMapFunc(filterComponentParameterResources)).
		Complete(r)
}

func filterComponentParameterResources(_ context.Context, object client.Object) []reconcile.Request {
	cr := object.(*parametersv1alpha1.ComponentParameter)
	return []reconcile.Request{{
		NamespacedName: client.ObjectKey{
			Name:      cr.Spec.ClusterName,
			Namespace: cr.Namespace,
		}}}
}

func (r *ParameterTemplateExtensionReconciler) reconcile(reqCtx intctrlutil.RequestCtx, runningCluster *appsv1.Cluster) (ctrl.Result, error) {
	expectedCluster, err := updateConfigsForParameterTemplate(reqCtx, r.Client, runningCluster)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	return r.update(reqCtx, runningCluster, expectedCluster)
}

func updateConfigsForComponent(reqCtx intctrlutil.RequestCtx, reader client.Client, cluster *appsv1.Cluster, compSpec *appsv1.ClusterComponentSpec) error {
	resolveParameterCR := func(compName string) (*parametersv1alpha1.ComponentParameter, error) {
		parameterKey := client.ObjectKey{
			Name:      configcore.GenerateComponentConfigurationName(cluster.Name, compName),
			Namespace: cluster.Namespace,
		}
		parameterCr := &parametersv1alpha1.ComponentParameter{}
		if err := reader.Get(reqCtx.Ctx, parameterKey, parameterCr); err != nil {
			return nil, err
		}
		return parameterCr, nil
	}
	updateConfigObject := func(compName string, config *appsv1.ClusterComponentConfig) error {
		cmKey := client.ObjectKey{
			Name:      configcore.GetComponentCfgName(cluster.Name, compName, pointer.StringDeref(config.Name, "")),
			Namespace: cluster.Namespace,
		}
		cm := corev1.ConfigMap{}
		if err := reader.Get(reqCtx.Ctx, cmKey, &cm); err != nil {
			return client.IgnoreNotFound(err)
		}
		config.ConfigMap = &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
		}
		return nil
	}
	checkAndUpdateConfigObject := func(compName string, config *appsv1.ClusterComponentConfig) error {
		parameterCR, err := resolveParameterCR(compName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}
		if intctrlutil.GetConfigTemplateItem(&parameterCR.Spec, pointer.StringDeref(config.Name, "")) == nil {
			reqCtx.Log.Info("config template does not support parameters extension", "component", compName, "config", config.Name)
			return nil
		}
		return updateConfigObject(compName, config)
	}

	for j, config := range compSpec.Configs {
		if !needUpdateConfigObject(config) {
			continue
		}
		if err := checkAndUpdateConfigObject(compSpec.Name, &compSpec.Configs[j]); err != nil {
			return err
		}
	}
	return nil
}

func updateConfigsForParameterTemplate(reqCtx intctrlutil.RequestCtx, reader client.Client, cluster *appsv1.Cluster) (*appsv1.Cluster, error) {
	expectedCluster := cluster.DeepCopy()
	if err := handleComponent(reqCtx, reader, expectedCluster); err != nil {
		return nil, err
	}
	if err := handleShardingComponent(reqCtx, reader, expectedCluster); err != nil {
		return nil, err
	}
	return expectedCluster, nil
}

func handleComponent(reqCtx intctrlutil.RequestCtx, reader client.Client, cluster *appsv1.Cluster) error {
	for i := range cluster.Spec.ComponentSpecs {
		compSpec := &cluster.Spec.ComponentSpecs[i]
		if err := updateConfigsForComponent(reqCtx, reader, cluster, compSpec); err != nil {
			return err
		}
	}
	return nil
}

func needUpdateConfigObject(config appsv1.ClusterComponentConfig) bool {
	if !pointer.BoolDeref(config.ExternalManaged, false) {
		return false
	}
	if pointer.StringDeref(config.Name, "") == "" {
		return false
	}
	if config.ConfigMap != nil {
		return false
	}
	return true
}

func hasValidConfigObject(config appsv1.ClusterComponentConfig) bool {
	if !pointer.BoolDeref(config.ExternalManaged, false) {
		return false
	}
	if pointer.StringDeref(config.Name, "") == "" {
		return false
	}
	return config.ConfigMap != nil
}

func handleShardingComponent(reqCtx intctrlutil.RequestCtx, reader client.Client, cluster *appsv1.Cluster) error {
	checkAndUpdateConfigObjectForSharding := func(shardingName string, config *appsv1.ClusterComponentConfig, shardingComps []*appsv1.ClusterComponentSpec) error {
		for _, comp := range shardingComps {
			if err := updateConfigsForComponent(reqCtx, reader, cluster, comp); err != nil {
				return err
			}
		}
		// Wait until all components of sharding have generated the parameters-related ConfigMap object.
		match := func(compSpec *appsv1.ClusterComponentSpec) bool {
			for _, compConfig := range compSpec.Configs {
				if pointer.StringEqual(compConfig.Name, config.Name) {
					return hasValidConfigObject(compConfig)
				}
			}
			return false
		}
		if generics.CountFunc(shardingComps, match) == len(shardingComps) {
			// TODO: The Configs API does not support sharding component.
			// Use a PlaceHolder to associate the sharding component with the cm objects rendered by the parameter controller.
			config.ConfigMap = &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: parameterTemplateObjectName(cluster.Name, pointer.StringDeref(config.Name, "")),
				},
			}
		}
		return nil
	}

	for i := range cluster.Spec.Shardings {
		shardingSpec := &cluster.Spec.Shardings[i]
		shardingComps, err := intctrlutil.GenShardingCompSpecList(reqCtx.Ctx, reader, cluster, shardingSpec)
		if err != nil {
			return err
		}
		for j, config := range shardingSpec.Template.Configs {
			if !needUpdateConfigObject(config) {
				continue
			}
			if err := checkAndUpdateConfigObjectForSharding(shardingSpec.Name, &shardingSpec.Template.Configs[j], shardingComps); err != nil {
				return err
			}
		}
	}
	return nil
}

func parameterTemplateObjectName(clusterName, tplName string) string {
	return configcore.GetComponentCfgName(clusterName, constant.KBComponentNamePlaceHolder, tplName)
}

func (r *ParameterTemplateExtensionReconciler) update(reqCtx intctrlutil.RequestCtx, running, expected *appsv1.Cluster) (ctrl.Result, error) {
	if reflect.DeepEqual(running.Spec, expected.Spec) {
		return ctrl.Result{}, nil
	}
	if err := r.Client.Patch(reqCtx.Ctx, expected, client.MergeFrom(running)); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}
