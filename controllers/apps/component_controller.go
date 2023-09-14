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

package apps

import (
	"context"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// ComponentReconciler reconciles a Component object
type ComponentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("component", req.NamespacedName),
		Recorder: r.Recorder,
	}

	rctx.Log.V(1).Info("reconcile", "component", req.NamespacedName)

	requeueError := func(err error) (ctrl.Result, error) {
		if re, ok := err.(intctrlutil.RequeueError); ok {
			return intctrlutil.RequeueAfter(re.RequeueAfter(), rctx.Log, re.Reason())
		}
		return intctrlutil.RequeueWithError(err, rctx.Log, "")
	}

	planBuilder := NewClusterPlanBuilder(rctx, r.Client, req)
	if err := planBuilder.Init(); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	plan, errBuild := planBuilder.
		AddTransformer(
			// handle component deletion first
			&componentDeletionTransformer{},
			// update finalizer and component definition labels
			&componentAssureMetaTransformer{},
			// validate ref objects existence and availability
			&componentLoadResourcesTransformer{},
			// validate config
			&ValidateEnableLogsTransformer{},
			// create cluster connection credential secret object
			&ClusterCredentialTransformer{},
			// create all components objects
			&ComponentTransformer{Client: r.Client},
			// add our finalizer to all objects
			&OwnershipTransformer{},
			// make all workload objects depending on credential secret
			&SecretTransformer{},
			// update cluster status
			&ClusterStatusTransformer{},
		).
		Build()

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
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Component{}).
		Complete(r)
}

type componentTransformContext struct {
	ReqCtx  intctrlutil.RequestCtx
	Client  roclient.ReadonlyClient
	Comp    *appsv1alpha1.Component
	CompDef *appsv1alpha1.ComponentDefinition
}

var _ graph.TransformContext = &componentTransformContext{}

func (c *componentTransformContext) GetContext() context.Context {
	return c.ReqCtx.Ctx
}

func (c *componentTransformContext) GetClient() roclient.ReadonlyClient {
	return c.Client
}

func (c *componentTransformContext) GetRecorder() record.EventRecorder {
	return c.ReqCtx.Recorder
}

func (c *componentTransformContext) GetLogger() logr.Logger {
	return c.ReqCtx.Log
}
