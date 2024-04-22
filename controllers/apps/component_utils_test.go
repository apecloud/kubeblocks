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

package apps

import (
	"fmt"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

func TestIsProbeTimeout(t *testing.T) {
	podsReadyTime := &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	compDef := &appsv1alpha1.ClusterComponentDefinition{
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe:                      &appsv1alpha1.ClusterDefinitionProbe{},
			RoleProbeTimeoutAfterPodsReady: appsv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady,
		},
	}
	if !IsProbeTimeout(compDef.Probes, podsReadyTime) {
		t.Error("probe timed out should be true")
	}
}

var _ = Describe("Component Utils", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql-clusterdef-" + randomStr
		clusterVersionName = "mysql-clusterversion-" + randomStr
		clusterName        = "mysql-" + randomStr
	)

	const (
		consensusCompDefRef = "consensus"
		consensusCompName   = "consensus"
		statelessCompName   = "stateless"
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Component test", func() {
		It("Component test", func() {
			By(" init cluster, instanceSet, pods")
			_, _, cluster := testapps.InitClusterWithHybridComps(&testCtx, clusterDefName,
				clusterVersionName, clusterName, statelessCompName, "stateful", consensusCompName)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
			_ = testapps.MockInstanceSetPods(&testCtx, its, clusterName, consensusCompName)

			By("test GetClusterByObject function")
			newCluster, _ := GetClusterByObject(ctx, k8sClient, its)
			Expect(newCluster != nil).Should(BeTrue())

			By("test getObjectListByComponentName function")
			itsList := &workloads.InstanceSetList{}
			_ = component.GetObjectListByComponentName(ctx, k8sClient, *cluster, itsList, consensusCompName)
			Expect(len(itsList.Items) > 0).Should(BeTrue())

			By("test getObjectListByCustomLabels function")
			itsList = &workloads.InstanceSetList{}
			matchLabel := constant.GetComponentWellKnownLabels(cluster.Name, consensusCompName)
			_ = getObjectListByCustomLabels(ctx, k8sClient, *cluster, itsList, client.MatchingLabels(matchLabel))
			Expect(len(itsList.Items) > 0).Should(BeTrue())

			By("test GetComponentStsMinReadySeconds")
			minReadySeconds, _ := component.GetComponentMinReadySeconds(ctx, k8sClient, *cluster, consensusCompName)
			Expect(minReadySeconds).To(Equal(int32(0)))
		})
	})

	Context("test mergeServiceAnnotations", func() {
		It("test sync pod spec default values set by k8s", func() {
			var (
				clusterName = "cluster"
				compName    = "component"
				podName     = "pod"
				role        = "leader"
				mode        = "ReadWrite"
			)
			pod := testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterName, compName, podName, role, mode)
			ppod := testapps.NewPodFactory(testCtx.DefaultNamespace, "pod").
				SetOwnerReferences("apps/v1", constant.StatefulSetKind, nil).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				AddRoleLabel(role).
				AddConsensusSetAccessModeLabel(mode).
				AddControllerRevisionHashLabel("").
				AddVolume(corev1.Volume{
					Name: testapps.DataVolumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: fmt.Sprintf("%s-%s", testapps.DataVolumeName, podName),
						},
					},
				}).
				AddContainer(corev1.Container{
					Name:  testapps.DefaultMySQLContainerName,
					Image: testapps.ApeCloudMySQLImage,
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/hello",
								Port: intstr.FromInt(1024),
							},
						},
						TimeoutSeconds:   1,
						PeriodSeconds:    1,
						FailureThreshold: 1,
					},
					StartupProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(1024),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: testapps.DataVolumeName, MountPath: "/test"},
					},
				}).
				GetObject()
			controllerutil.ResolvePodSpecDefaultFields(pod.Spec, &ppod.Spec)
			Expect(reflect.DeepEqual(pod.Spec, ppod.Spec)).Should(BeTrue())
		})
	})
})
