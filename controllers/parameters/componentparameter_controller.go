/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"
	"math"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ComponentParameterReconciler reconciles a ComponentParameter object
type ComponentParameterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parameters.kubeblocks.io,resources=componentparameters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentParameterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Recorder: r.Recorder,
		Log: log.FromContext(ctx).
			WithName("ComponentParameterReconciler").
			WithValues("Namespace", req.Namespace, "ComponentParameter", req.Name),
	}

	componentParam := &parametersv1alpha1.ComponentParameter{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, componentParam); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, componentParam, constant.ConfigFinalizerName, r.deletionHandler(reqCtx, componentParam))
	if res != nil {
		return *res, err
	}
	return r.reconcile(reqCtx, componentParam)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentParameterReconciler) SetupWithManager(mgr ctrl.Manager, multiClusterMgr multicluster.Manager) error {
	builder := intctrlutil.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ComponentParameter{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: int(math.Ceil(viper.GetFloat64(constant.CfgKBReconcileWorkers) / 2)),
		}).
		Owns(&corev1.ConfigMap{})
	if multiClusterMgr != nil {
		multiClusterMgr.Own(builder, &corev1.ConfigMap{}, &parametersv1alpha1.ComponentParameter{})
	}
	return builder.Complete(r)
}

func (r *ComponentParameterReconciler) reconcile(reqCtx intctrlutil.RequestCtx, componentParameter *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	tasks := generateReconcileTasks(reqCtx, componentParameter)
	if len(tasks) == 0 {
		return intctrlutil.Reconciled()
	}

	fetcherTask, err := prepareReconcileTask(reqCtx, r.Client, componentParameter)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to get related object.")
	}
	if !fetcherTask.ClusterObj.GetDeletionTimestamp().IsZero() {
		reqCtx.Log.Info("cluster is deleting, skip reconcile")
		return intctrlutil.Reconciled()
	}
	if fetcherTask.ClusterComObj == nil || fetcherTask.ComponentObj == nil {
		return r.failWithInvalidComponent(componentParameter, reqCtx)
	}

	taskCtx, err := NewTaskContext(reqCtx.Ctx, r.Client, componentParameter, fetcherTask)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to create task context.")
	}
	if err := r.runTasks(taskCtx, tasks, fetcherTask); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to run configuration reconcile task.")
	}
	return intctrlutil.Reconciled()
}

func (r *ComponentParameterReconciler) failWithInvalidComponent(componentParam *parametersv1alpha1.ComponentParameter, reqCtx intctrlutil.RequestCtx) (ctrl.Result, error) {
	msg := fmt.Sprintf("not found cluster component: [%s]", componentParam.Spec.ComponentName)

	reqCtx.Log.Error(fmt.Errorf("%s", msg), "")
	patch := client.MergeFrom(componentParam.DeepCopy())
	componentParam.Status.Message = msg
	if err := r.Client.Status().Patch(reqCtx.Ctx, componentParam, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update configuration status.")
	}
	return intctrlutil.Reconciled()
}

func (r *ComponentParameterReconciler) runTasks(taskCtx *TaskContext, tasks []Task, resource *Task) error {
	var (
		errs          []error
		compParameter = taskCtx.componentParameter
	)

	patch := client.MergeFrom(compParameter.DeepCopy())
	revision := strconv.FormatInt(compParameter.GetGeneration(), 10)
	for _, task := range tasks {
		if err := task.Do(resource, taskCtx, revision); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	updateCompParamStatus(&compParameter.Status, errs, compParameter.Generation)
	if err := r.Client.Status().Patch(taskCtx.ctx, compParameter, patch); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	return utilerrors.NewAggregate(errs)
}

func updateCompParamStatus(status *parametersv1alpha1.ComponentParameterStatus, errs []error, generation int64) {
	aggregatePhase := func(ss []parametersv1alpha1.ConfigTemplateItemDetailStatus) parametersv1alpha1.ParameterPhase {
		var phase = parametersv1alpha1.CFinishedPhase
		for _, s := range ss {
			switch {
			case intctrlutil.IsFailedPhase(s.Phase):
				return s.Phase
			case !intctrlutil.IsParameterFinished(s.Phase):
				phase = parametersv1alpha1.CRunningPhase
			}
		}
		return phase
	}

	status.ObservedGeneration = generation
	status.Message = ""
	status.Phase = aggregatePhase(status.ConfigurationItemStatus)
	if len(errs) > 0 {
		status.Message = utilerrors.NewAggregate(errs).Error()
	}
}

func (r *ComponentParameterReconciler) deletionHandler(reqCtx intctrlutil.RequestCtx, componentParameter *parametersv1alpha1.ComponentParameter) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		var cms = &corev1.ConfigMapList{}
		matchLabels := client.MatchingLabels(constant.GetCompLabels(componentParameter.Spec.ClusterName, componentParameter.Spec.ComponentName))
		if err := r.Client.List(reqCtx.Ctx, cms, client.InNamespace(componentParameter.Name), matchLabels); err != nil {
			return &reconcile.Result{}, err
		}
		if err := removeConfigRelatedFinalizer(reqCtx.Ctx, r.Client, cms.Items); err != nil {
			return &reconcile.Result{}, err
		}
		return nil, nil
	}
}

func removeConfigRelatedFinalizer(ctx context.Context, cli client.Client, objs []corev1.ConfigMap) error {
	for _, obj := range objs {
		if !controllerutil.ContainsFinalizer(&obj, constant.ConfigFinalizerName) {
			continue
		}
		patch := client.MergeFrom(obj.DeepCopy())
		controllerutil.RemoveFinalizer(&obj, constant.ConfigFinalizerName)
		if err := cli.Patch(ctx, &obj, patch); err != nil {
			return err
		}
	}
	return nil
}
