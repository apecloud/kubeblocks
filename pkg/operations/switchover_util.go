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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	// pod, err := getPodToPerformSwitchover(ctx, cli, synthesizedComp)
	// if err != nil {
	// 	return false, err
	// }
	// if pod == nil {
	// 	return false, nil
	// }
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
		// if pod.Name == switchover.InstanceName {
		// 	return false, nil
		// }
	}
	return true, nil
}

func roleLabelChanged(ctx context.Context,
	cli client.Reader,
	synthesizedComp component.SynthesizedComponent,
	switchover *opsv1alpha1.Switchover,
	switchoverCondition *metav1.Condition) (bool, error) {
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: switchover.InstanceName}, pod); err != nil {
		return false, fmt.Errorf("get pod %v/%v failed, err: %v", synthesizedComp.Namespace, switchover.InstanceName, err.Error())
	}
	var switchoverMessageMap map[string]SwitchoverMessage
	if err := json.Unmarshal([]byte(switchoverCondition.Message), &switchoverMessageMap); err != nil {
		return false, err
	}

	role, err := getRoleName(pod)
	if err != nil {
		return false, err
	}

	for _, switchoverMessage := range switchoverMessageMap {
		if switchoverMessage.ComponentName != synthesizedComp.Name {
			continue
		}
		if switchoverMessage.Role != role {
			return true, nil
		} else {
			return false, nil
		}
	}
	return false, errors.New("invalid switchover message")
}

func getRoleName(pod *corev1.Pod) (string, error) {
	roleName, ok := pod.Labels[constant.RoleLabelKey]
	if !ok || roleName == "" {
		return "", fmt.Errorf("pod %s/%s does not have a invalid role label", pod.Namespace, pod.Name)
	}
	return roleName, nil
}
