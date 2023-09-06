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
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/builder"
)

type KubeJob struct {
	// Name is the name of the action.
	Name string

	// Owner is the owner of the job.
	Owner *metav1.OwnerReference

	// ObjectMeta is the metadata of the job.
	ObjectMeta metav1.ObjectMeta

	// PodSpecBuilder is the builder of the pod spec.
	PodSpecBuilder builder.Builder

	// BackOffLimit is the number of retries before considering a Job as failed.
	BackOffLimit int32
}

func (j *KubeJob) GetName() string {
	return j.Name
}

func (j *KubeJob) Execute(ctx Context) error {
	key := client.ObjectKey{
		Namespace: j.ObjectMeta.Namespace,
		Name:      j.ObjectMeta.Name,
	}
	old := batchv1.Job{}
	// if found job exists, return
	if exists, err := intctrlutil.CheckResourceExists(ctx.Ctx, ctx.Client, key, &old); err != nil {
		return err
	} else if exists {
		return nil
	}

	podSpec := j.PodSpecBuilder.Build().(*corev1.PodSpec)

	// job not found, create it
	job := &batchv1.Job{
		ObjectMeta: j.ObjectMeta,
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: j.ObjectMeta,
				Spec:       *podSpec,
			},
			BackoffLimit: &j.BackOffLimit,
		},
	}

	// TODO: set finalizer
	// 		controllerutil.AddFinalizer(job, dataProtectionFinalizerName)

	// TODO: set controller reference
	// 		if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
	//			return err
	//		}

	return client.IgnoreAlreadyExists(ctx.Client.Create(ctx.Ctx, job))
}

func (j *KubeJob) validate() error {
	if j.ObjectMeta.Name == "" {
		return fmt.Errorf("name is required")
	}
	if j.PodSpecBuilder == nil {
		return fmt.Errorf("podSpecBuilder is required")
	}
	return nil
}

var _ Action = &KubeJob{}
