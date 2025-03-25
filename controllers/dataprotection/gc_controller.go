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

package dataprotection

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.Backup{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(client.Object) bool { return false }))).
		WatchesRawSource(s, nil).
		Complete(r)
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// delete expired backups.
func (r *GCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
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
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	// backup is being deleted, skip
	if !backup.DeletionTimestamp.IsZero() {
		reqCtx.Log.V(1).Info("backup is being deleted, skipping")
		return intctrlutil.Reconciled()
	}

	reqCtx.Log.V(1).Info("gc reconcile", "backup", req.String(),
		"phase", backup.Status.Phase, "expiration", backup.Status.Expiration)
	reqCtx.Log = reqCtx.Log.WithValues("expiration", backup.Status.Expiration)

	now := r.clock.Now()
	if backup.Status.Expiration == nil || backup.Status.Expiration.After(now) {
		reqCtx.Log.V(1).Info("backup is not expired yet, skipping")
		return intctrlutil.Reconciled()
	}

	if deletable, err := r.isBackupDeletable(reqCtx, backup); !deletable {
		return intctrlutil.Reconciled()
	} else if err != nil {
		reqCtx.Log.Error(err, "failed to check backup deletability")
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	reqCtx.Log.Info("backup has expired, delete it", "backup", req.String())
	if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, backup); err != nil {
		reqCtx.Log.Error(err, "failed to delete backup")
		r.Recorder.Event(backup, corev1.EventTypeWarning, "RemoveExpiredBackupsFailed", err.Error())
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

func getGCFrequency() time.Duration {
	gcFrequencySeconds := viper.GetInt(dptypes.CfgKeyGCFrequencySeconds)
	if gcFrequencySeconds > 0 {
		return time.Duration(gcFrequencySeconds) * time.Second
	}
	return dptypes.DefaultGCFrequencySeconds
}

// isBackupDeletable returns true if the backup can be deleted.
func (r *GCReconciler) isBackupDeletable(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) (bool, error) {
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return true, nil
	}
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	err := r.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      backup.Spec.BackupPolicyName,
		Namespace: backup.Namespace,
	}, backupPolicy)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	backupType := backup.Labels[dptypes.BackupTypeLabelKey]
	if len(backupType) == 0 {
		// fallback to get backup type from action set
		method := dputils.GetBackupMethodByName(backup.Spec.BackupMethod, backupPolicy)
		if method != nil {
			actionSet := &dpv1alpha1.ActionSet{}
			if len(method.ActionSetName) == 0 {
				actionSet = nil
			} else {
				err := r.Get(reqCtx.Ctx, client.ObjectKey{Name: method.ActionSetName}, actionSet)
				if apierrors.IsNotFound(err) {
					return true, nil
				} else if err != nil {
					return true, err
				}
			}
			backupType = string(dputils.GetBackupType(actionSet, method.SnapshotVolumes))
		}
	}
	if backupType == string(dpv1alpha1.BackupTypeIncremental) {
		parentBackup := &dpv1alpha1.Backup{}
		if len(backup.Status.ParentBackupName) != 0 {
			err := r.Get(reqCtx.Ctx, client.ObjectKey{
				Name:      backup.Status.ParentBackupName,
				Namespace: backup.Namespace,
			}, parentBackup)
			if apierrors.IsNotFound(err) {
				return true, nil
			} else if err != nil {
				return true, err
			}
		}
		reqCtx.Log.V(1).Info(fmt.Sprintf(
			"backup %s/%s is an incremental backup and will be auto-deleted when its parent backup is deleted, skipping",
			backup.Namespace, backup.Name))
		return false, nil
	}
	if backupPolicy.Spec.RetentionPolicy == dpv1alpha1.BackupPolicyRetentionPolicyRetainLatestBackup {
		isLatest, err := r.isLatestCompletedBackup(reqCtx.Ctx, backup)
		if err != nil {
			return true, err
		}
		if isLatest {
			reqCtx.Log.V(1).Info(fmt.Sprintf(
				"backup %s/%s is the latest completed backup of backup method %s and will be retained, skipping",
				backup.Namespace, backup.Name, backup.Spec.BackupMethod))
			return false, nil
		}
	}
	return true, nil
}

// isLatestCompletedBackup returns true if the backup is the latest completed backup.
func (r *GCReconciler) isLatestCompletedBackup(ctx context.Context, backup *dpv1alpha1.Backup) (bool, error) {
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return false, nil
	}
	relatedBackups, err := r.getRelatedBackups(ctx, backup)
	if err != nil {
		return false, err
	}
	if len(relatedBackups) == 0 {
		return false, nil
	}
	completedBackups := make([]*dpv1alpha1.Backup, 0)
	for _, b := range relatedBackups {
		if b.Status.Phase == dpv1alpha1.BackupPhaseCompleted {
			completedBackups = append(completedBackups, b)
		}
	}
	if len(completedBackups) == 0 {
		return false, nil
	}
	// sort by stop time in descending order
	sort.Slice(completedBackups, func(i, j int) bool {
		i, j = j, i
		return dputils.CompareWithBackupStopTime(*completedBackups[i], *completedBackups[j])
	})
	// retain the backup if it is the latest completed backup
	if completedBackups[0].Name == backup.Name {
		return true, nil
	}
	return false, nil
}

// getRelatedBackups returns the related backups of the given backup.
func (r *GCReconciler) getRelatedBackups(ctx context.Context, backup *dpv1alpha1.Backup) ([]*dpv1alpha1.Backup, error) {
	clusterUID := backup.Labels[dptypes.ClusterUIDLabelKey]
	if len(clusterUID) == 0 {
		return nil, nil
	}
	backupList := &dpv1alpha1.BackupList{}
	if err := r.List(ctx, backupList, client.InNamespace(backup.Namespace),
		client.MatchingLabels(map[string]string{
			dptypes.ClusterUIDLabelKey:   clusterUID,
			dptypes.BackupPolicyLabelKey: backup.Spec.BackupPolicyName,
		})); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	} else if apierrors.IsNotFound(err) {
		return nil, nil
	}
	var filtered []*dpv1alpha1.Backup
	for i, b := range backupList.Items {
		if len(b.Spec.BackupMethod) != 0 && b.Spec.BackupMethod == backup.Spec.BackupMethod {
			filtered = append(filtered, &backupList.Items[i])
		}
	}
	return filtered, nil
}
