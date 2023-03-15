/*
Copyright ApeCloud, Inc.

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

package loadbalancer

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/cmd/loadbalancer/internal/agent"
)

var ErrCrossSubnet = errors.New("Cross subnet")

type TrafficPolicy interface {
	ChooseNode(svc *corev1.Service) (string, error)
}

type ClusterTrafficPolicy struct {
	nm agent.NodeManager
}

func (c ClusterTrafficPolicy) ChooseNode(svc *corev1.Service) (string, error) {
	annotations := svc.GetAnnotations()
	nodeIP, ok := annotations[AnnotationKeyENINodeIP]
	if ok {
		return nodeIP, nil
	}

	subnetID := annotations[AnnotationKeySubnetID]
	node, err := c.nm.ChooseSpareNode(subnetID)
	if err != nil {
		return "", errors.Wrap(err, "Failed to choose spare node")
	}
	return node.GetIP(), nil
}

type LocalTrafficPolicy struct {
	logger logr.Logger
	client client.Client
	nm     agent.NodeManager
}

func (l LocalTrafficPolicy) ChooseNode(svc *corev1.Service) (string, error) {
	ctxLog := l.logger.WithValues("svc", getObjectFullName(svc))
	matchLabels := client.MatchingLabels{}
	for k, v := range svc.Spec.Selector {
		matchLabels[k] = v
	}
	listOptions := []client.ListOption{
		matchLabels,
		client.InNamespace(svc.GetNamespace()),
	}
	pods := &corev1.PodList{}
	if err := l.client.List(context.Background(), pods, listOptions...); err != nil {
		return "", errors.Wrap(err, "Failed to list service related pods")
	}
	if len(pods.Items) == 0 {
		return "", errors.New(fmt.Sprintf("Can not find master node for service %s", getObjectFullName(svc)))
	}
	ctxLog.Info("Found master pods", "count", len(pods.Items))

	pod := l.choosePod(pods)
	if pod == nil {
		return "", errors.New("Can not find valid backend pod")
	}

	node, err := l.nm.GetNode(pod.Status.HostIP)
	if err != nil {
		return "", err
	}

	var (
		svcSubnetID  = svc.GetAnnotations()[AnnotationKeySubnetID]
		nodeSubnetID = node.GetNodeInfo().GetSubnetId()
	)
	ctxLog.Info("Choose master pod", "name", getObjectFullName(pod),
		"svc subnet id", svcSubnetID, "node subnet id", node.GetNodeInfo().GetSubnetId())
	if svcSubnetID == "" || svcSubnetID == nodeSubnetID {
		return pod.Status.HostIP, nil
	}

	return "", ErrCrossSubnet
}

func (l LocalTrafficPolicy) choosePod(pods *corev1.PodList) *corev1.Pod {
	// latest created pods have high priority
	sort.SliceStable(pods.Items, func(i, j int) bool {
		return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
	})
	for index := range pods.Items {
		pod := pods.Items[index]
		if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning {
			return &pod
		}
	}
	return nil
}

type BestEffortLocalPolicy struct {
	LocalTrafficPolicy
	ClusterTrafficPolicy
}

func (b BestEffortLocalPolicy) ChooseNode(svc *corev1.Service) (string, error) {
	result, err := b.LocalTrafficPolicy.ChooseNode(svc)
	if err == nil {
		return result, nil
	}

	if err != ErrCrossSubnet {
		return "", errors.Wrapf(err, "Failed to choose node using Local traffic policy")
	}

	b.logger.Info("Pod cross subnets, degrade to cluster traffic policy", "svc", getObjectFullName(svc))
	return b.ClusterTrafficPolicy.ChooseNode(svc)
}

func getObjectFullName(obj metav1.Object) string {
	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}
