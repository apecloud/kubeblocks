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
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
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
func (r *ComponentParameterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&parametersv1alpha1.ComponentParameter{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers) / 4,
		}).
		Owns(&corev1.ConfigMap{}).
		Watches(&appsv1.Component{}, handler.EnqueueRequestsFromMapFunc(r.enqueueByComponent)).
		Complete(r)
}

func (r *ComponentParameterReconciler) enqueueByComponent(_ context.Context, object client.Object) []reconcile.Request {
	comp, ok := object.(*appsv1.Component)
	if !ok {
		return nil
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: comp.Namespace,
			Name:      comp.Name,
		},
	}}
}

func (r *ComponentParameterReconciler) reconcile(reqCtx intctrlutil.RequestCtx, compParam *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	fetcherTask, err := prepareReconcileTask(reqCtx, r.Client, compParam)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, errors.Wrap(err, "failed to get related object").Error())
	}
	if model.IsObjectDeleting(fetcherTask.ComponentObj) {
		reqCtx.Log.Info("cluster is deleting, skip reconcile")
		return intctrlutil.Reconciled()
	}
	if fetcherTask.ClusterComObj == nil || fetcherTask.ComponentObj == nil {
		return r.failWithInvalidComponent(reqCtx, compParam)
	}

	// Reconcile the internal execution model in stages:
	// 1. ensure the config item skeleton exists and is aligned with the current component definition;
	// 2. project spec.init/spec.desired into the skeletonized config item details;
	// 3. execute the downstream render/reconfigure flow only after spec has stabilized.
	//
	// Each spec mutation returns early and relies on the next reconcile to continue with a fresh view.
	skeletonUpdated, err := r.reconcileConfigItemDetails(reqCtx, compParam, fetcherTask)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, errors.Wrap(err, "failed to reconcile config item details").Error())
	}
	if skeletonUpdated {
		return intctrlutil.Reconciled()
	}

	specUpdated, handled, err := r.reconcileParameterValues(reqCtx, compParam, fetcherTask)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, errors.Wrap(err, "failed to reconcile parameter values").Error())
	}
	if handled || specUpdated {
		return intctrlutil.Reconciled()
	}

	tasks := generateReconcileTasks(reqCtx, compParam, fetcherTask.ComponentObj.Generation)
	if len(tasks) == 0 {
		reqCtx.Log.Info("nothing to reconcile")
		return intctrlutil.Reconciled()
	}
	taskCtx, err := newTaskContext(reqCtx.Ctx, r.Client, compParam, fetcherTask)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, errors.Wrap(err, "failed to create task context").Error())
	}
	if err := r.runTasks(taskCtx, tasks, fetcherTask); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
			errors.Wrap(err, "failed to run parameters reconcile task").Error())
	}
	return intctrlutil.Reconciled()
}

func (r *ComponentParameterReconciler) reconcileConfigItemDetails(reqCtx intctrlutil.RequestCtx, compParam *parametersv1alpha1.ComponentParameter, fetchTask *Task) (bool, error) {
	return reconcileConfigItemDetailsIntoSpec(reqCtx.Ctx, r.Client, compParam, fetchTask)
}

func (r *ComponentParameterReconciler) reconcileParameterValues(reqCtx intctrlutil.RequestCtx, compParam *parametersv1alpha1.ComponentParameter, fetchTask *Task) (bool, bool, error) {
	if compParam.Spec.Init == nil && compParam.Spec.Desired == nil {
		return false, false, nil
	}
	patched, err := reconcileParameterValuesIntoSpec(reqCtx.Ctx, r.Client, compParam, fetchTask)
	if err == nil {
		return patched, patched, nil
	}
	if statusErr := r.failWithParameterValues(reqCtx, compParam, err); statusErr != nil {
		return false, false, statusErr
	}
	return false, true, nil
}

func (r *ComponentParameterReconciler) failWithInvalidComponent(reqCtx intctrlutil.RequestCtx, compParam *parametersv1alpha1.ComponentParameter) (ctrl.Result, error) {
	msg := fmt.Sprintf("not found cluster component: [%s]", compParam.Spec.ComponentName)

	reqCtx.Log.Error(fmt.Errorf("%s", msg), "")
	patch := client.MergeFrom(compParam.DeepCopy())
	compParam.Status.Message = msg
	if err := r.Client.Status().Patch(reqCtx.Ctx, compParam, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
			errors.Wrap(err, "failed to update componentParameter status").Error())
	}
	return intctrlutil.Reconciled()
}

func (r *ComponentParameterReconciler) failWithParameterValues(reqCtx intctrlutil.RequestCtx, compParam *parametersv1alpha1.ComponentParameter, cause error) error {
	msg := cause.Error()
	if compParam.Status.ObservedGeneration == compParam.Generation &&
		compParam.Status.Phase == parametersv1alpha1.CMergeFailedPhase &&
		compParam.Status.Message == msg {
		return nil
	}
	patch := client.MergeFrom(compParam.DeepCopy())
	compParam.Status.ObservedGeneration = compParam.Generation
	compParam.Status.Phase = parametersv1alpha1.CMergeFailedPhase
	compParam.Status.Message = msg
	return r.Client.Status().Patch(reqCtx.Ctx, compParam, patch)
}

func (r *ComponentParameterReconciler) runTasks(taskCtx *taskContext, tasks []Task, resource *Task) error {
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

	r.updateCompParamStatus(&compParameter.Status, errs, compParameter.Generation)
	if err := r.Client.Status().Patch(taskCtx.ctx, compParameter, patch); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	return utilerrors.NewAggregate(errs)
}

func (r *ComponentParameterReconciler) updateCompParamStatus(status *parametersv1alpha1.ComponentParameterStatus, errs []error, generation int64) {
	aggregatePhase := func(ss []parametersv1alpha1.ConfigTemplateItemDetailStatus) parametersv1alpha1.ParameterPhase {
		var phase = parametersv1alpha1.CFinishedPhase
		for _, s := range ss {
			switch {
			case parameters.IsFailedPhase(s.Phase):
				return s.Phase
			case !parameters.IsParameterFinished(s.Phase):
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

func (r *ComponentParameterReconciler) deletionHandler(reqCtx intctrlutil.RequestCtx, compParam *parametersv1alpha1.ComponentParameter) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		cms := &corev1.ConfigMapList{}
		listOpts := []client.ListOption{
			client.InNamespace(compParam.GetNamespace()),
			client.MatchingLabels(constant.GetCompLabels(compParam.Spec.ClusterName, compParam.Spec.ComponentName)),
		}
		if err := r.Client.List(reqCtx.Ctx, cms, listOpts...); err != nil {
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
