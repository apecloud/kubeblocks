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

package component

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/finalizers,verbs=update

// owned workload API
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/finalizers,verbs=update

// owned K8s core API resources controller-gen RBAC marker
// full access on core API resources
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;update;patch

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=update

// read + update access
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// read only + watch access
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/status,verbs=get

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings/status,verbs=get

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("component", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "component", req.NamespacedName)

	planBuilder := newComponentPlanBuilder(reqCtx, r.Client)
	if err := planBuilder.Init(); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	requeueError := func(err error) (ctrl.Result, error) {
		if intctrlutil.IsRequeueError(err) {
			var re intctrlutil.RequeueError
			_ = errors.As(err, &re)
			return intctrlutil.RequeueAfter(re.RequeueAfter(), reqCtx.Log, re.Reason())
		}
		if apierrors.IsConflict(err) {
			return intctrlutil.Requeue(reqCtx.Log, err.Error())
		}
		c := planBuilder.(*componentPlanBuilder)
		appsutil.SendWarningEventWithError(r.Recorder, c.transCtx.Component, corev1.EventTypeWarning, err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	plan, errBuild := planBuilder.
		AddTransformer(
			// handle component pre-terminate
			&componentPreTerminateTransformer{},
			// handle component deletion
			&componentDeletionTransformer{},
			// handle finalizers and referenced definition labels
			&componentMetaTransformer{},
			// validate referenced componentDefinition objects, and build synthesized component
			&componentLoadResourcesTransformer{},
			// do validation for the spec & definition consistency
			&componentValidationTransformer{},
			// handle sidecar container
			&componentMonitorContainerTransformer{},
			// allocate ports for host-network component
			&componentHostNetworkTransformer{},
			// map for container ports to host ports
			&componentHostPortTransformer{},
			// handle component services
			&componentServiceTransformer{},
			// handle component system accounts
			&componentAccountTransformer{},
			// handle the TLS
			&componentTLSTransformer{},
			// resolve and build vars for template and Env
			&componentVarsTransformer{},
			// provision component system accounts, depend on vars
			&componentAccountProvisionTransformer{},
			// render config/script templates
			&componentFileTemplateTransformer{},
			// HACK: the legacy reload sidecar
			&componentReloadSidecarTransformer{Client: r.Client},
			// handle restore before workloads transform
			&componentRestoreTransformer{Client: r.Client},
			// handle RBAC for component workloads, it should be put before workload transformer
			&componentRBACTransformer{},
			// handle the component workload
			&componentWorkloadTransformer{Client: r.Client},
			// handle component postProvision lifecycle action
			&componentPostProvisionTransformer{},
			// update component status
			&componentStatusTransformer{Client: r.Client},
			// notify dependent components the possible spec changes
			&componentNotifierTransformer{},
		).Build()

	// Execute stage
	// errBuild not nil means build stage partial success or validation error
	// execute the plan first, delay error handling
	if errExec := plan.Execute(); errExec != nil {
		return requeueError(errExec)
	}
	if errBuild != nil {
		return requeueError(errBuild)
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	retryDurationMS := viper.GetInt(constant.CfgKeyCtrlrReconcileRetryDurationMS)
	if retryDurationMS != 0 {
		appsutil.RequeueDuration = time.Millisecond * time.Duration(retryDurationMS)
	}

	b := intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.Component{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		}).
		Owns(&workloads.InstanceSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&dpv1alpha1.Restore{}, handler.EnqueueRequestsFromMapFunc(r.filterComponentResources))

	if viper.GetBool(constant.EnableRBACManager) {
		b.Owns(&rbacv1.RoleBinding{}).
			Owns(&rbacv1.Role{}).
			Owns(&corev1.ServiceAccount{})
	}

	return b.Complete(r)
}

func (r *ComponentReconciler) filterComponentResources(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if v, ok := labels[constant.AppManagedByLabelKey]; !ok || v != constant.AppName {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.AppInstanceLabelKey]; !ok {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.KBAppComponentLabelKey]; !ok {
		return []reconcile.Request{}
	}
	fullCompName := constant.GenerateClusterComponentName(labels[constant.AppInstanceLabelKey], labels[constant.KBAppComponentLabelKey])
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      fullCompName,
			},
		},
	}
}
