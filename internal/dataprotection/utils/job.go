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

package utils

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/errors"
)

func EnsureBatchV1JobCompleted(ctx context.Context, cli client.Client, key client.ObjectKey) (bool, error) {
	job := &batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(ctx, cli, key, job)
	if err != nil {
		return false, err
	}
	if exists {
		if ContainsJobCondition(job, batchv1.JobComplete) {
			return true, nil
		}
		if ContainsJobCondition(job, batchv1.JobFailed) {
			return false, errors.NewBackupJobFailed(job.Name)
		}
	}
	return false, nil
}

func ContainsJobCondition(job *batchv1.Job, jobCondType batchv1.JobConditionType) bool {
	for _, jobCond := range job.Status.Conditions {
		if jobCond.Type == jobCondType {
			return true
		}
	}
	return false
}

func BatchV1JobCompleted(job *batchv1.Job) bool {
	return ContainsJobCondition(job, batchv1.JobComplete)
}

func BatchV1JobFailed(job *batchv1.Job) bool {
	return ContainsJobCondition(job, batchv1.JobFailed)
}
