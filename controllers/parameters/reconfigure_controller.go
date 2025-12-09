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
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	configReconcileInterval                 = time.Second * 1
	configurationNoChangedMessage           = "the configuration file has not been modified, skip reconfigure"
	configurationNotRelatedComponentMessage = "related component does not found any configSpecs, skip reconfigure"
)

var (
	reconfigureRequiredLabels = []string{
		constant.AppInstanceLabelKey,
		constant.KBAppComponentLabelKey,
		constant.CMConfigurationTemplateNameLabelKey,
		constant.CMConfigurationTypeLabelKey,
		constant.CMConfigurationSpecProviderLabelKey,
	}
)

// ReconfigureReconciler reconciles a ReconfigureRequest object
type ReconfigureReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ReconfigureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("configMap", req.NamespacedName),
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
		WithValues("cluster", config.Labels[constant.AppInstanceLabelKey]).
		WithValues("component", config.Labels[constant.KBAppComponentLabelKey])

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
		Name:      cfgcore.GenerateComponentConfigurationName(cm.Labels[constant.AppInstanceLabelKey], cm.Labels[constant.KBAppComponentLabelKey]),
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
	var (
		clusterName = configMap.Labels[constant.AppInstanceLabelKey]
		compName    = configMap.Labels[constant.KBAppComponentLabelKey]
	)
	rctx := newParameterReconcileContext(reqCtx,
		&render.ResourceCtx{
			Context:       reqCtx.Ctx,
			Client:        r.Client,
			Namespace:     configMap.Namespace,
			ClusterName:   clusterName,
			ComponentName: compName,
		},
		configMap,
		nil,
		map[string]string{
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: compName,
		})
	if err := rctx.GetRelatedObjects(); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}

	// Assumption: It is required that the cluster must have a component.
	if rctx.ClusterComObj == nil {
		reqCtx.Log.Info("not found component.")
		return intctrlutil.Reconciled()
	}

	configPatch, forceRestart, err := createConfigPatch(configMap, rctx.ConfigRender, rctx.ParametersDefs)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}

	// No parameters updated
	if configPatch != nil && !configPatch.IsModify {
		reqCtx.Recorder.Event(configMap, corev1.EventTypeNormal, appsv1alpha1.ReasonReconfigureRunning, "nothing changed, skip reconfigure")
		return r.updateConfigCMStatus(reqCtx, configMap, cfgcore.ReconfigureNoChangeType, nil)
	}

	if configPatch != nil {
		reqCtx.Log.V(1).Info(fmt.Sprintf(
			"reconfigure params: \n\tadd: %s\n\tdelete: %s\n\tupdate: %s",
			configPatch.AddConfig,
			configPatch.DeleteConfig,
			configPatch.UpdateConfig))
	}

	tasks, err := buildReconfigureTasks(configSpec, rctx, configPatch, forceRestart)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(configMap, r.Recorder, err, reqCtx.Log)
	}
	return r.performUpgrade(rctx, tasks)
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

func (r *ReconfigureReconciler) performUpgrade(rctx *ReconcileContext, tasks []reconfigureTask) (ctrl.Result, error) {
	var (
		err        error
		status     returnedStatus
		reloadType string
	)
	for _, task := range tasks {
		reloadType = string(task.policy)
		status, err = task.reconfigure()
		if err != nil || status.Status != ESNone {
			break
		}
	}
	// submit changes to the cluster
	if err1 := r.submit(rctx); err1 != nil {
		return intctrlutil.RequeueAfter(configReconcileInterval, rctx.Log, "failed to submit changes to the cluster", "error", err1)
	}
	if err != nil || status.Status != ESNone {
		return r.status(rctx, reloadType, status, err)
	}
	return r.succeed(rctx, reloadType, status)
}

func (r *ReconfigureReconciler) submit(rctx *ReconcileContext) error {
	if rctx.ClusterObj == nil || rctx.ClusterObjCopy == nil {
		return fmt.Errorf("the cluster object is nil")
	}
	if reflect.DeepEqual(rctx.ClusterObj.Spec, rctx.ClusterObjCopy.Spec) {
		return nil
	}
	return rctx.Client.Update(rctx.RequestCtx.Ctx, rctx.ClusterObj)
}

func (r *ReconfigureReconciler) status(rctx *ReconcileContext, reloadType string, status returnedStatus, err error) (ctrl.Result, error) {
	updatePhase := func(phase parametersv1alpha1.ParameterPhase, options ...options) (ctrl.Result, error) {
		return updateConfigPhaseWithResult(rctx.Client, rctx.RequestCtx, rctx.ConfigMap, reconciled(status, reloadType, phase, options...))
	}

	switch status.Status {
	case ESFailedAndRetry:
		return updatePhase(parametersv1alpha1.CFailedPhase, withFailed(err, true))
	case ESRetry:
		return updatePhase(parametersv1alpha1.CUpgradingPhase)
	case ESFailed:
		return updatePhase(parametersv1alpha1.CFailedAndPausePhase, withFailed(err, false))
	case ESNone:
		return r.succeed(rctx, reloadType, status)
	default:
		return updatePhase(parametersv1alpha1.CFailedAndPausePhase, withFailed(cfgcore.MakeError("unknown status"), false))
	}
}

func (r *ReconfigureReconciler) succeed(rctx *ReconcileContext, reloadType string, status returnedStatus) (ctrl.Result, error) {
	rctx.Recorder.Eventf(rctx.ConfigMap,
		corev1.EventTypeNormal,
		appsv1alpha1.ReasonReconfigureSucceed,
		"the reconfigure[%s] has been processed successfully",
		reloadType)

	result := reconciled(status, reloadType, parametersv1alpha1.CFinishedPhase)
	return r.updateConfigCMStatus(rctx.RequestCtx, rctx.ConfigMap, reloadType, &result)
}
