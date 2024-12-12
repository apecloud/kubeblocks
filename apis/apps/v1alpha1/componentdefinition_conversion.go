/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"github.com/jinzhu/copier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ConvertTo converts this ComponentDefinition to the Hub version (v1).
func (r *ComponentDefinition) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv1.ComponentDefinition)

	// objectMeta
	dst.ObjectMeta = r.ObjectMeta

	// spec
	if err := copier.Copy(&dst.Spec, &r.Spec); err != nil {
		return err
	}
	if err := incrementConvertTo(r, dst); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&dst.Status, &r.Status); err != nil {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1) to this version.
func (r *ComponentDefinition) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv1.ComponentDefinition)

	// objectMeta
	r.ObjectMeta = src.ObjectMeta

	// spec
	if err := copier.Copy(&r.Spec, &src.Spec); err != nil {
		return err
	}
	if err := incrementConvertFrom(r, src, &componentDefinitionConverter{}); err != nil {
		return err
	}

	// status
	if err := copier.Copy(&r.Status, &src.Status); err != nil {
		return err
	}

	return nil
}

// convertTo converts this ComponentDefinition to the Hub version (v1).
func (r *ComponentDefinition) incrementConvertTo(dstRaw metav1.Object) (incrementChange, error) {
	// changed
	if err := r.changesToComponentDefinition(dstRaw.(*appsv1.ComponentDefinition)); err != nil {
		return nil, err
	}

	// deleted
	c := &componentDefinitionConverter{
		Monitor:        r.Spec.Monitor,
		RoleArbitrator: r.Spec.RoleArbitrator,
	}
	if r.Spec.LifecycleActions != nil && r.Spec.LifecycleActions.Switchover != nil {
		c.LifecycleActionSwitchover = r.Spec.LifecycleActions.Switchover
	}
	return c, nil
}

// convertFrom converts from the Hub version (v1) to this version.
func (r *ComponentDefinition) incrementConvertFrom(srcRaw metav1.Object, ic incrementChange) error {
	// deleted
	c := ic.(*componentDefinitionConverter)
	r.Spec.Monitor = c.Monitor
	r.Spec.RoleArbitrator = c.RoleArbitrator
	if c.LifecycleActionSwitchover != nil {
		if r.Spec.LifecycleActions == nil {
			r.Spec.LifecycleActions = &ComponentLifecycleActions{}
		}
		r.Spec.LifecycleActions.Switchover = c.LifecycleActionSwitchover
	}

	// changed
	return r.changesFromComponentDefinition(srcRaw.(*appsv1.ComponentDefinition))
}

func (r *ComponentDefinition) changesToComponentDefinition(cmpd *appsv1.ComponentDefinition) error {
	// changed:
	//   spec
	//     vars
	//       - ValueFrom
	//           componentVarRef:
	//             instanceNames -> podNames
	//     updateStrategy -> updateConcurrency
	//     lifecycleActions

	for _, v := range r.Spec.Vars {
		if v.ValueFrom == nil || v.ValueFrom.ComponentVarRef == nil || v.ValueFrom.ComponentVarRef.InstanceNames == nil {
			continue
		}
		if err := r.toV1VarsPodNames(v, cmpd); err != nil {
			return err
		}
	}
	if r.Spec.UpdateStrategy != nil {
		cmpd.Spec.UpdateConcurrency = (*appsv1.UpdateConcurrency)(r.Spec.UpdateStrategy)
	}
	r.toV1LifecycleActions(cmpd)
	return nil
}

func (r *ComponentDefinition) changesFromComponentDefinition(cmpd *appsv1.ComponentDefinition) error {
	// changed:
	//   spec
	//     vars
	//       - ValueFrom
	//           componentVarRef:
	//             instanceNames -> podNames
	//     updateStrategy -> updateConcurrency
	//     lifecycleActions

	for _, v := range cmpd.Spec.Vars {
		if v.ValueFrom == nil || v.ValueFrom.ComponentVarRef == nil || v.ValueFrom.ComponentVarRef.PodNames == nil {
			continue
		}
		if err := r.fromV1VarsPodNames(v); err != nil {
			return err
		}
	}
	if cmpd.Spec.UpdateConcurrency != nil {
		r.Spec.UpdateStrategy = (*UpdateStrategy)(cmpd.Spec.UpdateConcurrency)
	}
	r.fromV1LifecycleActions(cmpd)
	return nil
}

func (r *ComponentDefinition) toV1VarsPodNames(v EnvVar, cmpd *appsv1.ComponentDefinition) error {
	for i, vv := range cmpd.Spec.Vars {
		if vv.Name == v.Name {
			opt := appsv1.VarOption(*v.ValueFrom.ComponentVarRef.InstanceNames)
			if cmpd.Spec.Vars[i].ValueFrom == nil {
				cmpd.Spec.Vars[i].ValueFrom = &appsv1.VarSource{}
			}
			if cmpd.Spec.Vars[i].ValueFrom.ComponentVarRef == nil {
				if err := copier.Copy(&cmpd.Spec.Vars[i].ValueFrom.ComponentVarRef.ClusterObjectReference,
					&v.ValueFrom.ComponentVarRef.ClusterObjectReference); err != nil {
					return err
				}
			}
			cmpd.Spec.Vars[i].ValueFrom.ComponentVarRef.PodNames = &opt
			break
		}
	}
	return nil
}

func (r *ComponentDefinition) fromV1VarsPodNames(v appsv1.EnvVar) error {
	for i, vv := range r.Spec.Vars {
		if vv.Name == v.Name {
			opt := VarOption(*v.ValueFrom.ComponentVarRef.PodNames)
			if r.Spec.Vars[i].ValueFrom == nil {
				r.Spec.Vars[i].ValueFrom = &VarSource{}
			}
			if r.Spec.Vars[i].ValueFrom.ComponentVarRef == nil {
				if err := copier.Copy(&r.Spec.Vars[i].ValueFrom.ComponentVarRef.ClusterObjectReference,
					&v.ValueFrom.ComponentVarRef.ClusterObjectReference); err != nil {
					return err
				}
			}
			r.Spec.Vars[i].ValueFrom.ComponentVarRef.InstanceNames = &opt
			break
		}
	}
	return nil
}

func (r *ComponentDefinition) toV1LifecycleActions(cmpd *appsv1.ComponentDefinition) {
	if r.Spec.LifecycleActions == nil {
		return
	}
	cmpd.Spec.LifecycleActions.PostProvision = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.PostProvision)
	cmpd.Spec.LifecycleActions.PreTerminate = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.PreTerminate)
	cmpd.Spec.LifecycleActions.MemberJoin = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.MemberJoin)
	cmpd.Spec.LifecycleActions.MemberLeave = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.MemberLeave)
	cmpd.Spec.LifecycleActions.Readonly = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.Readonly)
	cmpd.Spec.LifecycleActions.Readwrite = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.Readwrite)
	cmpd.Spec.LifecycleActions.DataDump = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.DataDump)
	cmpd.Spec.LifecycleActions.DataLoad = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.DataLoad)
	cmpd.Spec.LifecycleActions.Reconfigure = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.Reconfigure)
	cmpd.Spec.LifecycleActions.AccountProvision = r.toV1LifecycleActionHandler(r.Spec.LifecycleActions.AccountProvision)

	cmpd.Spec.LifecycleActions.RoleProbe = r.toV1LifecycleRoleProbe(r.Spec.LifecycleActions.RoleProbe)

	// don't convert switchover
}

func (r *ComponentDefinition) fromV1LifecycleActions(cmpd *appsv1.ComponentDefinition) {
	if cmpd.Spec.LifecycleActions == nil {
		return
	}
	r.Spec.LifecycleActions.PostProvision = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.PostProvision)
	r.Spec.LifecycleActions.PreTerminate = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.PreTerminate)
	r.Spec.LifecycleActions.MemberJoin = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.MemberJoin)
	r.Spec.LifecycleActions.MemberLeave = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.MemberLeave)
	r.Spec.LifecycleActions.Readonly = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.Readonly)
	r.Spec.LifecycleActions.Readwrite = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.Readwrite)
	r.Spec.LifecycleActions.DataDump = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.DataDump)
	r.Spec.LifecycleActions.DataLoad = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.DataLoad)
	r.Spec.LifecycleActions.Reconfigure = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.Reconfigure)
	r.Spec.LifecycleActions.AccountProvision = r.fromV1LifecycleActionHandler(cmpd.Spec.LifecycleActions.AccountProvision)

	r.Spec.LifecycleActions.RoleProbe = r.fromV1LifecycleRoleProbe(cmpd.Spec.LifecycleActions.RoleProbe)

	// don't convert switchover
}

func (r *ComponentDefinition) toV1LifecycleActionHandler(handler *LifecycleActionHandler) *appsv1.Action {
	if handler == nil || handler.CustomHandler == nil || handler.CustomHandler.Exec == nil {
		return nil
	}
	return r.toV1LifecycleAction(handler.CustomHandler)
}

func (r *ComponentDefinition) fromV1LifecycleActionHandler(action *appsv1.Action) *LifecycleActionHandler {
	if action == nil || action.Exec == nil {
		return nil
	}
	return &LifecycleActionHandler{
		CustomHandler: r.fromV1LifecycleAction(action),
	}
}

func (r *ComponentDefinition) toV1LifecycleRoleProbe(probe *RoleProbe) *appsv1.Probe {
	if probe == nil || probe.CustomHandler == nil || probe.CustomHandler.Exec == nil {
		return nil
	}
	a := r.toV1LifecycleAction(probe.CustomHandler)
	if a == nil {
		return nil
	}
	if probe.TimeoutSeconds > 0 {
		a.TimeoutSeconds = probe.TimeoutSeconds
	}
	return &appsv1.Probe{
		Action:              *a,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		PeriodSeconds:       probe.PeriodSeconds,
	}
}

func (r *ComponentDefinition) fromV1LifecycleRoleProbe(probe *appsv1.Probe) *RoleProbe {
	if probe == nil || probe.Exec == nil {
		return nil
	}
	a := r.fromV1LifecycleAction(&probe.Action)
	if a == nil {
		return nil
	}
	return &RoleProbe{
		LifecycleActionHandler: LifecycleActionHandler{
			CustomHandler: a,
		},
		TimeoutSeconds:      a.TimeoutSeconds,
		InitialDelaySeconds: probe.InitialDelaySeconds,
		PeriodSeconds:       probe.PeriodSeconds,
	}
}

func (r *ComponentDefinition) toV1LifecycleAction(action *Action) *appsv1.Action {
	if action == nil || action.Exec == nil {
		return nil
	}
	a := &appsv1.Action{
		Exec: &appsv1.ExecAction{
			Image:             action.Image,
			Env:               action.Env,
			Command:           action.Exec.Command,
			Args:              action.Exec.Args,
			TargetPodSelector: appsv1.TargetPodSelector(action.TargetPodSelector),
			MatchingKey:       action.MatchingKey,
			Container:         action.Container,
		},
		TimeoutSeconds: action.TimeoutSeconds,
	}
	if action.RetryPolicy != nil {
		a.RetryPolicy = &appsv1.RetryPolicy{
			MaxRetries:    action.RetryPolicy.MaxRetries,
			RetryInterval: action.RetryPolicy.RetryInterval,
		}
	}
	if action.PreCondition != nil {
		cond := appsv1.PreConditionType(*action.PreCondition)
		a.PreCondition = &cond
	}
	return a
}

func (r *ComponentDefinition) fromV1LifecycleAction(action *appsv1.Action) *Action {
	if action == nil || action.Exec == nil {
		return nil
	}
	a := &Action{
		Image: action.Exec.Image,
		Exec: &ExecAction{
			Command: action.Exec.Command,
			Args:    action.Exec.Args,
		},
		Env:               action.Exec.Env,
		TargetPodSelector: TargetPodSelector(action.Exec.TargetPodSelector),
		MatchingKey:       action.Exec.MatchingKey,
		Container:         action.Exec.Container,
		TimeoutSeconds:    action.TimeoutSeconds,
	}
	if action.RetryPolicy != nil {
		a.RetryPolicy = &RetryPolicy{
			MaxRetries:    action.RetryPolicy.MaxRetries,
			RetryInterval: action.RetryPolicy.RetryInterval,
		}
	}
	if action.PreCondition != nil {
		cond := PreConditionType(*action.PreCondition)
		a.PreCondition = &cond
	}
	return a
}

type componentDefinitionConverter struct {
	Monitor                   *MonitorConfig       `json:"monitor,omitempty"`
	RoleArbitrator            *RoleArbitrator      `json:"roleArbitrator,omitempty"`
	LifecycleActionSwitchover *ComponentSwitchover `json:"lifecycleActionSwitchover,omitempty"`
}
