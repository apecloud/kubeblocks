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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

// StsAction is an action that runs a statefulset workload.
type StsAction struct {
	// Name is the Name of the action.
	Name string

	// Owner is the owner of the statefulset.
	Owner client.Object

	// ObjectMeta is the metadata of the statefulset.
	ObjectMeta metav1.ObjectMeta

	// PodSpec is the
	PodSpec *corev1.PodSpec

	// Replicas is the number of replicas of the statefulset.
	Replicas *int32
}

func (s *StsAction) GetName() string {
	return s.Name
}

func (s *StsAction) Type() dpv1alpha1.ActionType {
	return dpv1alpha1.ActionTypeStatefulSet
}

func (s *StsAction) Execute(ctx Context) (*dpv1alpha1.ActionStatus, error) {
	sb := newStatusBuilder(s)
	handleErr := func(err error) (*dpv1alpha1.ActionStatus, error) {
		return sb.build(), err
	}

	if err := s.validate(); err != nil {
		return handleErr(err)
	}

	key := client.ObjectKey{
		Namespace: s.ObjectMeta.Namespace,
		Name:      s.ObjectMeta.Name,
	}
	original := appsv1.StatefulSet{}
	exists, err := intctrlutil.CheckResourceExists(ctx.Ctx, ctx.Client, key, &original)
	if err != nil {
		return handleErr(err)
	}

	sts, err := s.buildStatefulSet(ctx.Scheme)
	if err != nil {
		return handleErr(err)
	}

	// statefulSet does not exist, create it
	if !exists {
		msg := fmt.Sprintf("creating statefulSet %s/%s", sts.Namespace, sts.Name)
		ctx.Recorder.Event(s.Owner, corev1.EventTypeNormal, "CreatingSts-"+key.Name, msg)
		return handleErr(client.IgnoreAlreadyExists(ctx.Client.Create(ctx.Ctx, sts)))
	}

	// statefulSet exists, check statefulSet status and set action status accordingly
	sb = sb.startTimestamp(&original.CreationTimestamp).
		ObjectRef(&corev1.ObjectReference{
			Kind:       original.Kind,
			Namespace:  original.Namespace,
			Name:       original.Name,
			APIVersion: original.APIVersion,
			UID:        original.UID,
		}).availableReplicas(original.Status.AvailableReplicas)

	// statefulSet exist, update its status
	original.Spec.Template = sts.Spec.Template
	return handleErr(ctx.Client.Update(ctx.Ctx, &original))
}

func (s *StsAction) buildStatefulSet(scheme *runtime.Scheme) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: s.ObjectMeta,
		Spec: appsv1.StatefulSetSpec{
			Replicas: s.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: s.ObjectMeta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: s.ObjectMeta.Labels,
				},
				Spec: *s.PodSpec,
			},
		},
	}

	controllerutil.AddFinalizer(sts, types.DataProtectionFinalizerName)
	if err := controllerutil.SetControllerReference(s.Owner, sts, scheme); err != nil {
		return nil, err
	}
	return sts, nil
}

func (s *StsAction) validate() error {
	defaultReplicas := int32(1)
	if s.Replicas == nil || *s.Replicas <= 0 {
		s.Replicas = &defaultReplicas
	}
	return nil
}

var _ Action = &StsAction{}
