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

package extensions

import (
	"context"
	"runtime"

	ctrlerihandler "github.com/authzed/controller-idioms/handler"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// AddonReconciler reconciles a Addon object
type AddonReconciler struct {
	client.Client
	Scheme     *k8sruntime.Scheme
	Recorder   record.EventRecorder
	RestConfig *rest.Config
}

var _ record.EventRecorder = &AddonReconciler{}

func init() {
	viper.SetDefault(maxConcurrentReconcilesKey, runtime.NumCPU()*2)
}

// +kubebuilder:rbac:groups=extensions.kubeblocks.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions.kubeblocks.io,resources=addons/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=extensions.kubeblocks.io,resources=addons/finalizers,verbs=update

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=pods/log,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *AddonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("addon", req.NamespacedName),
		Recorder: r.Recorder,
	}

	buildStageCtx := func(next ...ctrlerihandler.Handler) stageCtx {
		return stageCtx{
			reqCtx:     &reqCtx,
			reconciler: r,
			next:       ctrlerihandler.Handlers(next).MustOne(),
		}
	}

	fetchNDeletionCheckStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&fetchNDeletionCheckStage{
			stageCtx: buildStageCtx(next...),
			deletionStage: deletionStage{
				stageCtx: buildStageCtx(ctrlerihandler.NoopHandler),
			},
		})
	}

	genIDProceedStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&genIDProceedCheckStage{stageCtx: buildStageCtx(next...)})
	}

	installableCheckStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&installableCheckStage{stageCtx: buildStageCtx(next...)})
	}

	autoInstallCheckStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&autoInstallCheckStage{stageCtx: buildStageCtx(next...)})
	}

	enabledAutoValuesStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&enabledWithDefaultValuesStage{stageCtx: buildStageCtx(next...)})
	}

	progressingStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&progressingHandler{stageCtx: buildStageCtx(next...)})
	}

	terminalStateStageBuilder := func(next ...ctrlerihandler.Handler) ctrlerihandler.Handler {
		return ctrlerihandler.NewTypeHandler(&terminalStateStage{stageCtx: buildStageCtx(next...)})
	}

	handlers := ctrlerihandler.Chain(
		fetchNDeletionCheckStageBuilder,
		genIDProceedStageBuilder,
		installableCheckStageBuilder,
		autoInstallCheckStageBuilder,
		enabledAutoValuesStageBuilder,
		progressingStageBuilder,
		terminalStateStageBuilder,
	).Handler("")

	handlers.Handle(ctx)
	res, ok := reqCtx.Ctx.Value(resultValueKey).(*ctrl.Result)
	if ok && res != nil {
		err, ok := reqCtx.Ctx.Value(errorValueKey).(error)
		if ok {
			return *res, err
		}
		return *res, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&extensionsv1alpha1.Addon{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, handler.EnqueueRequestsFromMapFunc(r.findAddonJobs)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurrentReconcilesKey),
		}).
		Complete(r)
}

func (r *AddonReconciler) findAddonJobs(job client.Object) []reconcile.Request {
	labels := job.GetLabels()
	if _, ok := labels[constant.AddonNameLabelKey]; !ok {
		return []reconcile.Request{}
	}
	if v, ok := labels[constant.AppManagedByLabelKey]; !ok || v != constant.AppName {
		return []reconcile.Request{}
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: job.GetNamespace(),
				Name:      job.GetName(),
			},
		},
	}
}

func (r *AddonReconciler) cleanupJobPods(reqCtx intctrlutil.RequestCtx) error {
	if err := r.DeleteAllOf(reqCtx.Ctx, &corev1.Pod{},
		client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS)),
		client.MatchingLabels{
			constant.AddonNameLabelKey:    reqCtx.Req.Name,
			constant.AppManagedByLabelKey: constant.AppName,
		},
	); err != nil {
		return err
	}
	return nil
}

func (r *AddonReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, addon *extensionsv1alpha1.Addon) (*ctrl.Result, error) {
	if addon.Annotations != nil && addon.Annotations[NoDeleteJobs] == "true" {
		return nil, nil
	}
	deleteJobIfExist := func(jobName string) error {
		key := client.ObjectKey{
			Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			Name:      jobName,
		}
		job := &batchv1.Job{}
		if err := r.Get(reqCtx.Ctx, key, job); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !job.DeletionTimestamp.IsZero() {
			return nil
		}
		if err := r.Delete(reqCtx.Ctx, job); err != nil {
			return client.IgnoreNotFound(err)
		}
		return nil
	}
	for _, j := range []string{getInstallJobName(addon), getUninstallJobName(addon)} {
		if err := deleteJobIfExist(j); err != nil {
			return nil, err
		}
	}
	if err := r.cleanupJobPods(reqCtx); err != nil {
		return nil, err
	}
	return nil, nil
}

// following provide r.Recorder wrapper for safe operation if r.Recorder is not provided

func (r *AddonReconciler) Event(object k8sruntime.Object, eventtype, reason, message string) {
	if r == nil || r.Recorder == nil {
		return
	}
	r.Recorder.Event(object, eventtype, reason, message)
}

// Eventf is just like Event, but with Sprintf for the message field.
func (r *AddonReconciler) Eventf(object k8sruntime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if r == nil || r.Recorder == nil {
		return
	}
	r.Recorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

// AnnotatedEventf is just like eventf, but with annotations attached
func (r *AddonReconciler) AnnotatedEventf(object k8sruntime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	if r == nil || r.Recorder == nil {
		return
	}
	r.Recorder.AnnotatedEventf(object, annotations, eventtype, reason, messageFmt, args...)
}
