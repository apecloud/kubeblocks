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

package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	joinMemberPodFQDNVar  = "KB_JOIN_MEMBER_POD_FQDN"
	joinMemberPodNameVar  = "KB_JOIN_MEMBER_POD_NAME"
	leaveMemberPodFQDNVar = "KB_LEAVE_MEMBER_POD_FQDN"
	leaveMemberPodNameVar = "KB_LEAVE_MEMBER_POD_NAME"
)

type memberJoin struct {
	synthesizedComp *component.SynthesizedComponent
	pod             *corev1.Pod
}

var _ lifecycleAction = &memberJoin{}

const MemberJoinName = "memberJoin"

func (a *memberJoin) name() string {
	return MemberJoinName
}

func (a *memberJoin) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following environment variables:
	//
	// - KB_JOIN_MEMBER_POD_FQDN: The pod FQDN of the replica being added to the group.
	// - KB_JOIN_MEMBER_POD_NAME: The pod name of the replica being added to the group.
	return map[string]string{
		joinMemberPodFQDNVar: component.PodFQDN(a.synthesizedComp.Namespace, a.synthesizedComp.FullCompName, a.pod.Name),
		joinMemberPodNameVar: a.pod.Name,
	}, nil
}

type memberLeave struct {
	synthesizedComp *component.SynthesizedComponent
	pod             *corev1.Pod
}

var _ lifecycleAction = &memberLeave{}

const MemberLeaveName = "memberLeave"

func (a *memberLeave) name() string {
	return MemberLeaveName
}

func (a *memberLeave) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following environment variables:
	//
	// - KB_LEAVE_MEMBER_POD_FQDN: The pod name of the replica being removed from the group.
	// - KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.
	return map[string]string{
		leaveMemberPodFQDNVar: component.PodFQDN(a.synthesizedComp.Namespace, a.synthesizedComp.FullCompName, a.pod.Name),
		leaveMemberPodNameVar: a.pod.Name,
	}, nil
}

// todo should modify request?
const RetryMemberLeaveByAnotherPodParamCli = "retryMemberLeaveByAnotherPodParamCli"

func retryMemberLeaveByAnotherPod(ctx context.Context, allPods []*corev1.Pod, podToLeave *corev1.Pod, req *proto.ActionRequest, cli *client.Client) error {
	podToRetry, err := getPodToRetryForMemberLeave(allPods, podToLeave, cli)
	if err != nil {
		return err
	}

	podCli, cliErr := kbacli.NewClient(*podToRetry)
	if cliErr != nil {
		return cliErr
	}

	if podCli == nil {
		return fmt.Errorf("podToRetry %s cli is err", podToRetry.Name)
	}

	_, actionErr := podCli.CallAction(ctx, *req)

	return actionErr
}

// todo is hardcode ok for namespace?
const MemberLeaveCMNamespace = "kb-system"
const MemberLeaveCMName = "kb-component-member-leave"

func getPodToRetryForMemberLeave(allPods []*corev1.Pod, podToLeave *corev1.Pod, cli *client.Client) (*corev1.Pod, error) {
	retryPodName := ""

	cmData, err := GetConfigMapData(*cli, MemberLeaveCMNamespace, MemberLeaveCMName)
	if err != nil {
		return nil, err
	}

	retryPodName, exists := cmData[podToLeave.Name]

	if !exists {
		idx := HashAndModSHA256(podToLeave.Name, len(allPods))
		cmData[podToLeave.Name] = allPods[idx].Name
		retryPodName = allPods[idx].Name
		err2 := SetConfigMapData(*cli, MemberLeaveCMNamespace, MemberLeaveCMName, cmData)
		if err2 != nil {
			return nil, err2
		}
	}

	for _, pod := range allPods {
		if pod.Name == retryPodName {
			return pod, nil
		}
	}

	return nil, errors.New("can't get pod to retry for member leave")
}

var leaveMemberCMMutex = &sync.Mutex{}

// todo move this code to other place
// todo should we use a global mutex?
func CreateConfigMap(ctx context.Context, cli client.Client, namespace string, name string, data map[string]string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	err := cli.Create(ctx, configMap)
	if err != nil {
		return fmt.Errorf("failed to create configmap %s/%s: %v", namespace, name, err)
	}

	fmt.Printf("ConfigMap %s/%s created successfully.\n", namespace, name)
	return nil
}

// todo move this code to other place
// todo should we use a global mutex?
func GetConfigMapData(cli client.Client, namespace, name string) (map[string]string, error) {
	leaveMemberCMMutex.Lock()
	defer leaveMemberCMMutex.Unlock()
	ctx := context.Background()
	configMap := &corev1.ConfigMap{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configMap)
	if err != nil {
		return nil, err
	}
	return configMap.Data, nil
}

// todo move this code to other place
// todo should we use a global mutex?
func SetConfigMapData(cli client.Client, namespace, name string, data map[string]string) error {
	leaveMemberCMMutex.Lock()
	defer leaveMemberCMMutex.Unlock()
	ctx := context.Background()
	configMap := &corev1.ConfigMap{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configMap)
	if err != nil {
		return err
	}

	configMap.Data = data
	return cli.Update(ctx, configMap)
}

func InitConfigMapForMemberLeave(ctx context.Context, cli client.Client) error {
	err := CreateConfigMap(ctx, cli, MemberLeaveCMNamespace, MemberLeaveCMName, map[string]string{})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create configmap %s/%s: %v", MemberLeaveCMNamespace, MemberLeaveCMName, err)
	}
	return nil
}

// todo move this code to other place
func HashAndModSHA256(a string, b int) int {
	hash := sha256.Sum256([]byte(a))
	hashValue := binary.BigEndian.Uint64(hash[:8])
	return int(hashValue % uint64(b))
}

// todo move this code to other place
func HashAnyToString(input any) (string, error) {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input to JSON: %w", err)
	}
	hash := sha256.Sum256(jsonData)

	return hex.EncodeToString(hash[:]), nil
}
