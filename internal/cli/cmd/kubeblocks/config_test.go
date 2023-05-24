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
	"github.com/apecloud/kubeblocks/internal/cli/printer"

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
					{
						Name:  "VOLUMESNAPSHOT",
						Value: "true",
					},
				},
			},
		}
		return deploy
	}

	mockHelmConfig := func() map[string]interface{} {
		return map[string]interface{}{
			"image": map[string]interface{}{
				"tag": "",
				"tools": map[string]interface{}{
					"repository": "apecloud/kubeblocks-tools",
				},
				"imagePullSecrets": []interface{}{},
				"pullPolicy":       "IfNotPresent",
				"registry":         "registry.cn-hangzhou.aliyuncs.com",
				"repository":       "apecloud/kubeblocks",
			},
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
			"serviceAccount": map[string]interface{}{
				"create":      true,
				"name":        "",
				"annotations": map[string]interface{}{},
			},
			"securityContext": map[string]interface{}{
				"allowPrivilegeEscalation": false,
				"capabilities": map[string]interface{}{
					"drop": []interface{}{
						"ALL",
					},
				},
			},
			"podSecurityContext": map[string]interface{}{
				"runAsNonRoot": true,
			},
			"service": map[string]interface{}{
				"port": 9999,
				"type": "ClusterIP",
			},
			"serviceMonitor": map[string]interface{}{
				"port":    8080,
				"enabled": false,
			},
			"Resources": nil,
			"autoscaling": map[string]interface{}{
				"maxReplicas":                    100,
				"minReplicas":                    1,
				"targetCPUUtilizationPercentage": 80,
				"enabled":                        false,
			},
			"nodeSelector": map[string]interface{}{},
			"affinity": map[string]interface{}{
				"nodeAffinity": map[string]interface{}{
					"preferredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
						"preference": map[string]interface{}{
							"matchExpressions": []interface{}{
								map[string]interface{}{
									"operator": "In",
									"values": []interface{}{
										"true",
									},
									"key": "kb-controller",
								},
							},
						},
					},
				},
			},
			"dataPlane": "",
			"admissionWebhooks": map[string]interface{}{
				"createSelfSignedCert": true,
				"enabled":              false,
				"ignoreReplicasCheck":  false,
			},
			"dataProtection": map[string]interface{}{
				"backupPVCCreatePolicy":      "",
				"backupPVCInitCapacity":      "",
				"backupPVCName":              "",
				"backupPVCStorageClassName":  "",
				"backupPVConfigMapName":      "",
				"backupPVConfigMapNamespace": "",
				"enableVolumeSnapshot":       false,
			},
			"addonController": map[string]interface{}{
				"jobImagePullPolicy": "IfNotPresent",
				"jobTTL":             "5m",
				"enabled":            true,
			},
			"topologySpreadConstraints": []interface{}{},
			"tolerations": map[string]interface{}{
				"tolerations": []interface{}{
					map[string]interface{}{
						"effect":   "NoSchedule",
						"key":      "kb-controller",
						"operator": "Equal",
						"value":    true,
					},
				},
			},
			"priorityClassName": nil,
			"nameOverride":      "",
			"fullnameOverride±": "",
			"dnsPolicy":         "ClusterFirst",
			"replicaCount":      1,
			"hostNetwork":       false,
			"keepAddons":        false,
		}
	}

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
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
		Expect(o.PreCheck()).Should(Succeed())
	})
	Context("run describe config cmd", func() {
		var output printer.Format
		var configs map[string]interface{}

		BeforeEach(func() {
			configs = mockHelmConfig()
		})

		It("describe-config --output table/wide", func() {
			output = printer.Table
			err := describeConfig(configs, output, streams.Out)
			Expect(err).Should(Succeed())
			expect := `KEY                                          VALUE                                                                                                                                                 
image.tag                                    ""                                                                                                                                                    
image.tools                                  {"repository":"apecloud/kubeblocks-tools"}                                                                                                            
image.imagePullSecrets                       []                                                                                                                                                    
image.pullPolicy                             "IfNotPresent"                                                                                                                                        
image.registry                               "registry.cn-hangzhou.aliyuncs.com"                                                                                                                   
image.repository                             "apecloud/kubeblocks"                                                                                                                                 
updateStrategy.rollingUpdate                 {"maxSurge":1,"maxUnavailable":"40%"}                                                                                                                 
updateStrategy.type                          "RollingUpdate"                                                                                                                                       
podDisruptionBudget.minAvailable             1                                                                                                                                                     
loggerSettings.developmentMode               false                                                                                                                                                 
loggerSettings.encoder                       "console"                                                                                                                                             
loggerSettings.timeEncoding                  "iso8601"                                                                                                                                             
serviceAccount.create                        true                                                                                                                                                  
serviceAccount.name                          ""                                                                                                                                                    
serviceAccount.annotations                   {}                                                                                                                                                    
securityContext.allowPrivilegeEscalation     false                                                                                                                                                 
securityContext.capabilities                 {"drop":["ALL"]}                                                                                                                                      
podSecurityContext.runAsNonRoot              true                                                                                                                                                  
service.port                                 9999                                                                                                                                                  
service.type                                 "ClusterIP"                                                                                                                                           
serviceMonitor.port                          8080                                                                                                                                                  
serviceMonitor.enabled                       false                                                                                                                                                 
Resources                                    <nil>                                                                                                                                                 
autoscaling.minReplicas                      1                                                                                                                                                     
autoscaling.targetCPUUtilizationPercentage   80                                                                                                                                                    
autoscaling.enabled                          false                                                                                                                                                 
autoscaling.maxReplicas                      100                                                                                                                                                   
affinity.nodeAffinity                        {"preferredDuringSchedulingIgnoredDuringExecution":{"preference":{"matchExpressions":[{"key":"kb-controller","operator":"In","values":["true"]}]}}}   
dataPlane                                    ""                                                                                                                                                    
admissionWebhooks.ignoreReplicasCheck        false                                                                                                                                                 
admissionWebhooks.createSelfSignedCert       true                                                                                                                                                  
admissionWebhooks.enabled                    false                                                                                                                                                 
dataProtection.backupPVConfigMapNamespace    ""                                                                                                                                                    
dataProtection.enableVolumeSnapshot          false                                                                                                                                                 
dataProtection.backupPVCCreatePolicy         ""                                                                                                                                                    
dataProtection.backupPVCInitCapacity         ""                                                                                                                                                    
dataProtection.backupPVCName                 ""                                                                                                                                                    
dataProtection.backupPVCStorageClassName     ""                                                                                                                                                    
dataProtection.backupPVConfigMapName         ""                                                                                                                                                    
addonController.enabled                      true                                                                                                                                                  
addonController.jobImagePullPolicy           "IfNotPresent"                                                                                                                                        
addonController.jobTTL                       "5m"                                                                                                                                                  
topologySpreadConstraints                    []                                                                                                                                                    
tolerations.tolerations                      [{"effect":"NoSchedule","key":"kb-controller","operator":"Equal","value":true}]                                                                       
priorityClassName                            <nil>                                                                                                                                                 
nameOverride                                 ""                                                                                                                                                    
fullnameOverride                             <nil>                                                                                                                                                 
dnsPolicy                                    "ClusterFirst"                                                                                                                                        
replicaCount                                 1                                                                                                                                                     
hostNetwork                                  false                                                                                                                                                 
keepAddons                                   false                                                                                                                                                 
`
			Expect(out.String()).Should(Equal(expect))
		})

		It("describe-config --output json", func() {
			output = printer.JSON
			err := describeConfig(configs, output, streams.Out)
			Expect(err).Should(Succeed())

		})

		It("describe-config --output yaml", func() {
			output = printer.YAML
			err := describeConfig(configs, output, streams.Out)
			Expect(err).Should(Succeed())

		})
	})

	//It("run describe config cmd", func() {
	//	o := &InstallOptions{
	//		Options: Options{
	//			IOStreams: streams,
	//			HelmCfg:   helm.NewFakeConfig(testing.Namespace),
	//			Namespace: "default",
	//			Client:    testing.FakeClientSet(mockDeploy(), mockConfigMap()),
	//		},
	//	}
	//	cmd := NewDescribeConfigCmd(tf, streams)
	//	Expect(cmd).ShouldNot(BeNil())
	//	done := testing.Capture()
	//	Expect(describeConfig(o)).Should(Succeed())
	//	capturedOutput, err := done()
	//	Expect(err).Should(Succeed())
	//	Expect(capturedOutput).Should(ContainSubstring("VOLUMESNAPSHOT=true"))
	//	Expect(capturedOutput).Should(ContainSubstring("BACKUP_PVC_NAME=test-pvc"))
	//})
})
