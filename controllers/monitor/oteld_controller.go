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

package monitor

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitorv1alpha1 "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	monitorreconsile "github.com/apecloud/kubeblocks/controllers/monitor/reconcile"
	monitortypes "github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// OTeldReconciler reconciles a OTeld object
type OTeldReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   *monitortypes.Config

	// sub-controllers
	tasks []monitortypes.ReconcileTask
}

//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=collectordatasources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=collectordatasources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=collectordatasources/finalizers,verbs=update
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=logsexportersinks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=logsexportersinks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=logsexportersinks/finalizers,verbs=update
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=metricsexportersinks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=metricsexportersinks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=metricsexportersinks/finalizers,verbs=update
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=otelds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=otelds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.kubeblocks.io,resources=otelds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OTeld object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (o *OTeldReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithName("OTeldCollectorReconciler: " + req.NamespacedName.String()),
	}

	oteld := &monitorv1alpha1.OTeld{}
	if err := o.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, oteld); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	res, err := intctrlutil.HandleCRDeletion(reqCtx, o, oteld, constant.MonitorFinalizerName, func() (*ctrl.Result, error) {
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if err := o.runTasks(reqCtx, oteld); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (o *OTeldReconciler) runTasks(reqCtx intctrlutil.RequestCtx, oteld *monitorv1alpha1.OTeld) error {
	oteldCtx := monitortypes.ReconcileCtx{
		RequestCtx:  reqCtx,
		OteldCfgRef: &monitortypes.OteldCfgRef{},
	}

	for _, task := range o.tasks {
		if err := task.Do(oteldCtx); err != nil {
			return err
		}
	}
	return nil
}

func New(params monitortypes.OTeldParams) *OTeldReconciler {
	reconcile := OTeldReconciler{
		Client:   params.Client,
		Scheme:   params.Scheme,
		Recorder: params.Recorder,
		// Config:   config,

		// sub-controllers
		tasks: []monitortypes.ReconcileTask{
			monitortypes.NewReconcileTask(monitorreconsile.OTeldName, monitortypes.WithReconcileOption(monitorreconsile.OTeld, params)),
			monitortypes.NewReconcileTask(monitorreconsile.OteldSecretName, monitortypes.WithReconcileOption(monitorreconsile.Secret, params)),
			monitortypes.NewReconcileTask(monitorreconsile.OteldConfigMapNamePattern, monitortypes.WithReconcileOption(monitorreconsile.ConfigMap, params)),
			monitortypes.NewReconcileTask(monitorreconsile.OteldServiceNamePattern, monitortypes.WithReconcileOption(monitorreconsile.Service, params)),
			monitortypes.NewReconcileTask(monitorreconsile.OTeldAPIServerName, monitortypes.WithReconcileOption(monitorreconsile.Deployment, params)),
			monitortypes.NewReconcileTask(monitorreconsile.OTeldAgentName, monitortypes.WithReconcileOption(monitorreconsile.OTeldAgent, params)),
			monitortypes.NewReconcileTask(monitorreconsile.PrometheusName, monitortypes.WithReconcileOption(monitorreconsile.Prometheus, params)),
			monitortypes.NewReconcileTask(monitorreconsile.LokiName, monitortypes.WithReconcileOption(monitorreconsile.Loki, params)),
			monitortypes.NewReconcileTask(monitorreconsile.GrafnaName, monitortypes.WithReconcileOption(monitorreconsile.Grafana, params)),
			monitortypes.NewReconcileTask(monitorreconsile.VMAgentName, monitortypes.WithReconcileOption(monitorreconsile.VMAgent, params)),
		},
	}

	return &reconcile
}

// SetupWithManager sets up the controller with the Manager.
func (o *OTeldReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1alpha1.OTeld{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Watches(&monitorv1alpha1.LogsExporterSink{},
			handler.EnqueueRequestsFromMapFunc(o.filterOTelResources)).
		Watches(&monitorv1alpha1.MetricsExporterSink{},
			handler.EnqueueRequestsFromMapFunc(o.filterOTelResources)).
		Watches(&monitorv1alpha1.CollectorDataSource{},
			handler.EnqueueRequestsFromMapFunc(o.filterOTelResources)).
		Complete(o)
}

func (o *OTeldReconciler) filterOTelResources(ctx context.Context, obj client.Object) []reconcile.Request {
	var (
		items    = monitorv1alpha1.OTeldList{}
		requests []reconcile.Request
	)

	if err := o.Client.List(ctx, &items, client.InNamespace(obj.GetNamespace())); err != nil {
		ctrl.Log.Error(err, "failed to list otelds")
		return []reconcile.Request{}
	}
	for _, item := range items.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.GetNamespace(),
				Name:      item.Name,
			},
		})
	}
	return requests
}
