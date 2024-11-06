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

package operations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// switchover constants
const (
	OpsReasonForSkipSwitchover             = "SkipSwitchover"
	KBSwitchoverCandidateInstanceForAnyPod = "*"
	KBSwitchoverDoNCheckRoleChangeKey      = "DoSwitchoverAndCheckRoleChange"
)

// needDoSwitchover checks whether we need to perform a switchover.
func needDoSwitchover(ctx context.Context,
	cli client.Client,
	synthesizedComp *component.SynthesizedComponent,
	switchover *opsv1alpha1.Switchover) (bool, error) {
	// get the Pod object whose current role label is already serviceable and writable
	pod, err := getPodToPerformSwitchover(ctx, cli, synthesizedComp)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	switch switchover.InstanceName {
	case KBSwitchoverCandidateInstanceForAnyPod:
		return true, nil
	default:
		targetPod := &corev1.Pod{}
		if err := cli.Get(ctx, client.ObjectKey{Name: switchover.InstanceName, Namespace: synthesizedComp.Namespace}, targetPod); err != nil {
			if apierrors.IsNotFound(err) {
				return false, controllerutil.NewFatalError(err.Error())
			}
			return false, err
		}
		if targetPod.Labels[constant.AppInstanceLabelKey] != synthesizedComp.ClusterName || component.GetComponentNameFromObj(targetPod) != switchover.ComponentName {
			return false, controllerutil.NewFatalError(fmt.Sprintf(`the pod "%s" not belongs to the component "%s"`, switchover.InstanceName, switchover.ComponentName))
		}
		// If the current instance is already the primary, then no switchover will be performed.
		if pod.Name == switchover.InstanceName {
			return false, nil
		}
	}
	return true, nil
}

// checkPodRoleLabelConsistency checks whether the pod role label is consistent with the specified role label after switchover.
func checkPodRoleLabelConsistency(ctx context.Context,
	cli client.Reader,
	synthesizedComp component.SynthesizedComponent,
	switchover *opsv1alpha1.Switchover,
	switchoverCondition *metav1.Condition) (bool, error) {
	if switchover == nil || switchoverCondition == nil {
		return false, nil
	}
	pod, err := getPodToPerformSwitchover(ctx, cli, &synthesizedComp)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	var switchoverMessageMap map[string]SwitchoverMessage
	if err := json.Unmarshal([]byte(switchoverCondition.Message), &switchoverMessageMap); err != nil {
		return false, err
	}

	for _, switchoverMessage := range switchoverMessageMap {
		if switchoverMessage.ComponentName != synthesizedComp.Name {
			continue
		}
		switch switchoverMessage.Switchover.InstanceName {
		case KBSwitchoverCandidateInstanceForAnyPod:
			if pod.Name != switchoverMessage.OldPod {
				return true, nil
			}
		default:
			if pod.Name == switchoverMessage.Switchover.InstanceName {
				return true, nil
			}
		}
	}
	return false, nil
}

func getPodToPerformSwitchover(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent) (*corev1.Pod, error) {
	role, err := getTargetRoleName(synthesizedComp.Roles)
	if err != nil {
		return nil, err
	}
	pod, err := getPodByRole(ctx, cli, synthesizedComp, role)
	return pod, err
}

func getPodByRole(ctx context.Context, cli client.Reader, synthesizeComp *component.SynthesizedComponent, targetRole string) (*corev1.Pod, error) {
	pods, err := component.ListOwnedPodsWithRole(ctx, cli, synthesizeComp.Namespace, synthesizeComp.ClusterName, synthesizeComp.Name, targetRole)
	if err != nil {
		return nil, err
	}
	if len(pods) != 1 {
		return nil, errors.New("component pod list is empty or has more than one serviceable and writable pod")
	}
	return pods[0], nil
}

// getTargetRole returns the role on which the switchover is performed
// FIXME: the assumption that only one role supports switchover may change in the future
func getTargetRoleName(roles []appsv1.ReplicaRole) (string, error) {
	targetRole := ""
	if len(roles) == 0 {
		return targetRole, errors.New("component has no roles definition, does not support switchover")
	}
	for _, role := range roles {
		// FIXME: the assumption that only one role supports switchover may change in the future
		if role.SwitchoverBeforeUpdate {
			if targetRole != "" {
				return targetRole, errors.New("componentDefinition has more than role that needs switchover before, does not support switchover")
			}
			targetRole = role.Name
		}
	}
	return targetRole, nil
}
