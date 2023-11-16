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

package action

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

// JobAction is an action that creates a batch job.
type JobAction struct {
	// Name is the Name of the action.
	Name string

	// Owner is the owner of the job.
	Owner client.Object

	// ObjectMeta is the metadata of the job.
	ObjectMeta metav1.ObjectMeta

	PodSpec *corev1.PodSpec

	// BackOffLimit is the number of retries before considering a JobAction as failed.
	BackOffLimit *int32
}

func (j *JobAction) GetName() string {
	return j.Name
}

func (j *JobAction) Type() dpv1alpha1.ActionType {
	return dpv1alpha1.ActionTypeJob
}

func (j *JobAction) Execute(ctx Context) (*dpv1alpha1.ActionStatus, error) {
	sb := newStatusBuilder(j)
	handleErr := func(err error) (*dpv1alpha1.ActionStatus, error) {
		return sb.withErr(err).build(), err
	}

	if err := j.validate(); err != nil {
		return handleErr(err)
	}

	key := client.ObjectKey{
		Namespace: j.ObjectMeta.Namespace,
		Name:      j.ObjectMeta.Name,
	}
	original := batchv1.Job{}
	exists, err := ctrlutil.CheckResourceExists(ctx.Ctx, ctx.Client, key, &original)
	if err != nil {
		return handleErr(err)
	}

	// job exists, check job status and set action status accordingly
	if exists {
		objRef, _ := ref.GetReference(ctx.Scheme, &original)
		sb = sb.startTimestamp(&original.CreationTimestamp).objectRef(objRef)
		_, finishedType, msg := utils.IsJobFinished(&original)
		switch finishedType {
		case batchv1.JobComplete:
			return sb.phase(dpv1alpha1.ActionPhaseCompleted).
				completionTimestamp(nil).
				build(), nil
		case batchv1.JobFailed:
			return sb.phase(dpv1alpha1.ActionPhaseFailed).
				completionTimestamp(nil).
				reason(msg).
				build(), nil
		}
		// job is running
		return handleErr(nil)
	}

	// job doesn't exist, create it
	job := &batchv1.Job{
		ObjectMeta: j.ObjectMeta,
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: j.ObjectMeta,
				Spec:       *j.PodSpec,
			},
			BackoffLimit: j.BackOffLimit,
		},
	}

	controllerutil.AddFinalizer(job, types.DataProtectionFinalizerName)
	if job.Namespace == j.Owner.GetNamespace() {
		if err = utils.SetControllerReference(j.Owner, job, ctx.Scheme); err != nil {
			return handleErr(err)
		}
	}
	msg := fmt.Sprintf("creating job %s/%s", job.Namespace, job.Name)
	ctx.Recorder.Event(j.Owner, corev1.EventTypeNormal, "CreatingJob", msg)
	return handleErr(client.IgnoreAlreadyExists(ctx.Client.Create(ctx.Ctx, job)))
}

func (j *JobAction) validate() error {
	if j.ObjectMeta.Name == "" {
		return fmt.Errorf("name is required")
	}
	if j.PodSpec == nil {
		return fmt.Errorf("PodSpec is required")
	}
	if j.BackOffLimit == nil {
		j.BackOffLimit = &types.DefaultBackOffLimit
	}
	return nil
}

var _ Action = &JobAction{}
