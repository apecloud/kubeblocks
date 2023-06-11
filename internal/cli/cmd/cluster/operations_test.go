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

package cluster

import (
	"bytes"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("operations", func() {
	const (
		clusterName  = "cluster-ops"
		clusterName1 = "cluster-ops1"
	)
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		in      *bytes.Buffer
	)

	BeforeEach(func() {
		streams, in, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		clusterWithTwoComps := testing.FakeCluster(clusterName, testing.Namespace)
		clusterWithOneComp := clusterWithTwoComps.DeepCopy()
		clusterWithOneComp.Name = clusterName1
		clusterWithOneComp.Spec.ComponentSpecs = []appsv1alpha1.ClusterComponentSpec{
			clusterWithOneComp.Spec.ComponentSpecs[0],
		}
		clusterWithOneComp.Spec.ComponentSpecs[0].ClassDefRef = &appsv1alpha1.ClassDefRef{Class: testapps.Class1c1gName}
		classDef := testapps.NewComponentClassDefinitionFactory("custom", clusterWithOneComp.Spec.ClusterDefRef, testing.ComponentDefName).
			AddClasses(testapps.DefaultResourceConstraintName, []string{testapps.Class1c1gName}).
			GetObject()
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef(),
			testing.FakeClusterVersion(), clusterWithTwoComps, clusterWithOneComp, classDef)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	initCommonOperationOps := func(opsType appsv1alpha1.OpsType, clusterName string, hasComponentNamesFlag bool, objs ...runtime.Object) *OperationsOptions {
		o := newBaseOperationsOptions(tf, streams, opsType, hasComponentNamesFlag)
		o.Dynamic = tf.FakeDynamicClient
		o.Client = testing.FakeClientSet(objs...)
		o.Name = clusterName
		o.Namespace = testing.Namespace
		return o
	}

	getOpsName := func(opsType appsv1alpha1.OpsType, phase appsv1alpha1.OpsPhase) string {
		return strings.ToLower(string(opsType)) + "-" + strings.ToLower(string(phase))
	}

	generationOps := func(opsType appsv1alpha1.OpsType, phase appsv1alpha1.OpsPhase) *appsv1alpha1.OpsRequest {
		return &appsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getOpsName(opsType, phase),
				Namespace: testing.Namespace,
			},
			Spec: appsv1alpha1.OpsRequestSpec{
				ClusterRef: "test-cluster",
				Type:       opsType,
			},
			Status: appsv1alpha1.OpsRequestStatus{
				Phase: phase,
			},
		}

	}

	It("Upgrade Ops", func() {
		o := newBaseOperationsOptions(tf, streams, appsv1alpha1.UpgradeType, false)
		o.Dynamic = tf.FakeDynamicClient

		By("validate o.name is null")
		Expect(o.Validate()).To(MatchError(missingClusterArgErrMassage))

		By("validate upgrade when cluster-version is null")
		o.Namespace = testing.Namespace
		o.Name = clusterName
		o.OpsType = appsv1alpha1.UpgradeType
		Expect(o.Validate()).To(MatchError("missing cluster-version"))

		By("expect to validate success")
		o.ClusterVersionRef = "test-cluster-version"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())
	})

	It("VolumeExpand Ops", func() {
		compName := "replicasets"
		vctName := "data"
		persistentVolumeClaim := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%s-%d", vctName, clusterName, compName, 0),
				Namespace: testing.Namespace,
				Labels: map[string]string{
					constant.AppInstanceLabelKey:             clusterName,
					constant.VolumeClaimTemplateNameLabelKey: vctName,
					constant.KBAppComponentLabelKey:          compName,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("3Gi"),
					},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: map[corev1.ResourceName]resource.Quantity{
					"storage": resource.MustParse("1Gi"),
				},
			},
		}
		o := initCommonOperationOps(appsv1alpha1.VolumeExpansionType, clusterName, true, persistentVolumeClaim)
		By("validate volumeExpansion when components is null")
		Expect(o.Validate()).To(MatchError(`missing components, please specify the "--components" flag for multi-components cluster`))

		By("validate volumeExpansion when vct-names is null")
		o.ComponentNames = []string{compName}
		Expect(o.Validate()).To(MatchError("missing volume-claim-templates"))

		By("validate volumeExpansion when storage is null")
		o.VCTNames = []string{vctName}
		Expect(o.Validate()).To(MatchError("missing storage"))

		By("validate recovery from volume expansion failure")
		o.Storage = "2Gi"
		Expect(o.Validate()).Should(Succeed())
		Expect(o.Out.(*bytes.Buffer).String()).To(ContainSubstring("Warning: this opsRequest is a recovery action for volume expansion failure and will re-create the PersistentVolumeClaims when RECOVER_VOLUME_EXPANSION_FAILURE=false"))

		By("validate passed")
		o.Storage = "4Gi"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())
	})

	It("Vscale Ops", func() {
		o := initCommonOperationOps(appsv1alpha1.VerticalScalingType, clusterName1, true)
		By("test CompleteComponentsFlag function")
		o.ComponentNames = nil
		By("expect to auto complete components when cluster has only one component")
		Expect(o.CompleteComponentsFlag()).Should(Succeed())
		Expect(o.ComponentNames[0]).Should(Equal(testing.ComponentName))

		By("validate invalid class")
		o.Class = "class-not-exists"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(HaveOccurred())

		By("expect to validate success with class")
		o.Class = testapps.Class1c1gName
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).ShouldNot(HaveOccurred())

		By("validate invalid resource")
		o.Class = ""
		o.CPU = "100"
		o.Memory = "100Gi"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(HaveOccurred())

		By("expect to validate success with resource")
		o.Class = ""
		o.CPU = "1"
		o.Memory = "1Gi"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).ShouldNot(HaveOccurred())
	})

	It("Hscale Ops", func() {
		o := initCommonOperationOps(appsv1alpha1.HorizontalScalingType, clusterName1, true)
		By("test CompleteComponentsFlag function")
		o.ComponentNames = nil
		By("expect to auto complete components when cluster has only one component")
		Expect(o.CompleteComponentsFlag()).Should(Succeed())
		Expect(o.ComponentNames[0]).Should(Equal(testing.ComponentName))

		By("expect to Validate success")
		o.Replicas = 1
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())

		By("expect for componentNames is nil when cluster has only two component")
		o.Name = clusterName
		o.ComponentNames = nil
		Expect(o.CompleteComponentsFlag()).Should(Succeed())
		Expect(o.ComponentNames).Should(BeEmpty())
	})

	It("Restart ops", func() {
		o := initCommonOperationOps(appsv1alpha1.RestartType, clusterName, true)
		By("expect for not found error")
		o.Args = []string{clusterName + "2"}
		Expect(o.Complete())
		Expect(o.CompleteRestartOps().Error()).Should(ContainSubstring("not found"))

		By("expect for complete success")
		o.Name = clusterName
		Expect(o.CompleteRestartOps()).Should(Succeed())

		By("test Restart command")
		restartCmd := NewRestartCmd(tf, streams)
		_, _ = in.Write([]byte(clusterName + "\n"))
		done := testing.Capture()
		restartCmd.Run(restartCmd, []string{clusterName})
		capturedOutput, _ := done()
		Expect(testing.ContainExpectStrings(capturedOutput, "kbcli cluster describe-ops")).Should(BeTrue())
	})

	It("cancel ops", func() {
		By("init some opsRequests which are needed for canceling opsRequest")
		completedPhases := []appsv1alpha1.OpsPhase{appsv1alpha1.OpsCancelledPhase, appsv1alpha1.OpsSucceedPhase, appsv1alpha1.OpsFailedPhase}
		supportedOpsType := []appsv1alpha1.OpsType{appsv1alpha1.VerticalScalingType, appsv1alpha1.HorizontalScalingType}
		notSupportedOpsType := []appsv1alpha1.OpsType{appsv1alpha1.RestartType, appsv1alpha1.UpgradeType}
		processingPhases := []appsv1alpha1.OpsPhase{appsv1alpha1.OpsPendingPhase, appsv1alpha1.OpsCreatingPhase, appsv1alpha1.OpsRunningPhase}
		opsList := make([]runtime.Object, 0)
		for _, opsType := range supportedOpsType {
			for _, phase := range completedPhases {
				opsList = append(opsList, generationOps(opsType, phase))
			}
			for _, phase := range processingPhases {
				opsList = append(opsList, generationOps(opsType, phase))
			}
			// mock cancelling opsRequest
			opsList = append(opsList, generationOps(opsType, appsv1alpha1.OpsCancellingPhase))
		}

		for _, opsType := range notSupportedOpsType {
			opsList = append(opsList, generationOps(opsType, appsv1alpha1.OpsRunningPhase))
		}
		tf.FakeDynamicClient = testing.FakeDynamicClient(opsList...)

		By("expect an error for not supported phase")
		o := newBaseOperationsOptions(tf, streams, "", false)
		o.Dynamic = tf.FakeDynamicClient
		o.Namespace = testing.Namespace
		o.autoApprove = true
		for _, phase := range completedPhases {
			for _, opsType := range supportedOpsType {
				o.Name = getOpsName(opsType, phase)
				Expect(cancelOps(o).Error()).Should(Equal(fmt.Sprintf("can not cancel the opsRequest when phase is %s", phase)))
			}
		}

		By("expect an error for not supported opsType")
		for _, opsType := range notSupportedOpsType {
			o.Name = getOpsName(opsType, appsv1alpha1.OpsRunningPhase)
			Expect(cancelOps(o).Error()).Should(Equal(fmt.Sprintf("opsRequest type: %s not support cancel action", opsType)))
		}

		By("expect an error for cancelling opsRequest")
		for _, opsType := range supportedOpsType {
			o.Name = getOpsName(opsType, appsv1alpha1.OpsCancellingPhase)
			Expect(cancelOps(o).Error()).Should(Equal(fmt.Sprintf(`opsRequest "%s" is cancelling`, o.Name)))
		}

		By("expect succeed for canceling the opsRequest which is processing")
		for _, phase := range processingPhases {
			for _, opsType := range supportedOpsType {
				o.Name = getOpsName(opsType, phase)
				Expect(cancelOps(o)).Should(Succeed())
			}
		}
	})
})
