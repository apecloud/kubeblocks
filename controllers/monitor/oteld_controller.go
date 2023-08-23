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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitorv1alpha1 "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/reconcile"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// OTeldReconciler reconciles a OTeld object
type OTeldReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   *types.Config

	// sub-controllers
	tasks []types.ReconcileTask
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
	reqCtx := types.ReconcileCtx{
		Ctx:    ctx,
		Req:    req,
		Log:    log.FromContext(ctx).WithName("OTeldCollectorReconciler"),
		Config: r.Config,
	}

	// TODO prepare required resources

	if err := r.runTasks(reqCtx); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *OTeldReconciler) runTasks(reqCtx types.ReconcileCtx) error {
	for _, task := range r.tasks {
		if err := task.Do(reqCtx); err != nil {
			return err
		}
	}
	return nil
}

func New(params types.OTeldParams, config *types.Config) *OTeldReconciler {
	reconcile := OTeldReconciler{
		Client:   params.Client,
		Scheme:   params.Scheme,
		Recorder: params.Recorder,
		Config:   config,

		// sub-controllers
		tasks: []types.ReconcileTask{
			types.NewReconcileTask(reconcile.OTeldName, types.WithReconcileOption(reconcile.OTeld, params)),
			types.NewReconcileTask(reconcile.OTeldAPIServerName, types.WithReconcileOption(reconcile.Deployment, params)),
			types.NewReconcileTask(reconcile.OTeldAgentName, types.WithReconcileOption(reconcile.OTeldAgent, params)),
			types.NewReconcileTask(reconcile.PrometheusName, types.WithReconcileOption(reconcile.Prometheus, params)),
			types.NewReconcileTask(reconcile.LokiName, types.WithReconcileOption(reconcile.Loki, params)),
			types.NewReconcileTask(reconcile.GrafnaName, types.WithReconcileOption(reconcile.Grafana, params)),
			types.NewReconcileTask(reconcile.VMAgentName, types.WithReconcileOption(reconcile.VMAgent, params)),
		},
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Log.Info(fmt.Sprintf("config file changed: %s", e.Name))
		newConfig, err := types.LoadConfig(viper.ConfigFileUsed())
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
		// For(&monitorv1alpha1.OTeld{}).
		For(&monitorv1alpha1.LogsExporterSink{}).
		For(&monitorv1alpha1.MetricsExporterSink{}).
		For(&monitorv1alpha1.CollectorDataSource{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
