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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("DataProtection", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("backup", func() {
		It("validate create backup", func() {
			By("without cluster name")
			o := &CreateBackupOptions{
				BaseOptions: create.BaseOptions{
					Dynamic:   testing.FakeDynamicClient(),
					IOStreams: streams,
				},
			}
			o.IOStreams = streams
			Expect(o.Validate()).To(MatchError("missing cluster name"))

			By("not found connection secret")
			o.Name = testing.ClusterName
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("run backup command", func() {
			cluster := testing.FakeCluster(testing.ClusterName, testing.Namespace)
			clusterDefLabel := map[string]string{
				constant.ClusterDefLabelKey: "apecloud-mysql",
			}
			cluster.SetLabels(clusterDefLabel)

			template := testing.FakeBackupPolicyTemplate()
			template.SetLabels(clusterDefLabel)

			secrets := testing.FakeSecrets(testing.Namespace, testing.ClusterName)
			tf.FakeDynamicClient = fake.NewSimpleDynamicClient(scheme.Scheme, &secrets.Items[0], cluster, template)
			cmd := NewCreateBackupCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			// must succeed otherwise exit 1 and make test fails
			_ = cmd.Flags().Set("backup-type", "snapshot")
			cmd.Run(cmd, []string{testing.ClusterName})
		})

	})

	It("delete-backup", func() {
		By("test delete-backup cmd")
		cmd := NewDeleteBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		args := []string{"test1"}
		clusterLabel := util.BuildLabelSelectorByNames("", args)

		By("test delete-backup with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.BackupGVR())
		Expect(completeForDeleteBackup(o, args)).Should(HaveOccurred())

		By("test delete-backup with cluster and force")
		o.Force = true
		Expect(completeForDeleteBackup(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete-backup with cluster and force and labels")
		o.Force = true
		customLabel := "test=test"
		o.LabelSelector = customLabel
		Expect(completeForDeleteBackup(o, args)).Should(Succeed())
		Expect(o.LabelSelector == customLabel+","+clusterLabel).Should(BeTrue())
	})

	It("list-backup", func() {
		cmd := NewListBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		By("test list-backup cmd with no backup")
		tf.FakeDynamicClient = testing.FakeDynamicClient()
		o := list.NewListOptions(tf, streams, types.BackupGVR())
		Expect(printBackupList(o)).Should(Succeed())
		Expect(o.ErrOut.(*bytes.Buffer).String()).Should(ContainSubstring("No backups found"))

		By("test list-backup")
		backup1 := testing.FakeBackup("test1")
		backup1.Labels = map[string]string{
			constant.AppInstanceLabelKey: "apecloud-mysql",
		}
		tf.FakeDynamicClient = testing.FakeDynamicClient(backup1)
		Expect(printBackupList(o)).Should(Succeed())
		Expect(o.Out.(*bytes.Buffer).String()).Should(ContainSubstring("test1"))
		Expect(o.Out.(*bytes.Buffer).String()).Should(ContainSubstring("apecloud-mysql (deleted)"))
	})

	It("delete-restore", func() {
		By("test delete-restore cmd")
		cmd := NewDeleteRestoreCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		args := []string{"test1"}
		clusterLabel := util.BuildLabelSelectorByNames("", args)

		By("test delete-restore with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.BackupGVR())
		Expect(completeForDeleteRestore(o, args)).Should(HaveOccurred())

		By("test delete-restore with cluster and force")
		o.Force = true
		Expect(completeForDeleteRestore(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete-restore with cluster and force and labels")
		o.Force = true
		customLabel := "test=test"
		o.LabelSelector = customLabel
		Expect(completeForDeleteRestore(o, args)).Should(Succeed())
		Expect(o.LabelSelector == customLabel+","+clusterLabel).Should(BeTrue())
	})

	It("list-restore", func() {
		cmd := NewListRestoreCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("restore", func() {
		timestamp := time.Now().Format("20060102150405")
		backupName := "backup-test-" + timestamp
		clusterName := "source-cluster-" + timestamp
		newClusterName := "new-cluster-" + timestamp
		secrets := testing.FakeSecrets(testing.Namespace, clusterName)
		clusterDefLabel := map[string]string{
			constant.ClusterDefLabelKey: "apecloud-mysql",
		}

		cluster := testing.FakeCluster(clusterName, testing.Namespace)
		cluster.SetLabels(clusterDefLabel)

		template := testing.FakeBackupPolicyTemplate()
		template.SetLabels(clusterDefLabel)

		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(scheme.Scheme, &secrets.Items[0], cluster, template)
		// create backup
		cmd := NewCreateBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		_ = cmd.Flags().Set("backup-type", "snapshot")
		_ = cmd.Flags().Set("backup-name", backupName)
		cmd.Run(nil, []string{clusterName})

		By("restore new cluster from source cluster which is not deleted")
		// mock backup is ok
		mockBackupInfo(tf.FakeDynamicClient, backupName, clusterName)
		cmdRestore := NewCreateRestoreCmd(tf, streams)
		Expect(cmdRestore != nil).To(BeTrue())
		_ = cmdRestore.Flags().Set("backup", backupName)
		cmdRestore.Run(nil, []string{newClusterName})

		By("restore new cluster from source cluster which is deleted")
		// mock cluster is not lived in kubernetes
		mockBackupInfo(tf.FakeDynamicClient, backupName, "deleted-cluster")
		cmdRestore.Run(nil, []string{newClusterName + "1"})

		By("run restore cmd with cluster spec.affinity=nil")
		patchCluster := []byte(`{"spec":{"affinity":null}}`)
		_, _ = tf.FakeDynamicClient.Resource(types.ClusterGVR()).Namespace(testing.Namespace).Patch(context.TODO(), clusterName,
			k8sapitypes.MergePatchType, patchCluster, metav1.PatchOptions{})
		cmdRestore.Run(nil, []string{newClusterName + "-with-nil-affinity"})
	})
})

func mockBackupInfo(dynamic dynamic.Interface, backupName, clusterName string) {
	clusterString := fmt.Sprintf(`{"metadata":{"name":"deleted-cluster","namespace":"%s"},"spec":{"clusterDefinitionRef":"apecloud-mysql","clusterVersionRef":"ac-mysql-8.0.30","componentSpecs":[{"name":"mysql","componentDefRef":"mysql","replicas":1}]}}`, testing.Namespace)
	backupStatus := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Completed",
			},
			"metadata": map[string]interface{}{
				"name": backupName,
				"annotations": map[string]interface{}{
					constant.ClusterSnapshotAnnotationKey: clusterString,
				},
				"labels": map[string]interface{}{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: "test",
				},
			},
		},
	}
	_, err := dynamic.Resource(types.BackupGVR()).Namespace(testing.Namespace).UpdateStatus(context.TODO(),
		backupStatus, metav1.UpdateOptions{})
	Expect(err).Should(Succeed())
}
