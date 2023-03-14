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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("operations", func() {
	const ClusterName = "cluster-ops"
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		in      *bytes.Buffer
	)

	BeforeEach(func() {
		streams, in, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef(),
			testing.FakeClusterVersion(), testing.FakeCluster(ClusterName, testing.Namespace))
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("operations", func() {
		o := newBaseOperationsOptions(streams, appsv1alpha1.UpgradeType, false)
		o.Dynamic = tf.FakeDynamicClient
		By("validate o.name is null")
		Expect(o.Validate()).To(MatchError(missingClusterArgErrMassage))

		By("validate upgrade when cluster-version is null")
		o.Namespace = testing.Namespace
		o.Name = ClusterName
		o.OpsType = appsv1alpha1.UpgradeType
		Expect(o.Validate()).To(MatchError("missing cluster-version"))
		o.ClusterVersionRef = "test-cluster-version"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())

		By("validate volumeExpansion when components is null")
		o.HasComponentNamesFlag = true
		o.OpsType = appsv1alpha1.VolumeExpansionType
		Expect(o.Validate()).To(MatchError("missing component-names"))

		By("validate volumeExpansion when vct-names is null")
		o.ComponentNames = []string{"replicasets"}
		Expect(o.Validate()).To(MatchError("missing volume-claim-template-names"))

		By("validate volumeExpansion when storage is null")
		o.VCTNames = []string{"data"}
		Expect(o.Validate()).To(MatchError("missing storage"))
		o.Storage = "2Gi"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())

		o.Replicas = 1
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())

		By("test CompleteRestartOps function")
		inputs := buildOperationsInputs(tf, o)
		Expect(o.Complete(inputs, []string{ClusterName}))
		o.ComponentNames = nil
		o.Namespace = "default"
		Expect(o.CompleteRestartOps().Error()).Should(ContainSubstring("not found"))

		o.Namespace = testing.Namespace
		Expect(o.CompleteRestartOps()).Should(Succeed())

		By("test Restart command")
		restartCmd := NewRestartCmd(tf, streams)
		_, _ = in.Write([]byte(ClusterName + "\n"))
		done := testing.Capture()
		restartCmd.Run(restartCmd, []string{ClusterName})
		capturedOutput, _ := done()
		Expect(testing.ContainExpectStrings(capturedOutput, "kbcli cluster describe-ops")).Should(BeTrue())
	})

	It("check params for reconfiguring operations", func() {
		const (
			ns                 = "default"
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			clusterName        = "test-cluster"
			statefulCompType   = "replicasets"
			statefulCompName   = "mysql"
			configTplName      = "mysql-config-tpl"
			configVolumeName   = "mysql-config"
		)

		By("Create configmap and config constraint obj")
		configmap := testapps.NewCustomizedObj("resources/mysql_config_cm.yaml", &corev1.ConfigMap{}, testapps.WithNamespace(ns))
		constraint := testapps.NewCustomizedObj("resources/mysql_config_template.yaml",
			&appsv1alpha1.ConfigConstraint{})
		componentConfig := testapps.NewConfigMap(ns, cfgcore.GetComponentCfgName(clusterName, statefulCompName, configVolumeName), testapps.SetConfigMapData("my.cnf", ""))
		By("Create a clusterDefinition obj")
		clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
			AddComponent(testapps.StatefulMySQLComponent, statefulCompType).
			AddConfigTemplate(configTplName, configmap.Name, constraint.Name, ns, configVolumeName, nil).
			GetObject()
		By("Create a clusterVersion obj")
		clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponent(statefulCompType).
			GetObject()
		By("creating a cluster")
		clusterObj := testapps.NewClusterFactory(ns, clusterName,
			clusterDefObj.Name, "").
			AddComponent(statefulCompName, statefulCompType).GetObject()

		objs := []runtime.Object{configmap, constraint, clusterDefObj, clusterVersionObj, clusterObj, componentConfig}
		ttf, o := NewFakeOperationsOptions(ns, clusterObj.Name, appsv1alpha1.ReconfiguringType, objs...)
		defer ttf.Cleanup()
		o.ComponentNames = []string{"replicasets", "proxy"}
		By("validate reconfiguring when multi components")
		Expect(o.Validate()).To(MatchError("reconfiguring only support one component."))

		By("validate reconfiguring parameter")
		o.ComponentNames = []string{statefulCompName}
		Expect(o.parseUpdatedParams().Error()).To(ContainSubstring("reconfiguring required configure file or updated parameters"))
		o.Parameters = []string{"abcd"}

		Expect(o.parseUpdatedParams().Error()).To(ContainSubstring("updated parameter format"))
		o.Parameters = []string{"abcd=test"}
		o.CfgTemplateName = configTplName
		o.IOStreams = streams
		in.Write([]byte(o.Name + "\n"))

		Expect(o.Validate()).Should(Succeed())
	})

	It("list and delete operations", func() {
		clusterName := "wesql"
		args := []string{clusterName}
		clusterLabel := util.BuildLabelSelectorByNames("", args)
		testLabel := "kubeblocks.io/test=test"

		By("test delete OpsRequest with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.OpsGVR())
		Expect(completeForDeleteOps(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with cluster and custom label")
		o.LabelSelector = testLabel
		Expect(completeForDeleteOps(o, args)).Should(Succeed())
		Expect(o.LabelSelector == testLabel+","+clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with name")
		o.Names = []string{"test1"}
		Expect(completeForDeleteOps(o, nil)).Should(Succeed())
		Expect(len(o.ConfirmedNames)).Should(Equal(1))
	})

})
