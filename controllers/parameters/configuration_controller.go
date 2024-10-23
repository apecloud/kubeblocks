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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const reconcileInterval = time.Second * 2

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=configurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Configuration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithName("ConfigurationReconcile").WithValues("configuration", req.NamespacedName),
		Recorder: r.Recorder,
	}

	config := &appsv1alpha1.Configuration{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, config); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "cannot find configuration")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, config, constant.ConfigFinalizerName, nil)
	if res != nil {
		return *res, err
	}

	tasks := make([]Task, 0, len(config.Spec.ConfigItemDetails))
	for _, item := range config.Spec.ConfigItemDetails {
		if status := fromItemStatus(reqCtx, &config.Status, item); status != nil {
			tasks = append(tasks, NewTask(item, status))
		}
	}
	if len(tasks) == 0 {
		return intctrlutil.Reconciled()
	}

	fetcherTask := &Task{}
	err = fetcherTask.Init(&configctrl.ResourceCtx{
		Context:       ctx,
		Client:        r.Client,
		Namespace:     config.Namespace,
		ClusterName:   config.Spec.ClusterRef,
		ComponentName: config.Spec.ComponentName,
	}, fetcherTask).Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		Complete()
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to get related object.")
	}

	if !fetcherTask.ClusterObj.GetDeletionTimestamp().IsZero() {
		reqCtx.Log.Info("cluster is deleting, skip reconcile")
		return intctrlutil.Reconciled()
	}
	if fetcherTask.ClusterComObj == nil || fetcherTask.ComponentObj == nil {
		return r.failWithInvalidComponent(config, reqCtx)
	}
	if err := r.runTasks(TaskContext{config, ctx, fetcherTask}, tasks); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to run configuration reconcile task.")
	}
	if !isAllReady(config) {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *ConfigurationReconciler) failWithInvalidComponent(configuration *appsv1alpha1.Configuration, reqCtx intctrlutil.RequestCtx) (ctrl.Result, error) {
	msg := fmt.Sprintf("not found cluster component or cluster definition component: [%s]", configuration.Spec.ComponentName)
	reqCtx.Log.Error(fmt.Errorf("%s", msg), "")
	patch := client.MergeFrom(configuration.DeepCopy())
	configuration.Status.Message = msg
	if err := r.Client.Status().Patch(reqCtx.Ctx, configuration, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to update configuration status.")
	}
	return intctrlutil.Reconciled()
}

func isAllReady(configuration *appsv1alpha1.Configuration) bool {
	for _, item := range configuration.Spec.ConfigItemDetails {
		itemStatus := configuration.Status.GetItemStatus(item.Name)
		if itemStatus != nil && !isFinishStatus(itemStatus.Phase) {
			return false
		}
	}
	return true
}

func (r *ConfigurationReconciler) runTasks(taskCtx TaskContext, tasks []Task) (err error) {
	var (
		errs            []error
		synthesizedComp *component.SynthesizedComponent
		configuration   = taskCtx.configuration
	)

	// build synthesized component for the component
	synthesizedComp, err = component.BuildSynthesizedComponent(taskCtx.ctx, r.Client,
		taskCtx.fetcher.ComponentDefObj, taskCtx.fetcher.ComponentObj, taskCtx.fetcher.ClusterObj)
	if err == nil {
		err = buildTemplateVars(taskCtx.ctx, r.Client, taskCtx.fetcher.ComponentDefObj, synthesizedComp)
	}
	if err != nil {
		return err
	}

	// TODO manager multiple version
	patch := client.MergeFrom(configuration.DeepCopy())
	revision := strconv.FormatInt(configuration.GetGeneration(), 10)
	for _, task := range tasks {
		if err := task.Do(taskCtx.fetcher, synthesizedComp, revision); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	configuration.Status.Message = ""
	if len(errs) > 0 {
		configuration.Status.Message = utilerrors.NewAggregate(errs).Error()
	}
	if err := r.Client.Status().Patch(taskCtx.ctx, configuration, patch); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	return utilerrors.NewAggregate(errs)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager, multiClusterMgr multicluster.Manager) error {
	b := intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&appsv1alpha1.Configuration{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: int(math.Ceil(viper.GetFloat64(constant.CfgKBReconcileWorkers) / 2)),
		}).
		Owns(&corev1.ConfigMap{})

	if multiClusterMgr != nil {
		multiClusterMgr.Own(b, &corev1.ConfigMap{}, &appsv1alpha1.Configuration{})
	}

	return b.Complete(r)
}

func fromItemStatus(ctx intctrlutil.RequestCtx, status *appsv1alpha1.ConfigurationStatus, item appsv1alpha1.ConfigurationItemDetail) *appsv1alpha1.ConfigurationItemDetailStatus {
	if item.ConfigSpec == nil {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration is creating and pass: %s", item.Name))
		return nil
	}
	itemStatus := status.GetItemStatus(item.Name)
	if itemStatus == nil || itemStatus.Phase == "" {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration cr is creating and pass: %v", item))
		return nil
	}
	if !isReconcileStatus(itemStatus.Phase) {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration cr is creating or deleting and pass: %v", itemStatus))
		return nil
	}
	return itemStatus
}

func isReconcileStatus(phase appsv1alpha1.ConfigurationPhase) bool {
	return phase != "" &&
		phase != appsv1alpha1.CCreatingPhase &&
		phase != appsv1alpha1.CDeletingPhase
}

func isFinishStatus(phase appsv1alpha1.ConfigurationPhase) bool {
	return phase == appsv1alpha1.CFinishedPhase || phase == appsv1alpha1.CFailedAndPausePhase
}

func buildTemplateVars(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) error {
	if compDef != nil && len(compDef.Spec.Vars) > 0 {
		templateVars, _, err := component.ResolveTemplateNEnvVars(ctx, cli, synthesizedComp, compDef.Spec.Vars)
		if err != nil {
			return err
		}
		synthesizedComp.TemplateVars = templateVars
	}
	return nil
}
