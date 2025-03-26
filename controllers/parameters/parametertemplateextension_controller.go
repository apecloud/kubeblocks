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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
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
		Complete(r)
}

func (r *ParameterTemplateExtensionReconciler) reconcile(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) (ctrl.Result, error) {
	extractor := func(comp appsv1.ClusterComponentSpec) string {
		return comp.ComponentDef
	}

	paramTplAsMap := make(map[string][]appsv1.ComponentFileTemplate)
	cmpds := generics.Map(cluster.Spec.ComponentSpecs, extractor)
	for _, cmpd := range cmpds {
		paramTpl, err := resolveParameterTemplate(r.Client, reqCtx.Ctx, cmpd)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		paramTplAsMap[cmpd] = paramTpl
	}
	return r.update(reqCtx, cluster, paramTplAsMap)
}

func (r *ParameterTemplateExtensionReconciler) update(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster, paramTplAsMap map[string][]appsv1.ComponentFileTemplate) (ctrl.Result, error) {
	patch := client.MergeFrom(cluster.DeepCopy())
	for i := range cluster.Spec.ComponentSpecs {
		updateParameterConfigExtension(&cluster.Spec.ComponentSpecs[i], paramTplAsMap, cluster.Name)
	}
	if err := r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func updateParameterConfigExtension(compSpec *appsv1.ClusterComponentSpec, paramTplAsMap map[string][]appsv1.ComponentFileTemplate, clusterName string) {
	newExtension := func(template appsv1.ComponentFileTemplate, compName string) appsv1.ClusterComponentConfig {
		return appsv1.ClusterComponentConfig{
			Name: pointer.String(template.Name),
			ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configcore.GetComponentCfgName(clusterName, compName, template.Name),
					},
				},
			},
			ExternalManaged: pointer.Bool(true),
		}
	}

	for _, template := range paramTplAsMap[compSpec.ComponentDef] {
		match := func(config appsv1.ClusterComponentConfig) bool {
			return pointer.StringDeref(config.Name, "") == template.Name
		}
		index := generics.FindFirstFunc(compSpec.Configs, match)
		if index < 0 {
			compSpec.Configs = append(compSpec.Configs, newExtension(template, compSpec.Name))
		} else {
			compSpec.Configs[index] = newExtension(template, compSpec.Name)
		}
	}
}

func resolveParameterTemplate(reader client.Reader, ctx context.Context, cmpdName string) ([]appsv1.ComponentFileTemplate, error) {
	cmpd, err := getCompDefinition(ctx, reader, cmpdName)
	if err != nil {
		return nil, err
	}
	if len(cmpd.Spec.Configs) == 0 {
		return nil, nil
	}

	pcr, err := intctrlutil.ResolveComponentConfigRender(ctx, reader, cmpd)
	if err != nil {
		return nil, err
	}
	return configctrl.ResolveParameterTemplate(cmpd.Spec, pcr.Spec), nil
}
