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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("DataProtection", func() {
	const policyName = "policy"
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	var out *bytes.Buffer
	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("backup", func() {
		initClient := func(policies ...*dataprotectionv1alpha1.BackupPolicy) {
			clusterDef := testing.FakeClusterDef()
			cluster := testing.FakeCluster(testing.ClusterName, testing.Namespace)
			clusterDefLabel := map[string]string{
				constant.ClusterDefLabelKey: clusterDef.Name,
			}
			cluster.SetLabels(clusterDefLabel)
			pods := testing.FakePods(1, testing.Namespace, testing.ClusterName)
			objects := []runtime.Object{
				cluster, clusterDef, &pods.Items[0],
			}
			for _, v := range policies {
				objects = append(objects, v)
			}
			tf.FakeDynamicClient = testing.FakeDynamicClient(objects...)
		}

		It("list-backup-policy", func() {
			By("fake client")
			defaultBackupPolicy := testing.FakeBackupPolicy(policyName, testing.ClusterName)
			policy2 := testing.FakeBackupPolicy("policy1", testing.ClusterName)
			initClient(defaultBackupPolicy, policy2)

			By("test list-backup-policy cmd")
			cmd := NewListBackupPolicyCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			cmd.Run(cmd, nil)
			Expect(out.String()).Should(ContainSubstring(defaultBackupPolicy.Name))
			Expect(out.String()).Should(ContainSubstring("true"))
			Expect(len(strings.Split(strings.Trim(out.String(), "\n"), "\n"))).Should(Equal(3))
		})

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

			By("test without default backupPolicy")
			o.Name = testing.ClusterName
			o.Namespace = testing.Namespace
			initClient()
			o.Dynamic = tf.FakeDynamicClient
			Expect(o.Validate()).Should(MatchError(fmt.Errorf(`not found any backup policy for cluster "%s"`, testing.ClusterName)))

			By("test with two default backupPolicy")
			defaultBackupPolicy := testing.FakeBackupPolicy(policyName, testing.ClusterName)
			initClient(defaultBackupPolicy, testing.FakeBackupPolicy("policy2", testing.ClusterName))
			o.Dynamic = tf.FakeDynamicClient
			Expect(o.Validate()).Should(MatchError(fmt.Errorf(`cluster "%s" has multiple default backup policies`, o.Name)))

			By("test with one default backupPolicy")
			initClient(defaultBackupPolicy)
			o.Dynamic = tf.FakeDynamicClient
			Expect(o.Validate()).Should(Succeed())
		})

		It("run backup command", func() {
			defaultBackupPolicy := testing.FakeBackupPolicy(policyName, testing.ClusterName)
			initClient(defaultBackupPolicy)
			By("test with specified backupPolicy")
			cmd := NewCreateBackupCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			// must succeed otherwise exit 1 and make test fails
			_ = cmd.Flags().Set("backup-policy", defaultBackupPolicy.Name)
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
		o := ListBackupOptions{ListOptions: list.NewListOptions(tf, streams, types.BackupGVR())}
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

	It("restore", func() {
		timestamp := time.Now().Format("20060102150405")
		backupName := "backup-test-" + timestamp
		clusterName := "source-cluster-" + timestamp
		newClusterName := "new-cluster-" + timestamp
		secrets := testing.FakeSecrets(testing.Namespace, clusterName)
		clusterDef := testing.FakeClusterDef()
		cluster := testing.FakeCluster(clusterName, testing.Namespace)
		clusterDefLabel := map[string]string{
			constant.ClusterDefLabelKey: clusterDef.Name,
		}
		cluster.SetLabels(clusterDefLabel)
		backupPolicy := testing.FakeBackupPolicy("backPolicy", cluster.Name)

		pods := testing.FakePods(1, testing.Namespace, clusterName)
		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
			scheme.Scheme, &secrets.Items[0], &pods.Items[0], cluster, backupPolicy)
		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
			scheme.Scheme, &secrets.Items[0], &pods.Items[0], clusterDef, cluster, backupPolicy)
		tf.Client = &clientfake.RESTClient{}
		// create backup
		cmd := NewCreateBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		_ = cmd.Flags().Set("backup-type", "snapshot")
		_ = cmd.Flags().Set("backup-name", backupName)
		cmd.Run(nil, []string{clusterName})

		By("restore new cluster from source cluster which is not deleted")
		// mock backup is ok
		mockBackupInfo(tf.FakeDynamicClient, backupName, clusterName, nil)
		cmdRestore := NewCreateRestoreCmd(tf, streams)
		Expect(cmdRestore != nil).To(BeTrue())
		_ = cmdRestore.Flags().Set("backup", backupName)
		cmdRestore.Run(nil, []string{newClusterName})

		By("restore new cluster from source cluster which is deleted")
		// mock cluster is not lived in kubernetes
		mockBackupInfo(tf.FakeDynamicClient, backupName, "deleted-cluster", nil)
		cmdRestore.Run(nil, []string{newClusterName + "1"})

		By("run restore cmd with cluster spec.affinity=nil")
		patchCluster := []byte(`{"spec":{"affinity":null}}`)
		_, _ = tf.FakeDynamicClient.Resource(types.ClusterGVR()).Namespace(testing.Namespace).Patch(context.TODO(), clusterName,
			k8sapitypes.MergePatchType, patchCluster, metav1.PatchOptions{})
		cmdRestore.Run(nil, []string{newClusterName + "-with-nil-affinity"})
	})

	It("restore-to-time", func() {
		timestamp := time.Now().Format("20060102150405")
		backupName := "backup-test-" + timestamp
		clusterName := "source-cluster-" + timestamp
		secrets := testing.FakeSecrets(testing.Namespace, clusterName)
		clusterDef := testing.FakeClusterDef()
		cluster := testing.FakeCluster(clusterName, testing.Namespace)
		clusterDefLabel := map[string]string{
			constant.ClusterDefLabelKey: clusterDef.Name,
		}
		cluster.SetLabels(clusterDefLabel)
		backupPolicy := testing.FakeBackupPolicy("backPolicy", cluster.Name)
		backupTypeMeta := testing.FakeBackup("backup-none").TypeMeta
		backupLabels := map[string]string{
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: "test",
		}
		now := metav1.Now()
		baseBackup := testapps.NewBackupFactory(testing.Namespace, "backup-base").
			SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
			SetBackLog(now.Add(-time.Minute), now.Add(-time.Second)).
			SetLabels(backupLabels).GetObject()
		baseBackup.TypeMeta = backupTypeMeta
		baseBackup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
		incrBackup := testapps.NewBackupFactory(testing.Namespace, backupName).
			SetBackupType(dataprotectionv1alpha1.BackupTypeIncremental).
			SetBackLog(now.Add(-time.Minute), now.Add(time.Minute)).
			SetLabels(backupLabels).GetObject()
		incrBackup.TypeMeta = backupTypeMeta

		pods := testing.FakePods(1, testing.Namespace, clusterName)
		tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
			scheme.Scheme, &secrets.Items[0], &pods.Items[0], cluster, backupPolicy, baseBackup, incrBackup)
		tf.Client = &clientfake.RESTClient{}

		By("restore new cluster from source cluster which is not deleted")
		cmdRestore := NewCreateRestoreCmd(tf, streams)
		Expect(cmdRestore != nil).To(BeTrue())
		_ = cmdRestore.Flags().Set("restore-to-time", util.TimeFormatWithDuration(&now, time.Second))
		_ = cmdRestore.Flags().Set("source-cluster", clusterName)
		cmdRestore.Run(nil, []string{})
	})
})

func mockBackupInfo(dynamic dynamic.Interface, backupName, clusterName string, manifests map[string]any) {
	clusterString := fmt.Sprintf(`{"metadata":{"name":"deleted-cluster","namespace":"%s"},"spec":{"clusterDefinitionRef":"apecloud-mysql","clusterVersionRef":"ac-mysql-8.0.30","componentSpecs":[{"name":"mysql","componentDefRef":"mysql","replicas":1}]}}`, testing.Namespace)
	backupStatus := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"phase":     "Completed",
				"manifests": manifests,
			},
			"metadata": map[string]any{
				"name": backupName,
				"annotations": map[string]any{
					constant.ClusterSnapshotAnnotationKey: clusterString,
				},
				"labels": map[string]any{
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
