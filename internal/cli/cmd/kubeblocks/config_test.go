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
	"bytes"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"helm.sh/helm/v3/pkg/cli/values"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("backupconfig", func() {
	var streams genericiooptions.IOStreams
	var tf *cmdtesting.TestFactory
	var o *InstallOptions
	var out *bytes.Buffer

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
				},
			},
		}
		return deploy
	}

	mockHelmConfig := func(release string, opt *Options) (map[string]interface{}, error) {
		values := map[string]interface{}{
			"updateStrategy": map[string]interface{}{
				"rollingUpdate": map[string]interface{}{
					"maxSurge":       1,
					"maxUnavailable": "40%",
				},
				"type": "RollingUpdate",
			},
			"podDisruptionBudget": map[string]interface{}{
				"minAvailable": 1,
			},
			"loggerSettings": map[string]interface{}{
				"developmentMode": false,
				"encoder":         "console",
				"timeEncoding":    "iso8601",
			},
			"cloudProvider": map[string]interface{}{
				"accessKey": "testAK",
			},
			"priorityClassName": nil,
			"nameOverride":      "",
			"fullnameOverride":  "",
			"dnsPolicy":         "ClusterFirst",
			"replicaCount":      1,
			"hostNetwork":       false,
			"keepAddons":        false,
		}
		for _, key := range sensitiveValues {
			sp := strings.Split(key, ".")
			rootKey := sp[0]
			if node, ok := values[rootKey]; ok {
				encryptNodeData(values, node, sp, 0)
			}
		}
		return values, nil
	}

	BeforeEach(func() {
		streams, _, out, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
		o = &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(testing.Namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(mockDeploy()),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version:   version.DefaultKubeBlocksVersion,
			ValueOpts: values.Options{Values: []string{"snapshot-controller.enabled=true"}},
		}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("run config cmd", func() {
		cmd := NewConfigCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(o.PreCheck()).Should(HaveOccurred())
	})

	It("pruningConfigResults test, and expected success", func() {
		configs := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}
		tests := []struct {
			configs       map[string]interface{}
			showAllConfig bool
			filterConfig  string
			keyWhiteList  []string
			results       map[string]interface{}
		}{
			{
				configs,
				true,
				"",
				keyWhiteList,
				configs,
			}, {
				configs,
				false,
				"key1",
				keyWhiteList,
				map[string]interface{}{
					"key1": "value1",
				},
			}, {
				configs,
				false,
				"",
				[]string{"key2"},
				map[string]interface{}{
					"key2": "value2",
				},
			}, {
				configs,
				false,
				"",
				[]string{},
				map[string]interface{}{},
			}}
		Eventually(func(g Gomega) {
			for _, t := range tests {
				showAllConfig = t.showAllConfig
				filterConfig = t.filterConfig
				keyWhiteList = t.keyWhiteList
				g.Expect(pruningConfigResults(t.configs)).Should(Equal(t.results))
			}
		}).Should(Succeed())
	})

	Context("run describe config cmd", func() {
		var output printer.Format

		It("describe-config --output table/wide", func() {
			output = printer.Table
			err := describeConfig(o, output, mockHelmConfig)
			Expect(err).Should(Succeed())
			expect := `KEY                                VALUE                                   
cloudProvider.accessKey            "******"                                
dnsPolicy                          "ClusterFirst"                          
fullnameOverride                   ""                                      
hostNetwork                        false                                   
keepAddons                         false                                   
loggerSettings.developmentMode     false                                   
loggerSettings.encoder             "console"                               
loggerSettings.timeEncoding        "iso8601"                               
nameOverride                       ""                                      
podDisruptionBudget.minAvailable   1                                       
priorityClassName                  <nil>                                   
replicaCount                       1                                       
updateStrategy.rollingUpdate       {"maxSurge":1,"maxUnavailable":"40%"}   
updateStrategy.type                "RollingUpdate"                         
`
			Expect(out.String()).Should(Equal(expect))
		})

		It("describe-config --output json", func() {
			output = printer.JSON
			expect := `{
  "cloudProvider": {
    "accessKey": "******"
  },
  "dnsPolicy": "ClusterFirst",
  "fullnameOverride": "",
  "hostNetwork": false,
  "keepAddons": false,
  "loggerSettings": {
    "developmentMode": false,
    "encoder": "console",
    "timeEncoding": "iso8601"
  },
  "nameOverride": "",
  "podDisruptionBudget": {
    "minAvailable": 1
  },
  "priorityClassName": null,
  "replicaCount": 1,
  "updateStrategy": {
    "rollingUpdate": {
      "maxSurge": 1,
      "maxUnavailable": "40%"
    },
    "type": "RollingUpdate"
  }
}
`
			err := describeConfig(o, output, mockHelmConfig)
			Expect(err).Should(Succeed())
			Expect(out.String()).Should(Equal(expect))
		})

		It("describe-config --output yaml", func() {
			output = printer.YAML
			expect := `cloudProvider:
  accessKey: '******'
dnsPolicy: ClusterFirst
fullnameOverride: ""
hostNetwork: false
keepAddons: false
loggerSettings:
  developmentMode: false
  encoder: console
  timeEncoding: iso8601
nameOverride: ""
podDisruptionBudget:
  minAvailable: 1
priorityClassName: null
replicaCount: 1
updateStrategy:
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 40%
  type: RollingUpdate
`
			err := describeConfig(o, output, mockHelmConfig)
			Expect(err).Should(Succeed())
			Expect(out.String()).Should(Equal(expect))
		})
	})
})
