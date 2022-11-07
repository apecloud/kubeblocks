/*
Copyright 2022 The KubeBlocks Authors

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

package dbaas

import (
	"context"
	"encoding/json"
	"k8s.io/apimachinery/pkg/types"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	dbaasconfig "github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	configutil "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReconfigureRequestReconciler reconciles a ReconfigureRequest object
type ReconfigureRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var ConfigurationRequiredLabels = []string{
	appNameLabelKey,
	appInstanceLabelKey,
	appComponentLabelKey,
	dbaasconfig.CMConfigurationTplLabelKey,
	dbaasconfig.CMInsConfigurationLabelKey,
}

//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=reconfigurerequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=reconfigurerequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=reconfigurerequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ReconfigureRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ReconfigureRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("Configuration", req.NamespacedName),
		Recorder: r.Recorder,
	}

	config := &corev1.ConfigMap{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, config); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "not find configmap", "key", req.NamespacedName)
	}

	if checkConfigurationObject(config) {
		intctrlutil.Reconciled()
	}

	if hash, ok := config.Labels[dbaasconfig.CMInsConfigurationHashLabelKey]; ok && hash == config.ResourceVersion {
		intctrlutil.Reconciled()
	}

	isAppliedCfg, err := dbaasconfig.ApplyConfigurationChange(r.Client, reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to check last-applied-configuration")
	} else if isAppliedCfg {
		intctrlutil.Reconciled()
	}

	tpl := &dbaasv1alpha1.ConfigurationTemplate{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, config); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Labels[dbaasconfig.CMConfigurationTplLabelKey],
	}, tpl); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	return r.sync(reqCtx, config, &tpl.Spec)
}

type ResourceConfigMapWithLabelPredicate struct {
	// hook default interface func
	predicate.Funcs
}

func (c *ResourceConfigMapWithLabelPredicate) Create(createEvent event.CreateEvent) bool {
	return checkConfigurationObject(createEvent.Object)
}

func (r *ResourceConfigMapWithLabelPredicate) Update(updateEvent event.UpdateEvent) bool {
	return checkConfigurationObject(updateEvent.ObjectNew)
}

func (r *ResourceConfigMapWithLabelPredicate) Delete(deleteEvent event.DeleteEvent) bool {
	return checkConfigurationObject(deleteEvent.Object)
}

func (r *ResourceConfigMapWithLabelPredicate) Generic(genericEvent event.GenericEvent) bool {
	return checkConfigurationObject(genericEvent.Object)
}

// type EnqueueRequestForConfigmap struct {
//	// hook default interface func
//	handler.Funcs
// }
//
//// Update process reconfigure
// func (e *EnqueueRequestForConfigmap) Update(event event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
//	//TODO implement me
//	panic("implement me")
// }

// SetupWithManager sets up the controller with the Manager.
func (r *ReconfigureRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		// Watches(&source.Kind{Type: &dbaasv1alpha1.ReconfigureRequest{}},
		//	&handler.EnqueueRequestForOwner{}).
		WithEventFilter(&ResourceConfigMapWithLabelPredicate{}).
		Complete(r)
}

func checkConfigurationObject(object client.Object) bool {
	return dbaasconfig.CheckConfigurationLabels(object, ConfigurationRequiredLabels)
}

func (r *ReconfigureRequestReconciler) sync(reqCtx intctrlutil.RequestCtx, config *corev1.ConfigMap, tpl *dbaasv1alpha1.ConfigurationTemplateSpec) (ctrl.Result, error) {

	var (
		clusters = dbaasv1alpha1.ClusterList{}
		stsLists = appv1.StatefulSetList{}
	)

	clusterLabels := map[string]string{
		appNameLabelKey:     config.Labels[appNameLabelKey],
		appInstanceLabelKey: config.Labels[appInstanceLabelKey],
	}

	componentLabels := map[string]string{
		appNameLabelKey:                        config.Labels[appNameLabelKey],
		appInstanceLabelKey:                    config.Labels[appInstanceLabelKey],
		appComponentLabelKey:                   config.Labels[appComponentLabelKey],
		dbaasconfig.CMInsConfigurationLabelKey: config.Labels[dbaasconfig.CMInsConfigurationLabelKey],
	}

	versionMeta, err := dbaasconfig.GetConfigurationVersion(config, reqCtx, tpl)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// Not any parameters updated
	if !versionMeta.IsModify {
		return r.updateCfgStatus(reqCtx, versionMeta, config)
	}

	// find Cluster CR
	if err := r.Client.List(reqCtx.Ctx, &clusters, client.InNamespace(config.Namespace), client.MatchingLabels(clusterLabels)); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			configutil.WrapError(err,
				"failed to get cluster. configmap[%s] label[%s]",
				reqCtx.Req.NamespacedName, clusterLabels),
			reqCtx.Log)
	}

	clusterLen := len(clusters.Items)
	if clusterLen > 1 {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			configutil.MakeError("get multi cluster[%d]. configmap[%s] label[%s]",
				clusterLen, reqCtx.Req.NamespacedName, clusterLabels),
			reqCtx.Log)
	} else if clusterLen == 0 {
		return intctrlutil.Reconciled()
	}

	// find STS CR
	if err := r.Client.List(reqCtx.Ctx, &stsLists, client.InNamespace(config.Namespace), client.MatchingLabels(componentLabels)); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			configutil.WrapError(err,
				"failed to get component. configmap[%s] label[%s]",
				reqCtx.Req.NamespacedName, componentLabels),
			reqCtx.Log)
	}
	componentLen := len(stsLists.Items)
	if componentLen == 0 {
		return intctrlutil.Reconciled()
	}

	return r.performUpdate(reqCtx, versionMeta, config, clusters.Items[0], stsLists.Items)
}

func (r *ReconfigureRequestReconciler) updateCfgStatus(ctx intctrlutil.RequestCtx, meta *configutil.ConfigDiffInformation, cfg *corev1.ConfigMap) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, ctx.Log)
	}

	if ok, err := dbaasconfig.UpdateAppliedConfiguration(r.Client, ctx, cfg, configData); err != nil || !ok {
		return intctrlutil.RequeueAfter(dbaasconfig.ConfigReconcileInterval, ctx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureRequestReconciler) performUpdate(ctx intctrlutil.RequestCtx, meta *configutil.ConfigDiffInformation, config *corev1.ConfigMap, cluster dbaasv1alpha1.Cluster, component []appv1.StatefulSet) (ctrl.Result, error) {
	// TODO(zt) process update policy

	r.updateCfgStatus(ctx, meta, config)
	intctrlutil.RecordCreatedEvent(r.Recorder, config)

	return intctrlutil.Reconciled()
}
