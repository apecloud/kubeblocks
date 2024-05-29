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
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"k8s.io/utils/pointer"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// LogCollectionReconciler reconciles a job of the backup and restore to collect the failed message.
type LogCollectionReconciler struct {
	client.Client
	Scheme     *k8sruntime.Scheme
	Recorder   record.EventRecorder
	RestConfig *rest.Config
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restores/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the backup closer to the desired state.
func (r *LogCollectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("job", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "logCollection", req.NamespacedName)

	// get job object, and return if not found
	job := &batchv1.Job{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, job); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if len(job.OwnerReferences) == 0 {
		return intctrlutil.Reconciled()
	}

	owner := job.OwnerReferences[0]
	switch owner.Kind {
	case dptypes.BackupKind:
		if err := r.patchBackupStatus(reqCtx, job, owner.Name); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	case dptypes.RestoreKind:
		if err := r.patchRestoreStatus(reqCtx, job, owner.Name); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogCollectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				job, ok := e.ObjectNew.(*batchv1.Job)
				if !ok {
					return false
				}
				if !r.ownedByDataProtection(job) {
					return false
				}
				for _, c := range job.Status.Conditions {
					if c.Type == batchv1.JobFailed {
						return true
					}
				}
				return false
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		})).WithOptions(controller.Options{
		MaxConcurrentReconciles: viper.GetInt(dptypes.CfgDataProtectionReconcileWorkers),
	}).Complete(r)
}

func (r *LogCollectionReconciler) ownedByDataProtection(job *batchv1.Job) bool {
	if len(job.OwnerReferences) == 0 {
		return false
	}
	if !strings.HasPrefix(job.OwnerReferences[0].APIVersion, dptypes.DataprotectionAPIGroup) {
		return false
	}
	return slices.Contains([]string{dptypes.BackupKind, dptypes.RestoreKind}, job.OwnerReferences[0].Kind)
}

func (r *LogCollectionReconciler) collectErrorLogs(reqCtx intctrlutil.RequestCtx, job *batchv1.Job) (string, error) {
	podList := &corev1.PodList{}
	if err := r.Client.List(reqCtx.Ctx, podList,
		client.InNamespace(job.Namespace),
		client.MatchingLabels{
			"job-name": job.Name,
		}); err != nil {
		return "", err
	}
	if len(podList.Items) == 0 {
		return "", nil
	}
	// sort pod with oldest creation place front
	slices.SortFunc(podList.Items, func(a, b corev1.Pod) bool {
		return !b.CreationTimestamp.Before(&(a.CreationTimestamp))
	})
	oldestPod := podList.Items[0]
	clientset, err := corev1client.NewForConfig(r.RestConfig)
	if err != nil {
		return "", err
	}
	// fetch the 10 tail logs
	currOpts := &corev1.PodLogOptions{
		Container: job.Spec.Template.Spec.Containers[0].Name,
		TailLines: pointer.Int64(10),
	}
	req := clientset.Pods(oldestPod.Namespace).GetLogs(oldestPod.Name, currOpts)
	data, err := req.DoRaw(reqCtx.Ctx)
	if err != nil {
		return "", err
	}
	startIndex := len(data) - 512
	if startIndex < 0 {
		startIndex = 0
	}
	// if the line exists trailing spaces, yaml can not parse it to multi-lines.
	lines := strings.Split(string(data[startIndex:]), "\n")
	var errorMessages []string
	for i := range lines {
		errorMessages = append(errorMessages, strings.TrimSpace(lines[i]))
	}
	return strings.Join(errorMessages, "\n"), nil
}

func (r *LogCollectionReconciler) patchBackupStatus(reqCtx intctrlutil.RequestCtx, job *batchv1.Job, backupName string) error {
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: backupName, Namespace: reqCtx.Req.Namespace}, backup); err != nil {
		return err
	}
	if backup.Status.FailureReason != "" {
		return nil
	}
	errorLogs, err := r.collectErrorLogs(reqCtx, job)
	if err != nil {
		return fmt.Errorf("collect error logs failed: %s", err.Error())
	}
	if errorLogs == "" {
		return nil
	}
	backup.Status.FailureReason = errorLogs
	return r.Client.Status().Update(reqCtx.Ctx, backup)
}

func (r *LogCollectionReconciler) patchRestoreStatus(reqCtx intctrlutil.RequestCtx, job *batchv1.Job, restoreName string) error {
	restore := &dpv1alpha1.Restore{}
	err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreName, Namespace: reqCtx.Req.Namespace}, restore)
	if err != nil {
		return err
	}
	actions := restore.Status.Actions
	if len(actions.PrepareData) == 0 && len(actions.PostReady) == 0 {
		return nil
	}

	objectKey := dprestore.BuildJobKeyForActionStatus(job.Name)
	logPrefix := "Collection logs"
	var hasPatchedLogs bool
	doLogsPatch := func(actions []dpv1alpha1.RestoreStatusAction) error {
		for i := range actions {
			if actions[i].ObjectKey != objectKey {
				continue
			}
			if strings.HasPrefix(actions[i].Message, logPrefix) {
				hasPatchedLogs = true
				return nil
			}
			errorLogs, err := r.collectErrorLogs(reqCtx, job)
			if err != nil {
				return fmt.Errorf("collect error logs failed: %s", err.Error())
			}
			if errorLogs == "" {
				return nil
			}
			actions[i].Message = fmt.Sprintf("%s: %s", logPrefix, errorLogs)
			break
		}
		return nil
	}
	if err = doLogsPatch(actions.PrepareData); err != nil {
		return err
	}
	if err = doLogsPatch(actions.PostReady); err != nil {
		return err
	}
	if hasPatchedLogs {
		return nil
	}
	return r.Client.Status().Update(reqCtx.Ctx, restore)
}
