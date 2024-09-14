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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
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
	switchover *appsv1alpha1.Switchover) (bool, error) {
	// get the Pod object whose current role label is already serviceable and writable
	pod, err := getServiceableNWritablePod(ctx, cli, *synthesizedComp)
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
		pods, err := component.ListOwnedPods(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
		if err != nil {
			return false, err
		}
		podParent, _ := instanceset.ParseParentNameAndOrdinal(pod.Name)
		siParent, o := instanceset.ParseParentNameAndOrdinal(switchover.InstanceName)
		if podParent != siParent || o < 0 || o >= len(pods) {
			return false, errors.New("switchover.InstanceName is invalid")
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
	switchover *appsv1alpha1.Switchover,
	switchoverCondition *metav1.Condition) (bool, error) {
	if switchover == nil || switchoverCondition == nil {
		return false, nil
	}
	pod, err := getServiceableNWritablePod(ctx, cli, synthesizedComp)
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
			if pod.Name != switchoverMessage.OldPrimary {
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

// getServiceableNWritablePod returns the serviceable and writable pod of the component.
func getServiceableNWritablePod(ctx context.Context, cli client.Reader, synthesizeComp component.SynthesizedComponent) (*corev1.Pod, error) {
	if synthesizeComp.Roles == nil {
		return nil, errors.New("component does not support switchover")
	}

	targetRole := ""
	for _, role := range synthesizeComp.Roles {
		if role.Serviceable && role.Writable {
			if targetRole != "" {
				return nil, errors.New("component has more than role is serviceable and writable, does not support switchover")
			}
			targetRole = role.Name
		}
	}
	if targetRole == "" {
		return nil, errors.New("component has no role is serviceable and writable, does not support switchover")
	}

	pods, err := component.ListOwnedPodsWithRole(ctx, cli, synthesizeComp.Namespace, synthesizeComp.ClusterName, synthesizeComp.Name, targetRole)
	if err != nil {
		return nil, err
	}
	if len(pods) != 1 {
		return nil, errors.New("component pod list is empty or has more than one serviceable and writable pod")
	}
	return pods[0], nil
}
