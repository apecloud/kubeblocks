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
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/configmanager"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ReconfigureReconciler reconciles a ReconfigureRequest object
type ReconfigureReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	ConfigReconcileInterval = time.Second * 1
)

const (
	configurationNoChangedMessage           = "the configuration file has not been modified, skip reconfigure"
	configurationNotUsingMessage            = "the configmap is not used by any container, skip reconfigure"
	configurationNotRelatedComponentMessage = "related component does not found any configSpecs, skip reconfigure"
)

var reconfigureRequiredLabels = []string{
	constant.AppInstanceLabelKey,
	constant.KBAppComponentLabelKey,
	constant.CMConfigurationTemplateNameLabelKey,
	constant.CMConfigurationTypeLabelKey,
	constant.CMConfigurationSpecProviderLabelKey,
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ReconfigureRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ReconfigureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithName("ReconfigureRequestReconcile").WithValues("ConfigMap", req.NamespacedName),
		Recorder: r.Recorder,
	}

	config := &corev1.ConfigMap{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, config); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if model.IsObjectDeleting(config) {
		return intctrlutil.Reconciled()
	}
	if !checkConfigurationObject(config) {
		return intctrlutil.Reconciled()
	}

	reqCtx.Log = reqCtx.Log.
		WithValues("ClusterName", config.Labels[constant.AppInstanceLabelKey]).
		WithValues("ComponentName", config.Labels[constant.KBAppComponentLabelKey])

	isAppliedConfigs, err := checkAndApplyConfigsChanged(r.Client, reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
			errors.Wrap(err, "failed to check last-applied-configuration").Error())
	}
	if isAppliedConfigs {
		return updateConfigPhase(r.Client, reqCtx, config, parametersv1alpha1.CFinishedPhase, configurationNoChangedMessage)
	}

	configSpec, err := r.getConfigSpec(reqCtx, config)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
			errors.Wrap(err, "failed to fetch related resources").Error())
	}
	if configSpec == nil {
		reqCtx.Log.Info(fmt.Sprintf("not found configSpec[%s] in the component[%s].",
			config.Labels[constant.CMConfigurationSpecProviderLabelKey], config.Labels[constant.KBAppComponentLabelKey]))
		reqCtx.Recorder.Event(config,
			corev1.EventTypeWarning,
			appsv1alpha1.ReasonReconfigureFailed,
			configurationNotRelatedComponentMessage)
		return updateConfigPhase(r.Client, reqCtx, config, parametersv1alpha1.CFinishedPhase, configurationNotRelatedComponentMessage)
	}

	return r.sync(reqCtx, config, configSpec)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconfigureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers) / 4,
		}).
		WithEventFilter(predicate.NewPredicateFuncs(checkConfigurationObject)).
		Complete(r)
}

func checkConfigurationObject(object client.Object) bool {
	return checkConfigLabels(object, reconfigureRequiredLabels)
}

func (r *ReconfigureReconciler) getConfigSpec(reqCtx intctrlutil.RequestCtx, cm *corev1.ConfigMap) (*appsv1.ComponentFileTemplate, error) {
	configSpecName, ok := cm.Labels[constant.CMConfigurationSpecProviderLabelKey]
	if !ok {
		return nil, nil
	}

	key := client.ObjectKey{
		Namespace: cm.Namespace,
		Name:      core.GenerateComponentConfigurationName(cm.Labels[constant.AppInstanceLabelKey], cm.Labels[constant.KBAppComponentLabelKey]),
	}
	obj := &parametersv1alpha1.ComponentParameter{}
	if err := r.Client.Get(reqCtx.Ctx, key, obj); err != nil {
		return nil, err
	}

	configSpec := parameters.GetConfigTemplateItem(&obj.Spec, configSpecName)
	if configSpec == nil {
		return nil, fmt.Errorf("not found config spec: %s in configuration[%s]", configSpecName, obj.Name)
	}
	return configSpec.ConfigSpec, nil
}

func (r *ReconfigureReconciler) sync(reqCtx intctrlutil.RequestCtx, configMap *corev1.ConfigMap, configSpec *appsv1.ComponentFileTemplate) (ctrl.Result, error) {
	clusterName := configMap.Labels[constant.AppInstanceLabelKey]
	componentName := configMap.Labels[constant.KBAppComponentLabelKey]
	rctx := newParameterReconcileContext(reqCtx,
		&render.ResourceCtx{
			Context:       reqCtx.Ctx,
			Client:        r.Client,
			Namespace:     configMap.Namespace,
			ClusterName:   clusterName,
			ComponentName: componentName,
		},
		configMap,
		nil,
		map[string]string{
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: componentName,
		})
	if err := rctx.GetRelatedObjects(); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}

	// Assumption: It is required that the cluster must have a component.
	if rctx.ClusterComObj == nil {
		reqCtx.Log.Info("not found component.")
		return intctrlutil.Reconciled()
	}

	if len(rctx.InstanceSetList) == 0 {
		reqCtx.Recorder.Event(configMap, corev1.EventTypeWarning, appsv1alpha1.ReasonReconfigureFailed,
			"the configmap is not used by any container, skip reconfigure")
		return updateConfigPhase(r.Client, reqCtx, configMap, parametersv1alpha1.CFinishedPhase, configurationNotUsingMessage)
	}

	configPatch, forceRestart, err := createConfigPatch(configMap, rctx.ConfigRender, rctx.ParametersDefs)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}

	// No parameters updated
	if configPatch != nil && !configPatch.IsModify {
		reqCtx.Recorder.Event(configMap, corev1.EventTypeNormal, appsv1alpha1.ReasonReconfigureRunning, "nothing changed, skip reconfigure")
		return r.updateConfigCMStatus(reqCtx, configMap, core.ReconfigureNoChangeType, nil)
	}

	if configPatch != nil {
		reqCtx.Log.V(1).Info(fmt.Sprintf(
			"reconfigure params: \n\tadd: %s\n\tdelete: %s\n\tupdate: %s",
			configPatch.AddConfig,
			configPatch.DeleteConfig,
			configPatch.UpdateConfig))
	}

	tasks, err := r.genReconfigureActionTasks(configSpec, rctx, configPatch, forceRestart)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}
	return r.performUpgrade(rctx, tasks)
}

func (r *ReconfigureReconciler) genReconfigureActionTasks(templateSpec *appsv1.ComponentFileTemplate, rctx *ReconcileContext, patch *core.ConfigPatchInfo, restart bool) ([]reconfigureTask, error) {
	var tasks []reconfigureTask

	// If the patch or ConfigRender is nil, return a single restart task.
	if patch == nil || rctx.ConfigRender == nil {
		return []reconfigureTask{r.buildRestartTask(templateSpec, rctx)}, nil
	}

	// needReloadAction determines if a reload action is needed based on the ParametersDefinition and ReloadPolicy.
	needReloadAction := func(pd *parametersv1alpha1.ParametersDefinition, policy parametersv1alpha1.ReloadPolicy) bool {
		return !restart || (policy == parametersv1alpha1.SyncDynamicReloadPolicy && parameters.NeedDynamicReloadAction(&pd.Spec))
	}

	for key, jsonPatch := range patch.UpdateConfig {
		pd, ok := rctx.ParametersDefs[key]
		// If the ParametersDefinition or its ReloadAction is nil, continue to the next iteration.
		if !ok || pd.Spec.ReloadAction == nil {
			continue
		}
		configFormat := parameters.GetComponentConfigDescription(&rctx.ConfigRender.Spec, key)
		if configFormat == nil || configFormat.FileFormatConfig == nil {
			continue
		}
		// Determine the appropriate ReloadPolicy.
		policy, err := r.resolveReconfigurePolicy(string(jsonPatch), configFormat.FileFormatConfig, &pd.Spec)
		if err != nil {
			return nil, err
		}
		// If a reload action is needed, append a new reload action task to the tasks slice.
		if needReloadAction(pd, policy) {
			tasks = append(tasks, r.buildReloadTask(policy, templateSpec, rctx, pd, configFormat, patch))
		}
	}

	// If no tasks were added, return a single restart task.
	if len(tasks) == 0 {
		return []reconfigureTask{r.buildRestartTask(templateSpec, rctx)}, nil
	}

	return tasks, nil
}

func (r *ReconfigureReconciler) buildReloadTask(policy parametersv1alpha1.ReloadPolicy,
	templateSpec *appsv1.ComponentFileTemplate,
	rctx *ReconcileContext,
	pd *parametersv1alpha1.ParametersDefinition,
	configDescription *parametersv1alpha1.ComponentConfigDescription,
	patch *core.ConfigPatchInfo) reconfigureTask {
	reCtx := reconfigureContext{
		RequestCtx:               rctx.RequestCtx,
		Client:                   rctx.Client,
		ConfigTemplate:           *templateSpec,
		ConfigMap:                rctx.ConfigMap,
		ParametersDef:            &pd.Spec,
		ConfigDescription:        configDescription,
		Cluster:                  rctx.ClusterObj,
		InstanceSetUnits:         rctx.InstanceSetList,
		ClusterComponent:         rctx.ClusterComObj,
		SynthesizedComponent:     rctx.BuiltinComponent,
		ReconfigureClientFactory: getClientFactory(),
		Patch:                    patch,
	}
	return reconfigureTask{policy: policy, taskCtx: reCtx}
}

func (r *ReconfigureReconciler) buildRestartTask(configTemplate *appsv1.ComponentFileTemplate, rctx *ReconcileContext) reconfigureTask {
	return reconfigureTask{
		policy: parametersv1alpha1.RestartPolicy,
		taskCtx: reconfigureContext{
			RequestCtx:           rctx.RequestCtx,
			Client:               rctx.Client,
			ConfigTemplate:       *configTemplate,
			ClusterComponent:     rctx.ClusterComObj,
			Cluster:              rctx.ClusterObj,
			SynthesizedComponent: rctx.BuiltinComponent,
			InstanceSetUnits:     rctx.InstanceSetList,
		},
	}
}

func (r *ReconfigureReconciler) resolveReconfigurePolicy(jsonPatch string, format *parametersv1alpha1.FileFormatConfig,
	pd *parametersv1alpha1.ParametersDefinitionSpec) (parametersv1alpha1.ReloadPolicy, error) {
	var policy = parametersv1alpha1.NonePolicy
	dynamicUpdate, err := core.CheckUpdateDynamicParameters(format, pd, jsonPatch)
	if err != nil {
		return policy, err
	}

	// make decision
	switch {
	case !dynamicUpdate && parameters.NeedDynamicReloadAction(pd): // static parameters update and need to do hot update
		policy = parametersv1alpha1.DynamicReloadAndRestartPolicy
	case !dynamicUpdate: // static parameters update and only need to restart
		policy = parametersv1alpha1.RestartPolicy
	case cfgcm.IsAutoReload(pd.ReloadAction): // if core support hot update, don't need to do anything
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	case enableSyncTrigger(pd.ReloadAction): // sync config-manager exec hot update
		policy = parametersv1alpha1.SyncDynamicReloadPolicy
	default: // config-manager auto trigger to hot update
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	}
	return policy, nil
}

func (r *ReconfigureReconciler) updateConfigCMStatus(reqCtx intctrlutil.RequestCtx, cfg *corev1.ConfigMap, reconfigureType string, result *parameters.Result) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, reqCtx.Log)
	}

	if ok, err := updateAppliedConfigs(r.Client, reqCtx, cfg, configData, reconfigureType, result); err != nil || !ok {
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, reqCtx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureReconciler) performUpgrade(rctx *ReconcileContext, tasks []reconfigureTask) (ctrl.Result, error) {
	var (
		err    error
		policy string
		status returnedStatus
	)
	for _, task := range tasks {
		policy = string(task.policy)
		status, err = task.reconfigure()
		if err != nil || status.Status != ESNone {
			return r.status(rctx, status, policy, err)
		}
	}
	return r.succeed(rctx, policy, status)
}

func (r *ReconfigureReconciler) status(rctx *ReconcileContext, returnedStatus returnedStatus, policy string, err error) (ctrl.Result, error) {
	updatePhase := func(phase parametersv1alpha1.ParameterPhase, options ...options) (ctrl.Result, error) {
		return updateConfigPhaseWithResult(rctx.Client, rctx.RequestCtx, rctx.ConfigMap, reconciled(returnedStatus, policy, phase, options...))
	}

	switch returnedStatus.Status {
	case ESFailedAndRetry:
		return updatePhase(parametersv1alpha1.CFailedPhase, withFailed(err, true))
	case ESRetry:
		return updatePhase(parametersv1alpha1.CUpgradingPhase)
	case ESFailed:
		return updatePhase(parametersv1alpha1.CFailedAndPausePhase, withFailed(err, false))
	case ESNone:
		return r.succeed(rctx, policy, returnedStatus)
	default:
		return updatePhase(parametersv1alpha1.CFailedAndPausePhase, withFailed(core.MakeError("unknown status"), false))
	}
}

func (r *ReconfigureReconciler) succeed(rctx *ReconcileContext, policy string, status returnedStatus) (ctrl.Result, error) {
	rctx.Recorder.Eventf(rctx.ConfigMap,
		corev1.EventTypeNormal,
		appsv1alpha1.ReasonReconfigureSucceed,
		"the reconfigure[%s] has been processed successfully",
		policy)
	result := reconciled(status, policy, parametersv1alpha1.CFinishedPhase)
	return r.updateConfigCMStatus(rctx.RequestCtx, rctx.ConfigMap, policy, &result)
}
