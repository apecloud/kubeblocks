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
	"reflect"
	"strconv"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/parameters/reconfigure"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/configmanager"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ReconfigureReconciler reconciles a ReconfigureRequest object
type ReconfigureReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	configReconcileInterval = time.Second * 1

	configurationNoChangedMessage           = "the configuration file has not been modified, skip reconfigure"
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

func checkConfigLabels(object client.Object, requiredLabs []string) bool {
	labels := object.GetLabels()
	if len(labels) == 0 {
		return false
	}

	for _, label := range requiredLabs {
		if _, ok := labels[label]; !ok {
			return false
		}
	}

	// reconfigure ConfigMap for db instance
	if ins, ok := labels[constant.CMConfigurationTypeLabelKey]; !ok || ins != constant.ConfigInstanceType {
		return false
	}

	return checkEnableCfgUpgrade(object)
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
	rctx := newReconcileContext(reqCtx, &render.ResourceCtx{
		Context:       reqCtx.Ctx,
		Client:        r.Client,
		Namespace:     configMap.Namespace,
		ClusterName:   configMap.Labels[constant.AppInstanceLabelKey],
		ComponentName: configMap.Labels[constant.KBAppComponentLabelKey],
	}, configMap, nil)
	if err := rctx.objects(); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}

	// Assumption: It is required that the cluster must have a component.
	if rctx.ClusterComObj == nil {
		reqCtx.Log.Info("not found component.")
		return intctrlutil.Reconciled()
	}

	configPatch, forceRestart, err := createConfigPatch(configMap, rctx.configRender, rctx.parametersDefs)
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

	tasks, err := r.buildReconfigureTasks(configSpec, rctx, configPatch, forceRestart)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}
	return r.performUpgrade(rctx, tasks)
}

func (r *ReconfigureReconciler) buildReconfigureTasks(templateSpec *appsv1.ComponentFileTemplate,
	rctx *reconcileContext, patch *core.ConfigPatchInfo, forceRestart bool) ([]reconfigure.Task, error) {

	// If the patch or ConfigRender is nil, return a single restart task.
	if patch == nil || rctx.configRender == nil {
		return []reconfigure.Task{r.buildRestartTask(templateSpec, rctx)}, nil
	}

	// needReloadAction determines if a reload action is needed based on the ParametersDefinition and ReloadPolicy.
	needReloadAction := func(pd *parametersv1alpha1.ParametersDefinition, policy parametersv1alpha1.ReloadPolicy) bool {
		return !forceRestart || (policy == parametersv1alpha1.SyncDynamicReloadPolicy && parameters.NeedDynamicReloadAction(&pd.Spec))
	}

	var tasks []reconfigure.Task
	for key, jsonPatch := range patch.UpdateConfig {
		pd, ok := rctx.parametersDefs[key]
		// If the ParametersDefinition or its ReloadAction is nil, continue to the next iteration.
		if !ok || pd.Spec.ReloadAction == nil {
			continue
		}
		configFormat := parameters.GetComponentConfigDescription(&rctx.configRender.Spec, key)
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
		return []reconfigure.Task{r.buildRestartTask(templateSpec, rctx)}, nil
	}

	return tasks, nil
}

func (r *ReconfigureReconciler) buildReloadTask(policy parametersv1alpha1.ReloadPolicy,
	templateSpec *appsv1.ComponentFileTemplate, rctx *reconcileContext, pd *parametersv1alpha1.ParametersDefinition,
	configDescription *parametersv1alpha1.ComponentConfigDescription, patch *core.ConfigPatchInfo) reconfigure.Task {
	reCtx := reconfigure.Context{
		RequestCtx:        rctx.RequestCtx,
		Client:            rctx.Client,
		ConfigTemplate:    *templateSpec,
		ConfigHash:        computeTargetConfigHash(&rctx.RequestCtx, rctx.configMap.Data),
		Cluster:           rctx.ClusterObj,
		ClusterComponent:  rctx.ClusterComObj,
		ITS:               rctx.its,
		ConfigDescription: configDescription,
		ParametersDef:     &pd.Spec,
		Patch:             patch,
	}
	return reconfigure.Task{Policy: policy, Ctx: reCtx}
}

func (r *ReconfigureReconciler) buildRestartTask(configTemplate *appsv1.ComponentFileTemplate, rctx *reconcileContext) reconfigure.Task {
	return reconfigure.Task{
		Policy: parametersv1alpha1.RestartPolicy,
		Ctx: reconfigure.Context{
			RequestCtx:       rctx.RequestCtx,
			Client:           rctx.Client,
			ConfigTemplate:   *configTemplate,
			ConfigHash:       computeTargetConfigHash(&rctx.RequestCtx, rctx.configMap.Data),
			Cluster:          rctx.ClusterObj,
			ClusterComponent: rctx.ClusterComObj,
			ITS:              rctx.its,
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
	case r.enableSyncTrigger(pd.ReloadAction): // sync config-manager exec hot update
		policy = parametersv1alpha1.SyncDynamicReloadPolicy
	default: // config-manager auto trigger to hot update
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	}
	return policy, nil
}

func (r *ReconfigureReconciler) enableSyncTrigger(reloadAction *parametersv1alpha1.ReloadAction) bool {
	if reloadAction == nil {
		return false
	}
	if reloadAction.ShellTrigger != nil {
		return !core.IsWatchModuleForShellTrigger(reloadAction.ShellTrigger)
	}
	return false
}

func (r *ReconfigureReconciler) updateConfigCMStatus(reqCtx intctrlutil.RequestCtx, cfg *corev1.ConfigMap, reconfigureType string, result *parameters.Result) (ctrl.Result, error) {
	configData, err := json.Marshal(cfg.Data)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cfg, r.Recorder, err, reqCtx.Log)
	}

	if ok, err := updateAppliedConfigs(r.Client, reqCtx, cfg, configData, reconfigureType, result); err != nil || !ok {
		return intctrlutil.RequeueAfter(configReconcileInterval, reqCtx.Log, "failed to patch status and retry...", "error", err)
	}

	return intctrlutil.Reconciled()
}

func (r *ReconfigureReconciler) performUpgrade(rctx *reconcileContext, tasks []reconfigure.Task) (ctrl.Result, error) {
	var (
		err    error
		policy string
		status reconfigure.Status
	)
	for _, task := range tasks {
		policy = string(task.Policy)
		status, err = task.Reconfigure()
		if err != nil || status.Status != reconfigure.StatusNone {
			break
		}
	}
	// submit changes to the cluster
	if err1 := r.submit(rctx); err1 != nil {
		return intctrlutil.RequeueAfter(configReconcileInterval, rctx.Log, "failed to submit changes to the cluster", "error", err1)
	}
	if err != nil || status.Status != reconfigure.StatusNone {
		return r.status(rctx, policy, status, err)
	}
	return r.succeed(rctx, policy, status)
}

func (r *ReconfigureReconciler) submit(rctx *reconcileContext) error {
	if rctx.ClusterObj == nil || rctx.ClusterObjCopy == nil {
		return fmt.Errorf("the cluster object is nil")
	}
	if reflect.DeepEqual(rctx.ClusterObj.Spec, rctx.ClusterObjCopy.Spec) {
		return nil
	}
	return rctx.Client.Update(rctx.RequestCtx.Ctx, rctx.ClusterObj)
}

func (r *ReconfigureReconciler) status(rctx *reconcileContext, policy string, status reconfigure.Status, err error) (ctrl.Result, error) {
	updatePhase := func(phase parametersv1alpha1.ParameterPhase, options ...options) (ctrl.Result, error) {
		return updateConfigPhaseWithResult(rctx.Client, rctx.RequestCtx, rctx.configMap, reconciled(status, policy, phase, options...))
	}

	switch status.Status {
	case reconfigure.StatusFailedAndRetry:
		return updatePhase(parametersv1alpha1.CFailedPhase, withFailed(err, true))
	case reconfigure.StatusRetry:
		return updatePhase(parametersv1alpha1.CUpgradingPhase)
	case reconfigure.StatusFailed:
		return updatePhase(parametersv1alpha1.CFailedAndPausePhase, withFailed(err, false))
	case reconfigure.StatusNone:
		return r.succeed(rctx, policy, status)
	default:
		return updatePhase(parametersv1alpha1.CFailedAndPausePhase, withFailed(core.MakeError("unknown status"), false))
	}
}

func (r *ReconfigureReconciler) succeed(rctx *reconcileContext, policy string, status reconfigure.Status) (ctrl.Result, error) {
	rctx.Recorder.Eventf(rctx.configMap,
		corev1.EventTypeNormal,
		appsv1alpha1.ReasonReconfigureSucceed,
		"the reconfigure[%s] has been processed successfully",
		policy)
	result := reconciled(status, policy, parametersv1alpha1.CFinishedPhase)
	return r.updateConfigCMStatus(rctx.RequestCtx, rctx.configMap, policy, &result)
}

func computeTargetConfigHash(reqCtx *intctrlutil.RequestCtx, data map[string]string) *string {
	hash, err := cfgutil.ComputeHash(data)
	if err != nil {
		if reqCtx != nil {
			reqCtx.Log.Error(err, "failed to get configuration version!")
		}
		return nil
	}
	return &hash
}

func createConfigPatch(cfg *corev1.ConfigMap, configRender *parametersv1alpha1.ParamConfigRenderer, paramsDefs map[string]*parametersv1alpha1.ParametersDefinition) (*core.ConfigPatchInfo, bool, error) {
	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil, true, nil
	}
	lastConfig, err := getLastVersionConfig(cfg)
	if err != nil {
		return nil, false, core.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(cfg))
	}

	patch, restart, err := core.CreateConfigPatch(lastConfig, cfg.Data, configRender.Spec, true)
	if err != nil {
		return nil, false, err
	}
	if !restart {
		restart = cfgcm.NeedRestart(paramsDefs, patch)
	}
	return patch, restart, nil
}

func getLastVersionConfig(cm *corev1.ConfigMap) (map[string]string, error) {
	data := make(map[string]string, 0)
	cfgContent, ok := cm.GetAnnotations()[constant.LastAppliedConfigAnnotationKey]
	if !ok {
		return data, nil
	}

	if err := json.Unmarshal([]byte(cfgContent), &data); err != nil {
		return nil, err
	}

	return data, nil
}

type options = func(*parameters.Result)

func reconciled(status reconfigure.Status, policy string, phase parametersv1alpha1.ParameterPhase, options ...options) parameters.Result {
	result := parameters.Result{
		Policy:        policy,
		Phase:         phase,
		ExecResult:    status.Status,
		ExpectedCount: status.ExpectedCount,
		SucceedCount:  status.SucceedCount,
		Retry:         true,
	}
	for _, option := range options {
		option(&result)
	}
	return result
}

func unReconciled(phase parametersv1alpha1.ParameterPhase, revision string, message string) parameters.Result {
	return parameters.Result{
		Phase:         phase,
		Revision:      revision,
		Message:       message,
		SucceedCount:  core.NotStarted,
		ExpectedCount: core.Unconfirmed,
		Failed:        false,
		Retry:         false,
	}
}

func isReconciledResult(result parameters.Result) bool {
	return result.ExecResult != "" && result.Policy != ""
}

func withFailed(err error, retry bool) options {
	return func(result *parameters.Result) {
		result.Retry = retry
		if err != nil {
			result.Failed = true
			result.Message = err.Error()
		}
	}
}

func checkEnableCfgUpgrade(object client.Object) bool {
	// check user's upgrade switch
	// config.kubeblocks.io/disable-reconfigure = "false"
	annotations := object.GetAnnotations()
	value, ok := annotations[constant.DisableUpgradeInsConfigurationAnnotationKey]
	if !ok {
		return true
	}

	enable, err := strconv.ParseBool(value)
	if err == nil && enable {
		return false
	}

	return true
}

func updateConfigPhase(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, phase parametersv1alpha1.ParameterPhase, message string) (ctrl.Result, error) {
	return updateConfigPhaseWithResult(cli, ctx, config, unReconciled(phase, "", message))
}

func updateConfigPhaseWithResult(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, result parameters.Result) (ctrl.Result, error) {
	revision, ok := config.ObjectMeta.Annotations[constant.ConfigurationRevision]
	if !ok || revision == "" {
		return intctrlutil.Reconciled()
	}

	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	if result.Failed && !result.Retry {
		ctx.Log.Info(fmt.Sprintf("failed to reconcile and disable retry for configmap[%+v]", client.ObjectKeyFromObject(config)))
		config.ObjectMeta.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey] = strconv.FormatBool(true)
	}

	gcConfigRevision(config)
	if _, ok := config.ObjectMeta.Annotations[core.GenerateRevisionPhaseKey(revision)]; !ok || isReconciledResult(result) {
		result.Revision = revision
		b, _ := json.Marshal(result)
		config.ObjectMeta.Annotations[core.GenerateRevisionPhaseKey(revision)] = string(b)
	}

	if err := cli.Patch(ctx.Ctx, config, patch); err != nil {
		return intctrlutil.RequeueWithError(err, ctx.Log, "")
	}
	if result.Retry {
		return intctrlutil.RequeueAfter(configReconcileInterval, ctx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func checkAndApplyConfigsChanged(client client.Client, ctx intctrlutil.RequestCtx, cm *corev1.ConfigMap) (bool, error) {
	annotations := cm.GetAnnotations()

	configData, err := json.Marshal(cm.Data)
	if err != nil {
		return false, err
	}

	lastConfig, ok := annotations[constant.LastAppliedConfigAnnotationKey]
	if !ok {
		return updateAppliedConfigs(client, ctx, cm, configData, core.ReconfigureCreatedPhase, nil)
	}

	return lastConfig == string(configData), nil
}

func updateAppliedConfigs(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, configData []byte, reconfigurePhase string, result *parameters.Result) (bool, error) {

	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	gcConfigRevision(config)
	if revision, ok := config.ObjectMeta.Annotations[constant.ConfigurationRevision]; ok && revision != "" {
		if result == nil {
			result = ptr.To(unReconciled(parametersv1alpha1.CFinishedPhase, "", fmt.Sprintf("phase: %s", reconfigurePhase)))
		}
		result.Revision = revision
		b, _ := json.Marshal(result)
		config.ObjectMeta.Annotations[core.GenerateRevisionPhaseKey(revision)] = string(b)
	}
	config.ObjectMeta.Annotations[constant.LastAppliedConfigAnnotationKey] = string(configData)
	hash, err := cfgutil.ComputeHash(config.Data)
	if err != nil {
		return false, err
	}
	config.ObjectMeta.Labels[constant.CMInsConfigurationHashLabelKey] = hash

	newReconfigurePhase := config.ObjectMeta.Labels[constant.CMInsLastReconfigurePhaseKey]
	if newReconfigurePhase == "" {
		newReconfigurePhase = core.ReconfigureCreatedPhase
	}
	if core.ReconfigureNoChangeType != reconfigurePhase && !core.IsParametersUpdateFromManager(config) {
		newReconfigurePhase = reconfigurePhase
	}
	config.ObjectMeta.Labels[constant.CMInsLastReconfigurePhaseKey] = newReconfigurePhase

	// delete reconfigure-policy
	delete(config.ObjectMeta.Annotations, constant.UpgradePolicyAnnotationKey)
	if err := cli.Patch(ctx.Ctx, config, patch); err != nil {
		return false, err
	}

	return true, nil
}
