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
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
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
	"sigs.k8s.io/controller-runtime/pkg/source"

	monitorv1alpha1 "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	monitorreconsile "github.com/apecloud/kubeblocks/controllers/monitor/reconcile"
	monitortypes "github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OTeld object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *OTeldReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := monitortypes.ReconcileCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithName("OTeldCollectorReconciler"),

		Config: r.Config,
	}

	// TODO prepare required resources

	if err := r.runTasks(reqCtx); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *OTeldReconciler) runTasks(reqCtx monitortypes.ReconcileCtx) error {
	for _, task := range r.tasks {
		if err := task.Do(reqCtx); err != nil {
			return err
		}
	}
	return nil
}

func New(params monitortypes.OTeldParams, config *monitortypes.Config) *OTeldReconciler {
	reconcile := OTeldReconciler{
		Client:   params.Client,
		Scheme:   params.Scheme,
		Recorder: params.Recorder,
		Config:   config,

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

	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Log.Info(fmt.Sprintf("config file changed: %s", e.Name))
		newConfig, err := monitortypes.LoadConfig(viper.ConfigFileUsed())
		if err != nil {
			log.Log.Error(err, fmt.Sprintf("failed to reload config: %s", e.Name))
			return
		}
		// TODO how to trigger the operator to reconcile
		reconcile.Config = newConfig
	})
	return &reconcile
}

// SetupWithManager sets up the controller with the Manager.
func (r *OTeldReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1alpha1.OTeldCollectorTemplate{}).
		Owns(&monitorv1alpha1.LogsExporterSink{}).
		Owns(&monitorv1alpha1.MetricsExporterSink{}).
		Owns(&monitorv1alpha1.CollectorDataSource{}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.filterOTelResources)).
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.filterOTelResources)).
		Watches(&source.Kind{Type: &appsv1.DaemonSet{}},
			handler.EnqueueRequestsFromMapFunc(r.filterOTelResources)).
		Watches(&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(r.filterOTelResources)).
		Complete(r)
}

func (r *OTeldReconciler) filterOTelResources(obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if obj.GetNamespace() != viper.GetString("OTELD_NAMESPACE") {
		return []reconcile.Request{}
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      labels[constant.AppInstanceLabelKey],
			},
		},
	}
}
