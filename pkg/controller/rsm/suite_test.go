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

package rsm

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

var (
	controller      *gomock.Controller
	k8sMock         *mocks.MockClient
	graphCli        model.GraphClient
	ctx             context.Context
	logger          logr.Logger
	transCtx        *rsmTransformContext
	transCtxForPods *rsmTransformContext
	dag             *graph.DAG
	dagForPods      *graph.DAG
	transformer     graph.Transformer
)

const (
	namespace   = "foo"
	name        = "bar"
	oldRevision = "old-revision"
	newRevision = "new-revision"
)

var (
	uid = types.UID("rsm-mock-uid")

	selectors = map[string]string{
		constant.AppInstanceLabelKey: name,
		workloadsManagedByLabelKey:   kindReplicatedStateMachine,
	}

	headlessSvcName = name + "-headless"

	roles = []workloads.ReplicaRole{
		{
			Name:       "leader",
			IsLeader:   true,
			CanVote:    true,
			AccessMode: workloads.ReadWriteMode,
		},
		{
			Name:       "follower",
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.ReadonlyMode,
		},
		{
			Name:       "logger",
			IsLeader:   false,
			CanVote:    true,
			AccessMode: workloads.NoneMode,
		},
		{
			Name:       "learner",
			IsLeader:   false,
			CanVote:    false,
			AccessMode: workloads.ReadonlyMode,
		},
	}

	roleProbe = &workloads.RoleProbe{
		CustomHandler: []workloads.Action{{Command: []string{"cmd"}}},
	}

	reconfiguration = workloads.MembershipReconfiguration{
		SwitchoverAction:  &workloads.Action{Command: []string{"cmd"}},
		MemberJoinAction:  &workloads.Action{Command: []string{"cmd"}},
		MemberLeaveAction: &workloads.Action{Command: []string{"cmd"}},
	}

	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        "foo-cluster-definition",
				constant.AppInstanceLabelKey:    "foo-cluster",
				constant.KBAppComponentLabelKey: name,
				constant.AppComponentLabelKey:   name + "def",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "svc",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromString("my-svc"),
				},
			},
		},
	}

	credential = workloads.Credential{
		Username: workloads.CredentialVar{Value: "foo"},
		Password: workloads.CredentialVar{Value: "bar"},
	}

	pod = builder.NewPodBuilder(namespace, getPodName(name, 0)).
		AddContainer(corev1.Container{
			Name:  "foo",
			Image: "bar",
			Ports: []corev1.ContainerPort{
				{
					Name:          "my-svc",
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: 12345,
				},
			},
		}).GetObject()
	template = corev1.PodTemplateSpec{
		ObjectMeta: pod.ObjectMeta,
		Spec:       pod.Spec,
	}

	observeActions = []workloads.Action{{Command: []string{"cmd"}}}

	rsm *workloads.ReplicatedStateMachine

	rsmForPods *workloads.ReplicatedStateMachine
)

func less(v1, v2 graph.Vertex) bool {
	return model.DefaultLess(v1, v2)
}

func makePodUpdateReady(newRevision string, pods ...*corev1.Pod) {
	readyCondition := corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	}
	for _, pod := range pods {
		pod.Labels[apps.StatefulSetRevisionLabel] = newRevision
		if pod.Labels[roleLabelKey] == "" {
			pod.Labels[roleLabelKey] = "learner"
		}
		pod.Status.Conditions = append(pod.Status.Conditions, readyCondition)
	}
}

func mockUnderlyingSts(rsm workloads.ReplicatedStateMachine, generation int64) *apps.StatefulSet {
	headLessSvc := buildHeadlessSvc(rsm)
	envConfig := buildEnvConfigMap(rsm)
	sts := buildSts(rsm, headLessSvc.Name, *envConfig)
	sts.Generation = generation
	sts.Status.ObservedGeneration = generation
	sts.Status.Replicas = *sts.Spec.Replicas
	sts.Status.ReadyReplicas = sts.Status.Replicas
	sts.Status.AvailableReplicas = sts.Status.ReadyReplicas
	sts.Status.UpdatedReplicas = sts.Status.ReadyReplicas
	sts.Status.UpdateRevision = rsm.Status.UpdateRevision
	return sts
}

func mockUnderlyingPods(rsm workloads.ReplicatedStateMachine) []corev1.Pod {
	podList := buildPods(rsm)
	pods := make([]corev1.Pod, len(podList))
	for i := range podList {
		pods[i] = *podList[i]
	}
	return pods
}

func mockDAGForPods() *graph.DAG {
	d := graph.NewDAG()
	graphCli.Root(d, transCtxForPods.rsmOrig, transCtxForPods.rsm, model.ActionStatusPtr())
	return d
}

func mockDAG() *graph.DAG {
	d := graph.NewDAG()
	graphCli.Root(d, transCtx.rsmOrig, transCtx.rsm, model.ActionStatusPtr())
	return d
}

func init() {
	model.AddScheme(workloads.AddToScheme)
}

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "ReplicatedStateMachine Suite")
}

var _ = BeforeSuite(func() {
	controller, k8sMock = testutil.SetupK8sMock()
	graphCli = model.NewGraphClient(k8sMock)
	ctx = context.Background()
	logger = logf.FromContext(ctx).WithValues("rsm-test", namespace)

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterSuite(func() {
	controller.Finish()
})
