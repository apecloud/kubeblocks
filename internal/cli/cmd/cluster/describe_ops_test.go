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

package cluster

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Expose", func() {
	const (
		namespace          = "test"
		opsName            = "test-ops"
		componentName      = "test_stateful"
		componentName1     = "test_stateless"
		clusterVersionName = "test-cluster-0.1"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = clitesting.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				mapping := map[string]*http.Response{
					"/api/v1/nodes/" + clitesting.NodeName: httpResp(clitesting.FakeNode()),
					urlPrefix + "/services":                httpResp(&corev1.ServiceList{}),
					urlPrefix + "/events":                  httpResp(&corev1.EventList{}),
					// urlPrefix + "/pods":                 httpResp(pods),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("describe", func() {
		cmd := NewDescribeOpsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("complete", func() {
		o := newDescribeOpsOptions(tf, streams)
		Expect(o.complete(nil).Error()).Should(Equal("OpsRequest name should be specified"))
		Expect(o.complete([]string{opsName})).Should(Succeed())
		Expect(o.names).Should(Equal([]string{opsName}))
		Expect(o.client).ShouldNot(BeNil())
		Expect(o.dynamic).ShouldNot(BeNil())
		Expect(o.namespace).Should(Equal(namespace))
	})

	generateOpsObject := func(opsName string, opsType appsv1alpha1.OpsType) *appsv1alpha1.OpsRequest {
		return &appsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      opsName,
				Namespace: namespace,
			},
			Spec: appsv1alpha1.OpsRequestSpec{
				ClusterRef: "test-cluster",
				Type:       opsType,
			},
		}
	}

	describeOps := func(opsType appsv1alpha1.OpsType, completeOps func(ops *appsv1alpha1.OpsRequest)) {
		randomStr := clitesting.GetRandomStr()
		ops := generateOpsObject(opsName+randomStr, opsType)
		completeOps(ops)
		tf.FakeDynamicClient = clitesting.FakeDynamicClient(ops)
		o := newDescribeOpsOptions(tf, streams)
		Expect(o.complete([]string{opsName + randomStr})).Should(Succeed())
		Expect(o.run()).Should(Succeed())
	}

	fakeOpsStatusAndProgress := func() appsv1alpha1.OpsRequestStatus {
		objectKey := "Pod/test-pod-wessxd"
		objectKey1 := "Pod/test-pod-xsdfwe"
		return appsv1alpha1.OpsRequestStatus{
			StartTimestamp:      metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			CompletionTimestamp: metav1.NewTime(time.Now()),
			Progress:            "1/2",
			Phase:               appsv1alpha1.OpsFailedPhase,
			Components: map[string]appsv1alpha1.OpsRequestComponentStatus{
				componentName: {
					Phase: appsv1alpha1.FailedPhase,
					ProgressDetails: []appsv1alpha1.ProgressStatusDetail{
						{
							ObjectKey: objectKey,
							Status:    appsv1alpha1.SucceedProgressStatus,
							StartTime: metav1.NewTime(time.Now().Add(-59 * time.Second)),
							EndTime:   metav1.NewTime(time.Now().Add(-39 * time.Second)),
							Message:   fmt.Sprintf("Successfully vertical scale Pod: %s in Component: %s", objectKey, componentName),
						},
						{
							ObjectKey: objectKey1,
							Status:    appsv1alpha1.FailedProgressStatus,
							StartTime: metav1.NewTime(time.Now().Add(-39 * time.Second)),
							EndTime:   metav1.NewTime(time.Now().Add(-1 * time.Second)),
							Message:   fmt.Sprintf("Failed to vertical scale Pod: %s in Component: %s", objectKey1, componentName),
						},
					},
				},
			},
			Conditions: []metav1.Condition{
				{
					Type:    "Processing",
					Reason:  "ProcessingOps",
					Status:  metav1.ConditionTrue,
					Message: "Start to process the OpsRequest.",
				},
				{
					Type:    "Failed",
					Reason:  "FailedScale",
					Status:  metav1.ConditionFalse,
					Message: "Failed to process the OpsRequest.",
				},
			},
		}
	}

	testPrintLastConfiguration := func(config appsv1alpha1.LastConfiguration,
		opsType appsv1alpha1.OpsType, expectStrings ...string) {
		o := newDescribeOpsOptions(tf, streams)
		if opsType == appsv1alpha1.UpgradeType {
			// capture stdout
			done := clitesting.Capture()
			o.printLastConfiguration(config, opsType)
			capturedOutput, err := done()
			Expect(err).Should(Succeed())
			Expect(clitesting.ContainExpectStrings(capturedOutput, expectStrings...)).Should(BeTrue())
			return
		}
		o.printLastConfiguration(config, opsType)
		out := o.Out.(*bytes.Buffer)
		Expect(clitesting.ContainExpectStrings(out.String(), expectStrings...)).Should(BeTrue())
	}

	It("run", func() {
		By("test describe Upgrade")
		describeOps(appsv1alpha1.UpgradeType, func(ops *appsv1alpha1.OpsRequest) {
			ops.Spec.Upgrade = &appsv1alpha1.Upgrade{
				ClusterVersionRef: clusterVersionName,
			}
		})

		By("test describe Restart")
		describeOps(appsv1alpha1.RestartType, func(ops *appsv1alpha1.OpsRequest) {
			ops.Spec.RestartList = []appsv1alpha1.ComponentOps{
				{ComponentName: componentName},
				{ComponentName: componentName1},
			}
		})

		By("test describe VerticalScaling")
		resourceRequirements := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"cpu":    apiresource.MustParse("100m"),
				"memory": apiresource.MustParse("200Mi"),
			},
			Limits: corev1.ResourceList{
				"cpu":    apiresource.MustParse("300m"),
				"memory": apiresource.MustParse("400Mi"),
			},
		}
		fakeVerticalScalingSpec := func() []appsv1alpha1.VerticalScaling {
			return []appsv1alpha1.VerticalScaling{
				{
					ComponentOps: appsv1alpha1.ComponentOps{
						ComponentName: componentName,
					},
					ResourceRequirements: resourceRequirements,
				},
			}
		}
		describeOps(appsv1alpha1.VerticalScalingType, func(ops *appsv1alpha1.OpsRequest) {
			ops.Spec.VerticalScalingList = fakeVerticalScalingSpec()
		})

		By("test describe HorizontalScaling")
		describeOps(appsv1alpha1.HorizontalScalingType, func(ops *appsv1alpha1.OpsRequest) {
			ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
				{
					ComponentOps: appsv1alpha1.ComponentOps{
						ComponentName: componentName,
					},
					Replicas: 1,
				},
			}
		})

		By("test describe VolumeExpansion and print OpsRequest status")
		volumeClaimTemplates := []appsv1alpha1.OpsRequestVolumeClaimTemplate{
			{
				Name:    "data",
				Storage: apiresource.MustParse("2Gi"),
			},
			{
				Name:    "log",
				Storage: apiresource.MustParse("4Gi"),
			},
		}
		describeOps(appsv1alpha1.VolumeExpansionType, func(ops *appsv1alpha1.OpsRequest) {
			ops.Spec.VolumeExpansionList = []appsv1alpha1.VolumeExpansion{
				{
					ComponentOps: appsv1alpha1.ComponentOps{
						ComponentName: componentName,
					},
					VolumeClaimTemplates: volumeClaimTemplates,
				},
			}
		})

		By("test printing OpsRequest status and conditions")
		describeOps(appsv1alpha1.VerticalScalingType, func(ops *appsv1alpha1.OpsRequest) {
			ops.Spec.VerticalScalingList = fakeVerticalScalingSpec()
			ops.Status = fakeOpsStatusAndProgress()
		})

		By("test printing OpsRequest last configuration")
		testPrintLastConfiguration(appsv1alpha1.LastConfiguration{
			ClusterVersionRef: clusterVersionName,
		}, appsv1alpha1.UpgradeType, "\nLast Configuration",
			fmt.Sprintf("%-20s%s", "Cluster Version:", clusterVersionName+"\n"))

		By("test verticalScaling last configuration")
		testPrintLastConfiguration(appsv1alpha1.LastConfiguration{
			Components: map[string]appsv1alpha1.LastComponentConfiguration{
				componentName: {
					ResourceRequirements: resourceRequirements,
				},
			},
		}, appsv1alpha1.VerticalScalingType, "100m", "200Mi", "300m", "400Mi",
			"REQUEST-CPU", "REQUEST-MEMORY", "LIMIT-CPU", "LIMIT-MEMORY")

		By("test HorizontalScaling last configuration")
		replicas := int32(2)
		testPrintLastConfiguration(appsv1alpha1.LastConfiguration{
			Components: map[string]appsv1alpha1.LastComponentConfiguration{
				componentName: {
					Replicas: &replicas,
				},
			},
		}, appsv1alpha1.HorizontalScalingType, "COMPONENT", "REPLICAS", componentName, "2")

		By("test VolumeExpansion last configuration")
		testPrintLastConfiguration(appsv1alpha1.LastConfiguration{
			Components: map[string]appsv1alpha1.LastComponentConfiguration{
				componentName: {
					VolumeClaimTemplates: volumeClaimTemplates,
				},
			},
		}, appsv1alpha1.VolumeExpansionType, "VOLUME-CLAIM-TEMPLATE", "STORAGE", "data", "2Gi", "log")

	})
})
