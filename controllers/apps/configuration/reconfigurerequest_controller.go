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

package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReconfigureRequestReconciler reconciles a ReconfigureRequest object
type ReconfigureRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	ConfigReconcileInterval = time.Second * 1
)

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
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "cannot find configmap")
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

	tpl := &appsv1alpha1.ConfigConstraint{}
	cfgConstraintsName, ok := config.Labels[constant.CMConfigurationConstraintsNameLabelKey]
	if !ok || len(cfgConstraintsName) == 0 {
		reqCtx.Log.V(1).Info("configuration without ConfigConstraints, does not support reconfiguring.")
	} else {
		if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
			Namespace: config.Namespace,
			Name:      config.Labels[constant.CMConfigurationConstraintsNameLabelKey],
		}, tpl); err != nil {
			return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
		}
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

	configPatch, forceRestart, err := createConfigPatch(config, tpl.Spec.FormatterConfig, keySelector)
	if err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// No parameters updated
	if configPatch != nil && !configPatch.IsModify {
		reqCtx.Recorder.Eventf(config, corev1.EventTypeNormal, appsv1alpha1.ReasonReconfigureRunning,
			"nothing changed, skip reconfigure")
		return r.updateConfigCMStatus(reqCtx, config, core.ReconfigureNoChangeType)
	}

	if configPatch != nil {
		reqCtx.Log.V(1).Info(fmt.Sprintf(
			"reconfigure params: \n\tadd: %s\n\tdelete: %s\n\tupdate: %s",
			configPatch.AddConfig,
			configPatch.DeleteConfig,
			configPatch.UpdateConfig))
	}

	reconcileContext := newConfigReconcileContext(reqCtx.Ctx, r.Client, config, tpl, componentName, configSpecName, componentLabels)
	if err := reconcileContext.GetRelatedObjects(); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(config, r.Recorder, err, reqCtx.Log)
	}

	// Assumption: It is required that the cluster must have a component.
	if reconcileContext.ClusterComponent == nil {
		reqCtx.Log.Info("not found component.")
		return intctrlutil.Reconciled()
	}
	if reconcileContext.ConfigSpec == nil {
		reqCtx.Log.Info(fmt.Sprintf("not found configSpec[%s] in the component[%s].", configSpecName, componentName))
		reqCtx.Recorder.Eventf(config, corev1.EventTypeWarning, appsv1alpha1.ReasonReconfigureFailed,
			"related component does not have any configSpecs, skip reconfigure")
		return intctrlutil.Reconciled()
	}
	if len(reconcileContext.StatefulSets) == 0 && len(reconcileContext.Deployments) == 0 && len(reconcileContext.RSMList) == 0 {
		reqCtx.Recorder.Eventf(config,
			corev1.EventTypeWarning, appsv1alpha1.ReasonReconfigureFailed,
			"the configmap is not used by any container, skip reconfigure")
		return intctrlutil.Reconciled()
	}
	// TODO(free6om): configuration controller needs workload type to do the config reloading job.
	// it's a rather hacky way converting rsm to sts to make configuration controller works as usual.
	// should make configuration controller recognizing rsm.
	if len(reconcileContext.RSMList) > 0 {
		for _, rsm := range reconcileContext.RSMList {
			reconcileContext.StatefulSets = append(reconcileContext.StatefulSets, *components.ConvertRSMToSTS(&rsm))
		}
	}

	return r.performUpgrade(reconfigureParams{
		ConfigSpecName:           configSpecName,
		ConfigPatch:              configPatch,
		ConfigMap:                config,
		ConfigConstraint:         &tpl.Spec,
		Client:                   r.Client,
		Ctx:                      reqCtx,
		Cluster:                  reconcileContext.Cluster,
		ContainerNames:           reconcileContext.Containers,
		ComponentUnits:           reconcileContext.StatefulSets,
		DeploymentUnits:          reconcileContext.Deployments,
		Component:                reconcileContext.ClusterDefComponent,
		ClusterComponent:         reconcileContext.ClusterComponent,
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
	if err := r.handleConfigEvent(params, core.PolicyExecStatus{
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
		params.Ctx.Recorder.Eventf(params.ConfigMap,
			corev1.EventTypeNormal, appsv1alpha1.ReasonReconfigureSucceed,
			"the reconfigure[%s] request[%s] has been processed successfully",
			policy.GetPolicyName(), getOpsRequestID(params.ConfigMap))
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

func getOpsRequestID(cm *corev1.ConfigMap) string {
	if len(cm.Annotations) != 0 {
		return cm.Annotations[constant.LastAppliedOpsCRAnnotationKey]
	}
	return ""
}

func (r *ReconfigureRequestReconciler) handleConfigEvent(params reconfigureParams, status core.PolicyExecStatus, err error) error {
	var (
		cm             = params.ConfigMap
		lastOpsRequest = ""
	)

	if len(cm.Annotations) != 0 {
		lastOpsRequest = cm.Annotations[constant.LastAppliedOpsCRAnnotationKey]
	}

	eventContext := intctrlutil.ConfigEventContext{
		ConfigSpecName:   params.ConfigSpecName,
		Client:           params.Client,
		ReqCtx:           params.Ctx,
		Cluster:          params.Cluster,
		Component:        params.Component,
		ConfigPatch:      params.ConfigPatch,
		ConfigConstraint: params.ConfigConstraint,
		ConfigMap:        params.ConfigMap,
		ComponentUnits:   params.ComponentUnits,
		DeploymentUnits:  params.DeploymentUnits,
		PolicyStatus:     status,
	}

	for _, handler := range intctrlutil.ConfigEventHandlerMap {
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
