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

package dataprotection

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// GCReconciler garbage collection reconciler, which periodically deletes expired backups.
type GCReconciler struct {
	client.Client
	Recorder  record.EventRecorder
	clock     clock.WithTickerAndDelayedExecution
	frequency time.Duration
}

func NewGCReconciler(mgr ctrl.Manager) *GCReconciler {
	return &GCReconciler{
		Client:    mgr.GetClient(),
		Recorder:  mgr.GetEventRecorderFor("gc-controller"),
		clock:     clock.RealClock{},
		frequency: getGCFrequency(),
	}
}

// SetupWithManager sets up the GCReconciler using the supplied manager.
// GCController only watches on CreateEvent for ensuring every new backup will be
// taken care of. Other events will be filtered to decrease the load on the controller.
func (r *GCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := dputils.NewPeriodicalEnqueueSource(mgr.GetClient(), &dpv1alpha1.BackupList{}, r.frequency, dputils.PeriodicalEnqueueSourceOption{})
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.Backup{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(_ event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).
		WatchesRawSource(s, nil).
		Complete(r)
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// delete expired backups.
func (r *GCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := ctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("gc backup", req.NamespacedName),
		Recorder: r.Recorder,
	}
	reqCtx.Log.V(1).Info("gcController getting backup")

	backup := &dpv1alpha1.Backup{}
	if err := r.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backup); err != nil {
		if apierrors.IsNotFound(err) {
			reqCtx.Log.Error(err, "backup ont found")
			return ctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	// backup is being deleted, skip
	if !backup.DeletionTimestamp.IsZero() {
		reqCtx.Log.V(1).Info("backup is being deleted, skipping")
		return ctrlutil.Reconciled()
	}

	reqCtx.Log.V(1).Info("gc reconcile", "backup", req.String(),
		"phase", backup.Status.Phase, "expiration", backup.Status.Expiration)
	reqCtx.Log = reqCtx.Log.WithValues("expiration", backup.Status.Expiration)

	now := r.clock.Now()
	if backup.Status.Expiration == nil || backup.Status.Expiration.After(now) {
		reqCtx.Log.V(1).Info("backup is not expired yet, skipping")
		return ctrlutil.Reconciled()
	}

	reqCtx.Log.Info("backup has expired, delete it", "backup", req.String())
	if err := ctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, backup); err != nil {
		reqCtx.Log.Error(err, "failed to delete backup")
		r.Recorder.Event(backup, corev1.EventTypeWarning, "RemoveExpiredBackupsFailed", err.Error())
		return ctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return ctrlutil.Reconciled()
}

func getGCFrequency() time.Duration {
	gcFrequencySeconds := viper.GetInt(dptypes.CfgKeyGCFrequencySeconds)
	if gcFrequencySeconds > 0 {
		return time.Duration(gcFrequencySeconds) * time.Second
	}
	return dptypes.DefaultGCFrequencySeconds
}
