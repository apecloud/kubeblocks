/*
Copyright ApeCloud, Inc.

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
	"strings"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReconfigureRequestReconciler reconciles a ReconfigureRequest object
type ReconfigureRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var ConfigurationRequiredLabels = []string{
	constant.AppNameLabelKey,
	constant.AppInstanceLabelKey,
	constant.KBAppComponentLabelKey,
	constant.CMConfigurationTemplateNameLabelKey,
	constant.CMConfigurationTypeLabelKey,
	constant.CMConfigurationSpecProviderLabelKey,
}

// +kubebuilder:rbac:groups=core,resources=configmap,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmap/finalizers,verbs=update

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
		Log:      log.FromContext(ctx).WithName("ReconfigureRequestReconcile").WithValues("ConfigMap", req.NamespacedName),
		Recorder: r.Recorder,
	}

	config := &corev1.ConfigMap{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, config); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "not find configmap")
	}

	if !checkConfigurationObject(config) {
		return intctrlutil.Reconciled()
	}

	reqCtx.Log = reqCtx.Log.
		WithValues("ClusterName", config.Labels[constant.AppInstanceLabelKey]).
		WithValues("ComponentName", config.Labels[constant.KBAppComponentLabelKey])
	if hash, ok := config.Labels[constant.CMInsConfigurationHashLabelKey]; ok && hash == config.ResourceVersion {
		return intctrlutil.Reconciled()
	}

	isAppliedConfigs, err := checkAndApplyConfigsChanged(r.Client, reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to check last-applied-configuration")
	} else if isAppliedConfigs {
		return intctrlutil.Reconciled()
	}

	if cfgConstraintsName, ok := config.Labels[constant.CMConfigurationConstraintsNameLabelKey]; !ok || len(cfgConstraintsName) == 0 {
		reqCtx.Log.V(1).Info("configuration not set ConfigConstraints, not support reconfigure.")
		return intctrlutil.Reconciled()
	}

	tpl := &appsv1alpha1.ConfigConstraint{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Labels[constant.CMConfigurationConstraintsNameLabelKey],
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
	return checkConfigLabels(object, ConfigurationRequiredLabels)
}

func (r *ReconfigureRequestReconciler) sync(reqCtx intctrlutil.RequestCtx, config *corev1.ConfigMap, tpl *appsv1alpha1.ConfigConstraint) (ctrl.Result, error) {

	var (
		stsLists   = appv1.StatefulSetList{}
		cluster    = appsv1alpha1.Cluster{}
		clusterKey = client.ObjectKey{
			Namespace: config.GetNamespace(),
			Name:      config.Labels[constant.AppInstanceLabelKey],
		}

		configKey = client.ObjectKeyFromObject(config)

		componentName  = config.Labels[constant.KBAppComponentLabelKey]
		configSpecName = config.Labels[constant.CMConfigurationSpecProviderLabelKey]
	)

	componentLabels := map[string]string{
		constant.AppNameLabelKey:        config.Labels[constant.AppNameLabelKey],
		constant.AppInstanceLabelKey:    config.Labels[constant.AppInstanceLabelKey],
		constant.KBAppComponentLabelKey: config.Labels[constant.KBAppComponentLabelKey],
	}

	var keySelector []string
	if keysLabel, ok := config.Labels[constant.CMConfigurationCMKeysLabelKey]; ok && keysLabel != "" {
		keySelector = strings.Split(keysLabel, ",")
	}

	configPatch, forceRestart, err := createConfigPatch(config, tpl.Spec.FormatterConfig.Format, keySelector)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// Not any parameters updated
	if !configPatch.IsModify {
		return r.updateConfigCMStatus(reqCtx, config, ReconfigureNoChangeType)
	}

	reqCtx.Log.V(1).Info(fmt.Sprintf("reconfigure params: \n\tadd: %s\n\tdelete: %s\n\tupdate: %s",
		configPatch.AddConfig,
		configPatch.DeleteConfig,
		configPatch.UpdateConfig))

	// Find Cluster CR
	if err := r.Client.Get(reqCtx.Ctx, clusterKey, &cluster); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err, "failed to get cluster. name[%s]", clusterKey),
			reqCtx.Log)
	}

	// Find ClusterComponentSpec from cluster cr
	clusterComponent := cluster.GetComponentByName(componentName)
	// Assumption: It is required that the cluster must have a component.
	if clusterComponent == nil {
		reqCtx.Log.Info("not found component.")
		return intctrlutil.Reconciled()
	}

	// Find ClusterDefinition Component  from ClusterDefinition CR
	component, err := getComponentFromClusterDefinition(reqCtx.Ctx, r.Client, &cluster, clusterComponent.ComponentDefRef)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config,
			r.Recorder,
			cfgcore.WrapError(err,
				"failed to get component from cluster definition. type[%s]", clusterComponent.ComponentDefRef),
			reqCtx.Log)
	} else if component == nil {
		reqCtx.Log.Error(cfgcore.MakeError("failed to find component which the configuration is associated."), "ignore the configmap")
		return intctrlutil.Reconciled()
	}

	if len(component.ConfigSpecs) == 0 {
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
	sts, containersList := getAssociatedComponentsByConfigmap(&stsLists, configKey, configSpecName)
	if len(sts) == 0 {
		reqCtx.Log.Info("configmap is not used by any container.")
		return intctrlutil.Reconciled()
	}

	return r.performUpgrade(reconfigureParams{
		ConfigSpecName:           configSpecName,
		ConfigPatch:              configPatch,
		ConfigMap:                config,
		ConfigConstraint:         &tpl.Spec,
		Client:                   r.Client,
		Ctx:                      reqCtx,
		Cluster:                  &cluster,
		ContainerNames:           containersList,
		ComponentUnits:           sts,
		Component:                component,
		ClusterComponent:         clusterComponent,
		Restart:                  forceRestart || !cfgcm.IsSupportReload(tpl.Spec.ReloadOptions),
		ReconfigureClientFactory: GetClientFactory(),
	})
}

func (r *ReconfigureRequestReconciler) updateConfigCMStatus(reqCtx intctrlutil.RequestCtx, cfg *corev1.ConfigMap, reconfigureType string) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, reqCtx.Log)
	}

	if ok, err := updateAppliedConfigs(r.Client, reqCtx, cfg, configData, reconfigureType); err != nil || !ok {
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, reqCtx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureRequestReconciler) performUpgrade(params reconfigureParams) (ctrl.Result, error) {
	policy, err := NewReconfigurePolicy(params.ConfigConstraint, params.ConfigPatch, getUpgradePolicy(params.ConfigMap), params.Restart)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.ConfigMap, r.Recorder, err, params.Ctx.Log)
	}

	returnedStatus, err := policy.Upgrade(params)
	if err := r.handleConfigEvent(params, cfgcore.PolicyExecStatus{
		PolicyName:    policy.GetPolicyName(),
		ExecStatus:    string(returnedStatus.Status),
		SucceedCount:  returnedStatus.SucceedCount,
		ExpectedCount: returnedStatus.ExpectedCount,
	}, err); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.ConfigMap, r.Recorder, err, params.Ctx.Log)
	}
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(params.ConfigMap, r.Recorder, err, params.Ctx.Log)
	}

	switch returnedStatus.Status {
	case ESRetry, ESAndRetryFailed:
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, params.Ctx.Log, "")
	case ESNone:
		return r.updateConfigCMStatus(params.Ctx, params.ConfigMap, policy.GetPolicyName())
	case ESFailed:
		if err := setCfgUpgradeFlag(params.Client, params.Ctx, params.ConfigMap, false); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, params.Ctx.Log, "")
		}
		return intctrlutil.Reconciled()
	default:
		return intctrlutil.Reconciled()
	}
}

func (r *ReconfigureRequestReconciler) handleConfigEvent(params reconfigureParams, status cfgcore.PolicyExecStatus, err error) error {
	var (
		cm             = params.ConfigMap
		lastOpsRequest = ""
	)

	if len(cm.Annotations) != 0 {
		lastOpsRequest = cm.Annotations[constant.LastAppliedOpsCRAnnotation]
	}

	eventContext := cfgcore.ConfigEventContext{
		ConfigSpecName:   params.ConfigSpecName,
		Client:           params.Client,
		ReqCtx:           params.Ctx,
		Cluster:          params.Cluster,
		Component:        params.Component,
		ConfigPatch:      params.ConfigPatch,
		ConfigConstraint: params.ConfigConstraint,
		ConfigMap:        params.ConfigMap,
		ComponentUnits:   params.ComponentUnits,
		PolicyStatus:     status,
	}

	for _, handler := range cfgcore.ConfigEventHandlerMap {
		if err := handler.Handle(eventContext, lastOpsRequest, fromReconfigureStatus(ExecStatus(status.ExecStatus)), err); err != nil {
			return err
		}
	}
	return nil
}

func fromReconfigureStatus(status ExecStatus) appsv1alpha1.OpsPhase {
	switch status {
	case ESFailed:
		return appsv1alpha1.OpsFailedPhase
	case ESNone:
		return appsv1alpha1.OpsSucceedPhase
	default:
		return appsv1alpha1.OpsRunningPhase
	}
}
