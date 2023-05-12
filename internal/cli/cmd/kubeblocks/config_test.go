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

package kubeblocks

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/cli/values"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("backupconfig", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	mockDeploy := func() *appsv1.Deployment {
		deploy := &appsv1.Deployment{}
		deploy.SetLabels(map[string]string{
			"app.kubernetes.io/name":    types.KubeBlocksChartName,
			"app.kubernetes.io/version": "0.3.0",
		})
		deploy.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name: "kb",
				Env: []corev1.EnvVar{
					{
						Name:  "CM_NAMESPACE",
						Value: "default",
					},
					{
						Name:  "VOLUMESNAPSHOT",
						Value: "true",
					},
				},
			},
		}
		return deploy
	}

	mockConfigMap := func() *corev1.ConfigMap {
		configmap := &corev1.ConfigMap{}
		configmap.Name = fmt.Sprintf("%s-manager-config", types.KubeBlocksChartName)
		configmap.SetLabels(map[string]string{
			"app.kubernetes.io/name": types.KubeBlocksChartName,
		})
		configmap.Data = map[string]string{
			"config.yaml": `BACKUP_PVC_NAME: "test-pvc"`,
		}
		return configmap
	}

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}

		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("run config cmd", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(testing.Namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(mockDeploy()),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version:   version.DefaultKubeBlocksVersion,
			Monitor:   true,
			ValueOpts: values.Options{Values: []string{"snapshot-controller.enabled=true"}},
		}
		cmd := NewConfigCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(o.PrecheckBeforeInstall()).Should(Succeed())
	})

	It("run describe config cmd", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(testing.Namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(mockDeploy(), mockConfigMap()),
			},
		}
		cmd := NewDescribeConfigCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		done := testing.Capture()
		Expect(describeConfig(o)).Should(Succeed())
		capturedOutput, err := done()
		Expect(err).Should(Succeed())
		Expect(capturedOutput).Should(ContainSubstring("VOLUMESNAPSHOT=true"))
		Expect(capturedOutput).Should(ContainSubstring("BACKUP_PVC_NAME=test-pvc"))
	})
})
