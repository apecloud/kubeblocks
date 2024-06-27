/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package multiversion

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

// covert workloadsv1alpha1.instanceset resources to workloadsv1.instanceset

var (
	itsResource = "instancesets"
	itsGVR      = workloadsv1.GroupVersion.WithResource(itsResource)
)

func init() {
	hook.RegisterCRDConversion(itsGVR, hook.NewNoVersion(1, 0), itsHandler(),
		hook.NewNoVersion(0, 9))
}

func itsHandler() hook.ConversionHandler {
	return &convertor{
		namespaces: []string{"default"}, // TODO: namespaces
		sourceKind: &itsConvertor{},
		targetKind: &itsConvertor{},
	}
}

type itsConvertor struct{}

func (c *itsConvertor) kind() string {
	return "InstanceSet"
}

func (c *itsConvertor) list(ctx context.Context, cli *versioned.Clientset, namespace string) ([]client.Object, error) {
	list, err := cli.WorkloadsV1alpha1().InstanceSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *itsConvertor) get(ctx context.Context, cli *versioned.Clientset, namespace, name string) (client.Object, error) {
	return cli.WorkloadsV1().InstanceSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *itsConvertor) convert(source client.Object) []client.Object {
	its := source.(*workloadsv1alpha1.InstanceSet)
	return []client.Object{
		&workloadsv1.InstanceSet{
			Spec: workloadsv1.InstanceSetSpec{
				Replicas:                  its.Spec.Replicas,
				MinReadySeconds:           its.Spec.MinReadySeconds,
				Selector:                  its.Spec.Selector,
				Service:                   its.Spec.Service,
				Template:                  its.Spec.Template,
				Instances:                 c.instances(its.Spec.Instances),
				OfflineInstances:          its.Spec.OfflineInstances,
				VolumeClaimTemplates:      its.Spec.VolumeClaimTemplates,
				PodManagementPolicy:       its.Spec.PodManagementPolicy,
				UpdateStrategy:            its.Spec.UpdateStrategy,
				Roles:                     c.roles(its.Spec.Roles),
				RoleProbe:                 c.roleProbe(its.Spec.RoleProbe),
				MembershipReconfiguration: c.membershipReconfiguration(its.Spec.MembershipReconfiguration),
				MemberUpdateStrategy:      c.memberUpdateStrategy(its.Spec.MemberUpdateStrategy),
				Paused:                    its.Spec.Paused,
				Credential:                c.credential(its.Spec.Credential),
			},
		},
	}
}

func (c *itsConvertor) instances(templates []workloadsv1alpha1.InstanceTemplate) []workloadsv1.InstanceTemplate {
	if len(templates) == 0 {
		return nil
	}
	newTemplates := make([]workloadsv1.InstanceTemplate, 0)
	for _, template := range templates {
		newTemplate := workloadsv1.InstanceTemplate{
			Name:                 template.Name,
			Replicas:             template.Replicas,
			Annotations:          template.Annotations,
			Labels:               template.Labels,
			Image:                template.Image,
			Resources:            template.Resources,
			Env:                  template.Env,
			Volumes:              template.Volumes,
			VolumeMounts:         template.VolumeMounts,
			VolumeClaimTemplates: template.VolumeClaimTemplates,
		}
		if template.SchedulingPolicy != nil {
			newTemplate.SchedulingPolicy = &workloadsv1.SchedulingPolicy{
				SchedulerName:             template.SchedulingPolicy.SchedulerName,
				NodeSelector:              template.SchedulingPolicy.NodeSelector,
				NodeName:                  template.SchedulingPolicy.NodeName,
				Affinity:                  template.SchedulingPolicy.Affinity,
				Tolerations:               template.SchedulingPolicy.Tolerations,
				TopologySpreadConstraints: template.SchedulingPolicy.TopologySpreadConstraints,
			}
		}
		newTemplates = append(newTemplates, newTemplate)
	}
	return newTemplates
}

func (c *itsConvertor) roles(roles []workloadsv1alpha1.ReplicaRole) []workloadsv1.ReplicaRole {
	if len(roles) == 0 {
		return nil
	}
	newRoles := make([]workloadsv1.ReplicaRole, 0)
	for _, role := range roles {
		newRoles = append(newRoles, workloadsv1.ReplicaRole{
			Name:       role.Name,
			AccessMode: workloadsv1.AccessMode(role.AccessMode),
			CanVote:    role.CanVote,
			IsLeader:   role.IsLeader,
		})
	}
	return newRoles
}

func (c *itsConvertor) roleProbe(probe *workloadsv1alpha1.RoleProbe) *workloadsv1.RoleProbe {
	if probe == nil {
		return nil
	}
	newProbe := &workloadsv1.RoleProbe{
		BuiltinHandler:      probe.BuiltinHandler,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		TimeoutSeconds:      probe.TimeoutSeconds,
		PeriodSeconds:       probe.PeriodSeconds,
		SuccessThreshold:    probe.SuccessThreshold,
		FailureThreshold:    probe.FailureThreshold,
		RoleUpdateMechanism: workloadsv1.RoleUpdateMechanism(probe.RoleUpdateMechanism),
	}
	if probe.CustomHandler != nil {
		newProbe.CustomHandler = make([]workloadsv1.Action, 0)
		for _, action := range probe.CustomHandler {
			newProbe.CustomHandler = append(newProbe.CustomHandler, workloadsv1.Action{
				Image:   action.Image,
				Command: action.Command,
				Args:    action.Args,
			})
		}
	}
	return newProbe
}

func (c *itsConvertor) membershipReconfiguration(r *workloadsv1alpha1.MembershipReconfiguration) *workloadsv1.MembershipReconfiguration {
	if r == nil {
		return nil
	}
	action := func(s *workloadsv1alpha1.Action) *workloadsv1.Action {
		return &workloadsv1.Action{
			Image:   s.Image,
			Command: s.Command,
			Args:    s.Args,
		}
	}
	rr := &workloadsv1.MembershipReconfiguration{}
	if r.SwitchoverAction != nil {
		rr.SwitchoverAction = action(r.SwitchoverAction)
	}
	if r.MemberJoinAction != nil {
		rr.MemberJoinAction = action(r.MemberJoinAction)
	}
	if r.MemberLeaveAction != nil {
		rr.MemberLeaveAction = action(r.MemberLeaveAction)
	}
	if r.LogSyncAction != nil {
		rr.LogSyncAction = action(r.LogSyncAction)
	}
	if r.PromoteAction != nil {
		rr.PromoteAction = action(r.PromoteAction)
	}
	return rr
}

func (c *itsConvertor) memberUpdateStrategy(strategy *workloadsv1alpha1.MemberUpdateStrategy) *workloadsv1.MemberUpdateStrategy {
	if strategy == nil {
		return nil
	}
	s := workloadsv1.MemberUpdateStrategy(*strategy)
	return &s
}

func (c *itsConvertor) credential(credential *workloadsv1alpha1.Credential) *workloadsv1.Credential {
	if credential == nil {
		return nil
	}
	return &workloadsv1.Credential{
		Username: workloadsv1.CredentialVar{
			Value:     credential.Username.Value,
			ValueFrom: credential.Username.ValueFrom,
		},
		Password: workloadsv1.CredentialVar{
			Value:     credential.Password.Value,
			ValueFrom: credential.Password.ValueFrom,
		},
	}
}
