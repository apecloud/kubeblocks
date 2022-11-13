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

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component"
	dbaasconfig "github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	cfgpolicy "github.com/apecloud/kubeblocks/controllers/dbaas/configuration/policy"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	appInstanceLabelKey  = "app.kubernetes.io/instance"
	appComponentLabelKey = "app.kubernetes.io/component-name"
	appNameLabelKey      = "app.kubernetes.io/name"
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
	dbaasconfig.CMConfigurationTplNameLabelKey,
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

	if !checkConfigurationObject(config) {
		return intctrlutil.Reconciled()
	}

	if hash, ok := config.Labels[dbaasconfig.CMInsConfigurationHashLabelKey]; ok && hash == config.ResourceVersion {
		return intctrlutil.Reconciled()
	}

	isAppliedCfg, err := dbaasconfig.ApplyConfigurationChange(r.Client, reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to check last-applied-configuration")
	} else if isAppliedCfg {
		return intctrlutil.Reconciled()
	}

	tpl := &dbaasv1alpha1.ConfigurationTemplate{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Labels[dbaasconfig.CMConfigurationTplNameLabelKey],
	}, tpl); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	return r.sync(reqCtx, config, tpl)
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

func (r *ReconfigureRequestReconciler) sync(reqCtx intctrlutil.RequestCtx, config *corev1.ConfigMap, tpl *dbaasv1alpha1.ConfigurationTemplate) (ctrl.Result, error) {

	var (
		stsLists   = appv1.StatefulSetList{}
		cluster    = dbaasv1alpha1.Cluster{}
		clusterKey = client.ObjectKey{
			Namespace: config.GetNamespace(),
			Name:      config.Labels[appInstanceLabelKey],
		}

		configTplName     = config.Labels[dbaasconfig.CMConfigurationTplNameLabelKey]
		configTplLabelKey = dbaasconfig.GenerateUniqLabelKeyWithConfig(configTplName)
	)

	componentLabels := map[string]string{
		appNameLabelKey:      config.Labels[appNameLabelKey],
		appInstanceLabelKey:  config.Labels[appInstanceLabelKey],
		appComponentLabelKey: config.Labels[appComponentLabelKey],
		configTplLabelKey:    config.Labels[dbaasconfig.CMConfigurationTplNameLabelKey],
	}

	versionMeta, err := dbaasconfig.GetConfigurationVersion(config, reqCtx, &tpl.Spec)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// Not any parameters updated
	if !versionMeta.IsModify {
		return r.updateCfgStatus(reqCtx, versionMeta, config)
	}

	// Find Cluster CR
	if err := r.Client.Get(reqCtx.Ctx, clusterKey, &cluster); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get cluster. name[%s]", clusterKey),
			reqCtx.Log)
	}

	// Find ClusterComponent from cluster cr
	componentName := config.Labels[appComponentLabelKey]
	clusterComponent := getClusterComponentsByName(cluster.Spec.Components, componentName)
	if clusterComponent == nil {
		// TODO(zt) how to process found component!
		reqCtx.Log.Info("not found component.", "componentName", componentName,
			"clusterName", cluster.GetName())
		return intctrlutil.Reconciled()
	}

	// Find ClusterDefinition Component  from ClusterDefinition CR
	component, err := component.GetComponentFromClusterDefinition(reqCtx.Ctx, r.Client, &cluster, clusterComponent.Type)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get component from cluster definition. type[%s]", clusterComponent.Type),
			reqCtx.Log)
	}

	if ok, _ := cfgcm.NeedBuildConfigSidecar(component.ConfigAutoReload, component.ConfigReloadType, component.ReloadConfiguration); !ok {
		return intctrlutil.Reconciled()
	}

	// find STS CR
	if err := r.Client.List(reqCtx.Ctx, &stsLists, client.InNamespace(config.Namespace), client.MatchingLabels(componentLabels)); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get component. configmap[%s] label[%s]",
				reqCtx.Req.NamespacedName, componentLabels),
			reqCtx.Log)
	}

	// configmap has never been used
	sts := dbaasconfig.GetComponentByUsingCM(&stsLists, client.ObjectKeyFromObject(config))
	if len(sts) == 0 {
		return intctrlutil.Reconciled()
	}

	return r.performUpgrade(cfgpolicy.ReconfigureParams{
		TplName:          tpl.GetName(),
		Meta:             versionMeta,
		Cfg:              config,
		Tpl:              &tpl.Spec,
		Client:           r.Client,
		Ctx:              reqCtx,
		Cluster:          &cluster,
		ComponentUnits:   sts,
		Component:        component,
		ClusterComponent: clusterComponent,
	})
}

func (r *ReconfigureRequestReconciler) updateCfgStatus(reqCtx intctrlutil.RequestCtx, meta *cfgcore.ConfigDiffInformation, cfg *corev1.ConfigMap) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, reqCtx.Log)
	}

	if ok, err := dbaasconfig.UpdateAppliedConfiguration(r.Client, reqCtx, cfg, configData); err != nil || !ok {
		return intctrlutil.RequeueAfter(dbaasconfig.ConfigReconcileInterval, reqCtx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureRequestReconciler) performUpgrade(params cfgpolicy.ReconfigureParams) (ctrl.Result, error) {
	// TODO(zt) process update policy

	policy, err := cfgpolicy.NewReconfigurePolicy(params.Tpl, params.Meta, dbaasconfig.GetUpgradePolicy(params.Cfg))
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}

	execStatus, err := policy.Upgrade(params)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}

	switch execStatus {
	case cfgpolicy.ES_Retry:
		return intctrlutil.RequeueAfter(dbaasconfig.ConfigReconcileInterval, params.Ctx.Log, "")
	case cfgpolicy.ES_None:
		return r.updateCfgStatus(params.Ctx, params.Meta, params.Cfg)
	case cfgpolicy.ES_Failed:
		if err := dbaasconfig.SetCfgUpgradeFlag(params.Client, params.Ctx, params.Cfg, false); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, params.Ctx.Log, "")
		}
		return intctrlutil.Reconciled()
	default:
		return intctrlutil.Reconciled()
	}
}
