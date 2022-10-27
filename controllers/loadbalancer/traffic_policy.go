/*
Copyright ApeCloud Inc.

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/loadbalancer/agent"
)

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

	subnetId := annotations[AnnotationKeySubnetId]
	node, err := c.nm.ChooseSpareNode(subnetId)
	if err != nil {
		return "", errors.Wrap(err, "Failed to choose spare node")
	}
	return node.GetIP(), nil
}

type LocalTrafficPolicy struct {
	logger logr.Logger
	client client.Client
}

func (l LocalTrafficPolicy) ChooseNode(svc *corev1.Service) (string, error) {
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
		return "", errors.New(fmt.Sprintf("Can not find master node for service %s", getServiceFullName(svc)))
	}
	l.logger.Info("Found master pods", "count", len(pods.Items))

	pod := l.choosePod(pods)
	if pod != nil {
		return pod.Status.HostIP, nil
	}
	return "", errors.New("Can not find valid backend pod")
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

func getServiceFullName(service *corev1.Service) string {
	return fmt.Sprintf("%s/%s", service.GetNamespace(), service.GetName())
}
