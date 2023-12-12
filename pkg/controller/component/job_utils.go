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

package component

import (
	"context"
	"errors"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	KBJobTTLSecondsAfterFinished = 5
)

// GetJobWithLabels gets the job list with the specified labels.
func GetJobWithLabels(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	matchLabels client.MatchingLabels) ([]batchv1.Job, error) {
	jobList := &batchv1.JobList{}
	if err := cli.List(ctx, jobList, client.InNamespace(cluster.Namespace), matchLabels); err != nil {
		return nil, err
	}
	return jobList.Items, nil
}

// CleanJobWithLabels cleans up the job tasks with label.
func CleanJobWithLabels(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	matchLabels client.MatchingLabels) error {
	jobList, err := GetJobWithLabels(ctx, cli, cluster, matchLabels)
	if err != nil {
		return err
	}
	for _, job := range jobList {
		var ttl = int32(KBJobTTLSecondsAfterFinished)
		patch := client.MergeFrom(job.DeepCopy())
		job.Spec.TTLSecondsAfterFinished = &ttl
		if err := cli.Patch(ctx, &job, patch); err != nil {
			return err
		}
	}
	return nil
}

// CleanJobByName cleans up the job task by name.
func CleanJobByName(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	jobName string) error {
	job := &batchv1.Job{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
	if err := cli.Get(ctx, key, job); err != nil {
		return err
	}
	var ttl = int32(KBJobTTLSecondsAfterFinished)
	patch := client.MergeFrom(job.DeepCopy())
	job.Spec.TTLSecondsAfterFinished = &ttl
	if err := cli.Patch(ctx, job, patch); err != nil {
		return err
	}
	return nil
}

// CheckJobSucceed checks the result of job execution.
// Returns:
// - bool: whether job exist, true exist
// - error: any error that occurred during the handling
func CheckJobSucceed(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	jobName string) error {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
	currentJob := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(ctx, cli, key, &currentJob)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("job not exist, pls check")
	}
	jobStatusConditions := currentJob.Status.Conditions
	if len(jobStatusConditions) > 0 {
		switch jobStatusConditions[0].Type {
		case batchv1.JobComplete:
			return nil
		case batchv1.JobFailed:
			return errors.New("job failed, pls check")
		default:
			return intctrlutil.NewErrorf(intctrlutil.ErrorWaitCacheRefresh, "requeue to waiting for job %s finished.", key.Name)
		}
	}
	return intctrlutil.NewErrorf(intctrlutil.ErrorWaitCacheRefresh, "requeue to waiting for job %s finished.", key.Name)
}
