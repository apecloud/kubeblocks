package loadbalancer

import (
	"context"
	"fmt"

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
	return pods.Items[0].Status.HostIP, nil
}
