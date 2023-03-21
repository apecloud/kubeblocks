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

package util

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("util", func() {
	It("Get home dir", func() {
		home, err := GetCliHomeDir()
		Expect(len(home) > 0).Should(BeTrue())
		Expect(err == nil).Should(BeTrue())
	})

	It("Get kubeconfig dir", func() {
		dir := GetKubeconfigDir()
		Expect(len(dir) > 0).Should(BeTrue())
	})

	It("DoWithRetry", func() {
		op := func() error {
			return fmt.Errorf("test DowithRetry")
		}
		logger := logr.New(log.NullLogSink{})
		Expect(DoWithRetry(context.TODO(), logger, op, &RetryOptions{MaxRetry: 2})).Should(HaveOccurred())
	})

	It("Config path", func() {
		path := ConfigPath("")
		Expect(len(path) == 0).Should(BeTrue())
		path = ConfigPath("test")
		Expect(len(path) > 0).Should(BeTrue())
		Expect(RemoveConfig("")).Should(HaveOccurred())
	})

	It("Print yaml", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "dataprotection.kubeblocks.io/v1alpha1",
				"kind":       "BackupJob",
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "test",
				},
				"spec": map[string]interface{}{
					"backupPolicyName": "backup-policy-demo",
					"backupType":       "full",
					"ttl":              "168h0m0s",
				},
			},
		}
		Expect(PrintObjYAML(obj)).Should(Succeed())
	})

	It("Print go template", func() {
		Expect(PrintGoTemplate(os.Stdout, `key: {{.Value}}`, struct {
			Value string
		}{"test"})).Should(Succeed())
	})

	It("Test Spinner", func() {
		spinner := Spinner(os.Stdout, "spinner test ... ")
		spinner(true)

		spinner = Spinner(os.Stdout, "spinner test ... ")
		spinner(false)
	})

	It("GetNodeByName", func() {
		nodes := []*corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		}

		testFn := func(name string) bool {
			n := GetNodeByName(nodes, name)
			return n.Name == name
		}
		Expect(testFn("test")).Should(BeTrue())
		Expect(testFn("non-exists")).Should(BeFalse())
	})

	It("GetPodStatus", func() {
		newPod := func(phase corev1.PodPhase) *corev1.Pod {
			return &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: phase,
				}}
		}

		var pods []*corev1.Pod
		for _, p := range []corev1.PodPhase{corev1.PodRunning, corev1.PodPending, corev1.PodSucceeded, corev1.PodFailed} {
			pods = append(pods, newPod(p))
		}

		r, w, s, f := GetPodStatus(pods)
		Expect(r).Should(Equal(1))
		Expect(w).Should(Equal(1))
		Expect(s).Should(Equal(1))
		Expect(f).Should(Equal(1))
	})

	It("TimeFormat", func() {
		t, _ := time.Parse(time.RFC3339, "2023-01-04T01:00:00.000Z")
		metav1Time := metav1.Time{Time: t}
		Expect(TimeFormat(&metav1Time)).Should(Equal("Jan 04,2023 01:00 UTC+0000"))
	})

	It("CheckEmpty", func() {
		res := ""
		Expect(CheckEmpty(res)).Should(Equal(types.None))
		res = "test"
		Expect(CheckEmpty(res)).Should(Equal(res))
	})

	It("BuildLabelSelectorByNames", func() {
		Expect(BuildLabelSelectorByNames("", nil)).Should(Equal(""))

		names := []string{"n1", "n2"}
		expected := fmt.Sprintf("%s in (%s)", constant.AppInstanceLabelKey, strings.Join(names, ","))
		Expect(BuildLabelSelectorByNames("", names)).Should(Equal(expected))
		Expect(BuildLabelSelectorByNames("label1", names)).Should(Equal("label1," + expected))
	})

	It("Event utils", func() {
		objs := SortEventsByLastTimestamp(testing.FakeEvents(), "")
		Expect(len(*objs)).Should(Equal(2))
		firstEvent := (*objs)[0].(*corev1.Event)
		secondEvent := (*objs)[1].(*corev1.Event)
		Expect(firstEvent.LastTimestamp.Before(&secondEvent.LastTimestamp)).Should(BeTrue())
		Expect(GetEventTimeStr(firstEvent)).Should(ContainSubstring("Jan 04,2023"))
	})

	It("Others", func() {
		if os.Getenv("TEST_GET_PUBLIC_IP") != "" {
			_, err := GetPublicIP()
			Expect(err).ShouldNot(HaveOccurred())
		}
		Expect(MakeSSHKeyPair("", "")).Should(HaveOccurred())
		Expect(SetKubeConfig("test")).Should(Succeed())
		Expect(NewFactory()).ShouldNot(BeNil())

		By("resource is empty")
		res := resource.Quantity{}
		Expect(ResourceIsEmpty(&res)).Should(BeTrue())
		res.Set(20)
		Expect(ResourceIsEmpty(&res)).Should(BeFalse())

		By("GVRToString")
		Expect(len(GVRToString(types.ClusterGVR())) > 0).Should(BeTrue())
	})

	It("IsSupportReconfigureParams", func() {
		const (
			ccName = "mysql_cc"
			testNS = "default"
		)

		configConstraintObj := testapps.NewCustomizedObj("resources/mysql-config-constraint.yaml",
			&appsv1alpha1.ConfigConstraint{}, testapps.WithNamespacedName(ccName, ""), func(cc *appsv1alpha1.ConfigConstraint) {
				if ccContext, err := testdata.GetTestDataFileContent("/cue_testdata/mysql_for_cli.cue"); err == nil {
					cc.Spec.ConfigurationSchema = &appsv1alpha1.CustomParametersValidation{
						CUE: string(ccContext),
					}
				}
			})

		tf := cmdtesting.NewTestFactory().WithNamespace(testNS)
		defer tf.Cleanup()

		Expect(appsv1alpha1.AddToScheme(scheme.Scheme)).Should(Succeed())
		mockClient := dynamicfakeclient.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, nil, configConstraintObj)
		configSpec := appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "for_test",
				TemplateRef: ccName,
				VolumeName:  "config",
			},
			ConfigConstraintRef: ccName,
		}

		type args struct {
			configSpec    appsv1alpha1.ComponentConfigSpec
			updatedParams map[string]string
		}
		tests := []struct {
			name     string
			args     args
			expected bool
		}{{
			name: "normal test",
			args: args{
				configSpec:    configSpec,
				updatedParams: testapps.WithMap("automatic_sp_privileges", "OFF", "innodb_autoinc_lock_mode", "1"),
			},
			expected: true,
		}, {
			name: "not match test",
			args: args{
				configSpec:    configSpec,
				updatedParams: testapps.WithMap("not_exist_field", "1"),
			},
			expected: false,
		}}

		for _, tt := range tests {
			Expect(IsSupportReconfigureParams(tt.args.configSpec, tt.args.updatedParams, mockClient)).Should(BeEquivalentTo(tt.expected))
		}
	})

	It("get IP location", func() {
		_, _ = getIPLocation()
	})

	It("get helm chart repo url", func() {
		Expect(GetHelmChartRepoURL()).ShouldNot(BeEmpty())
	})
})
