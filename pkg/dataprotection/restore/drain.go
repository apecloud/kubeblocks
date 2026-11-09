/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package restore

import (
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// DrainExistingJobs releases managers in existing Jobs without building any new work.
// Jobs are discovered independently of status so a restart after creation is safe.
func (r *RestoreManager) DrainExistingJobs(reqCtx intctrlutil.RequestCtx) (bool, error) {
	namespaces := map[string]struct{}{r.Restore.Namespace: {}}
	if namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS); namespace != "" {
		namespaces[namespace] = struct{}{}
	}
	drained := true
	for namespace := range namespaces {
		jobs := &batchv1.JobList{}
		if err := r.Client.List(reqCtx.Ctx, jobs, client.InNamespace(namespace),
			client.MatchingLabels{DataProtectionRestoreLabelKey: r.Restore.Name}); err != nil {
			return false, err
		}
		for i := range jobs.Items {
			job := &jobs.Items[i]
			if !r.ownsExistingJob(job) {
				continue
			}
			finished, _, message := utils.IsJobFinished(job)
			if !finished {
				drained = false
				continue
			}
			r.recordDrainedJob(job, message)
		}
		// A Job deletion can be visible before its Pods disappear. Those Pods
		// must also finish before the rejected operation becomes terminal.
		pods := &corev1.PodList{}
		if err := r.Client.List(reqCtx.Ctx, pods, client.InNamespace(namespace),
			client.MatchingLabels{DataProtectionRestoreLabelKey: r.Restore.Name}); err != nil {
			return false, err
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			owner := metav1.GetControllerOf(pod)
			if owner == nil || owner.Kind != "Job" || owner.APIVersion != batchv1.SchemeGroupVersion.String() {
				continue
			}
			job := &batchv1.Job{}
			err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Namespace: namespace, Name: owner.Name}, job)
			switch {
			case apierrors.IsNotFound(err):
				if !r.hasExistingJobName(owner.Name) || !r.isJobForRestoreAction(&batchv1.Job{ObjectMeta: pod.ObjectMeta}) {
					continue
				}
			case err != nil:
				return false, err
			case job.UID != owner.UID || !r.ownsExistingJob(job):
				continue
			}
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			drained = false
			for _, status := range pod.Status.ContainerStatuses {
				if status.Name == Restore && status.State.Terminated != nil {
					if err := r.StopManagerContainer(pod); err != nil {
						return false, err
					}
				}
			}
		}
	}
	return drained, nil
}

func (r *RestoreManager) ownsExistingJob(job *batchv1.Job) bool {
	if !r.isJobForRestoreAction(job) {
		return false
	}
	if job.Namespace == r.Restore.Namespace {
		owner := metav1.GetControllerOf(job)
		return owner != nil && owner.APIVersion == dpv1alpha1.GroupVersion.String() &&
			owner.Kind == "Restore" && owner.Name == r.Restore.Name && owner.UID == r.Restore.UID
	}
	// Cross-namespace Exec Jobs cannot carry a Restore owner reference. Their
	// existing correlation labels and UID-derived builder name identify them.
	return r.hasExistingJobName(job.Name)
}

func (r *RestoreManager) hasExistingJobName(name string) bool {
	if len(r.Restore.UID) < 8 {
		return false
	}
	// PostReady action Jobs use "post-ready", not the RestoreStage value.
	for _, stage := range []string{"preparedata", "post-ready"} {
		if strings.HasPrefix(name, "restore-"+stage+"-"+string(r.Restore.UID[:8])+"-") {
			return true
		}
	}
	return false
}

func (r *RestoreManager) recordDrainedJob(job *batchv1.Job, message string) {
	for _, actions := range []*[]dpv1alpha1.RestoreStatusAction{&r.Restore.Status.Actions.PrepareData, &r.Restore.Status.Actions.PostReady} {
		for i := range *actions {
			action := &(*actions)[i]
			if action.ObjectKey != BuildJobKeyForActionStatus(job.Name) || action.Status != dpv1alpha1.RestoreActionProcessing {
				continue
			}
			action.Status = dpv1alpha1.RestoreActionCompleted
			action.Message = "existing restore Job completed"
			if message != "" {
				action.Status = dpv1alpha1.RestoreActionFailed
				action.Message = message
			}
		}
	}
}
