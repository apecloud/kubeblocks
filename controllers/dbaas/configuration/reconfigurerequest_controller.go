/*
Copyright ApeCloud Inc.

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

package configuration

import (
	"context"
	"encoding/json"
	"fmt"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReconfigureRequestReconciler reconciles a ReconfigureRequest object
type ReconfigureRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var ConfigurationRequiredLabels = []string{
	intctrlutil.AppNameLabelKey,
	intctrlutil.AppInstanceLabelKey,
	intctrlutil.AppComponentLabelKey,
	cfgcore.CMConfigurationTplNameLabelKey,
	cfgcore.CMConfigurationTypeLabelKey,
	cfgcore.CMConfigurationISVTplLabelKey,
}

//+kubebuilder:rbac:groups=core,resources=configmap,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmap/finalizers,verbs=update

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

	if hash, ok := config.Labels[cfgcore.CMInsConfigurationHashLabelKey]; ok && hash == config.ResourceVersion {
		return intctrlutil.Reconciled()
	}

	isAppliedCfg, err := applyConfigurationChange(r.Client, reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to check last-applied-configuration")
	} else if isAppliedCfg {
		return intctrlutil.Reconciled()
	}

	if cfgConstraintsName, ok := config.Labels[cfgcore.CMConfigurationConstraintsNameLabelKey]; !ok || len(cfgConstraintsName) == 0 {
		reqCtx.Log.Info("configuration not set ConfigConstraints, not support reconfigure.", "config cm", client.ObjectKeyFromObject(config))
		return intctrlutil.Reconciled()
	}

	tpl := &dbaasv1alpha1.ConfigConstraint{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Labels[cfgcore.CMConfigurationConstraintsNameLabelKey],
	}, tpl); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	return r.sync(reqCtx, config, tpl)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconfigureRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(checkConfigurationObject)).
		Complete(r)
}

func checkConfigurationObject(object client.Object) bool {
	return checkConfigurationLabels(object, ConfigurationRequiredLabels)
}

func (r *ReconfigureRequestReconciler) sync(reqCtx intctrlutil.RequestCtx, config *corev1.ConfigMap, tpl *dbaasv1alpha1.ConfigConstraint) (ctrl.Result, error) {

	var (
		stsLists   = appv1.StatefulSetList{}
		cluster    = dbaasv1alpha1.Cluster{}
		clusterKey = client.ObjectKey{
			Namespace: config.GetNamespace(),
			Name:      config.Labels[intctrlutil.AppInstanceLabelKey],
		}

		configKey = client.ObjectKeyFromObject(config)

		configTplName     = config.Labels[cfgcore.CMConfigurationISVTplLabelKey]
		configTplLabelKey = cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName)
	)

	componentLabels := map[string]string{
		intctrlutil.AppNameLabelKey:      config.Labels[intctrlutil.AppNameLabelKey],
		intctrlutil.AppInstanceLabelKey:  config.Labels[intctrlutil.AppInstanceLabelKey],
		intctrlutil.AppComponentLabelKey: config.Labels[intctrlutil.AppComponentLabelKey],
		configTplLabelKey:                config.GetName(),
	}

	versionMeta, err := getConfigurationVersion(config, reqCtx, &tpl.Spec)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// Not any parameters updated
	if !versionMeta.IsModify {
		return r.updateCfgStatus(reqCtx, config, ReconfigureNoChangeType)
	}

	reqCtx.Log.Info(fmt.Sprintf("reconfigure params: \n\tadd: %s\n\tdelete: %s\n\tupdate: %s",
		versionMeta.AddConfig,
		versionMeta.DeleteConfig,
		versionMeta.UpdateConfig))

	// Find Cluster CR
	if err := r.Client.Get(reqCtx.Ctx, clusterKey, &cluster); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get cluster. name[%s]", clusterKey),
			reqCtx.Log)
	}

	// Find ClusterComponent from cluster cr
	componentName := config.Labels[intctrlutil.AppComponentLabelKey]
	clusterComponent := getClusterComponentsByName(cluster.Spec.Components, componentName)
	// fix cluster maybe not any component
	if clusterComponent == nil {
		reqCtx.Log.Info("not found component.", "componentName", componentName,
			"clusterName", cluster.GetName())
	} else {
		componentName = clusterComponent.Type
	}

	// Find ClusterDefinition Component  from ClusterDefinition CR
	component, err := getComponentFromClusterDefinition(reqCtx.Ctx, r.Client, &cluster, componentName)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get component from cluster definition. type[%s]", componentName),
			reqCtx.Log)
	} else if component == nil {
		reqCtx.Log.Info(fmt.Sprintf("failed to found component which the configuration is associated, component name: %s", componentName))
		return intctrlutil.Reconciled()
	}

	if component.ConfigSpec == nil {
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
	sts, containersList := getComponentByUsingCM(&stsLists, configKey)
	if len(sts) == 0 {
		reqCtx.Log.Info("configmap is not used by any container.", "cm name", configKey)
		return intctrlutil.Reconciled()
	}

	return r.performUpgrade(reconfigureParams{
		TplName:                  configTplName,
		Meta:                     versionMeta,
		Cfg:                      config,
		Tpl:                      &tpl.Spec,
		Client:                   r.Client,
		Ctx:                      reqCtx,
		Cluster:                  &cluster,
		ContainerNames:           containersList,
		ComponentUnits:           sts,
		Component:                component,
		ClusterComponent:         clusterComponent,
		Restart:                  !cfgcm.IsSupportReload(tpl.Spec.ReloadOptions),
		ReconfigureClientFactory: GetClientFactory(),
	})
}

func (r *ReconfigureRequestReconciler) updateCfgStatus(reqCtx intctrlutil.RequestCtx, cfg *corev1.ConfigMap, reconfigureType string) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, reqCtx.Log)
	}

	if ok, err := updateAppliedConfiguration(r.Client, reqCtx, cfg, configData, reconfigureType); err != nil || !ok {
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, reqCtx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureRequestReconciler) performUpgrade(params reconfigureParams) (ctrl.Result, error) {
	policy, err := NewReconfigurePolicy(params.Tpl, params.Meta, getUpgradePolicy(params.Cfg), params.Restart)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}

	returnedStatus, err := policy.Upgrade(params)
	if err := r.handleConfigEvent(params, cfgcore.PolicyExecStatus{
		PolicyName:    policy.GetPolicyName(),
		ExecStatus:    string(returnedStatus.Status),
		SucceedCount:  returnedStatus.SucceedCount,
		ExpectedCount: returnedStatus.ExpectedCount,
	}, err); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.Cfg, r.Recorder, err, params.Ctx.Log)
	}

	switch returnedStatus.Status {
	case ESRetry, ESAndRetryFailed:
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, params.Ctx.Log, "")
	case ESNone:
		return r.updateCfgStatus(params.Ctx, params.Cfg, policy.GetPolicyName())
	case ESFailed:
		if err := setCfgUpgradeFlag(params.Client, params.Ctx, params.Cfg, false); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, params.Ctx.Log, "")
		}
		return intctrlutil.Reconciled()
	default:
		return intctrlutil.Reconciled()
	}
}

func (r *ReconfigureRequestReconciler) handleConfigEvent(params reconfigureParams, status cfgcore.PolicyExecStatus, err error) error {
	var (
		cm             = params.Cfg
		lastOpsRequest = ""
	)

	if len(cm.Annotations) != 0 {
		lastOpsRequest = cm.Annotations[cfgcore.LastAppliedOpsCRAnnotation]
	}

	eventContext := cfgcore.ConfigEventContext{
		TplName:        params.TplName,
		Client:         params.Client,
		ReqCtx:         params.Ctx,
		Cluster:        params.Cluster,
		Component:      params.Component,
		Meta:           params.Meta,
		Tpl:            params.Tpl,
		Cfg:            params.Cfg,
		ComponentUnits: params.ComponentUnits,
		PolicyStatus:   status,
	}

	for _, handler := range cfgcore.ConfigEventHandlerMap {
		if err := handler.Handle(eventContext, lastOpsRequest, fromReconfigureStatus(ExecStatus(status.ExecStatus)), err); err != nil {
			return err
		}
	}
	return nil
}

func fromReconfigureStatus(status ExecStatus) dbaasv1alpha1.Phase {
	switch status {
	case ESFailed:
		return dbaasv1alpha1.FailedPhase
	case ESNone:
		return dbaasv1alpha1.SucceedPhase
	default:
		return dbaasv1alpha1.ReconfiguringPhase
	}
}
