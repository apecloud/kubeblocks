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
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("DataProtection", func() {
	const policyName = "policy"
	const repoName = "repo"
	var streams genericiooptions.IOStreams
	var tf *cmdtesting.TestFactory
	var out *bytes.Buffer
	BeforeEach(func() {
		streams, _, out, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("backup", func() {
		initClient := func(policies ...*dpv1alpha1.BackupPolicy) {
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
			policy3 := testing.FakeBackupPolicy("policy2", testing.ClusterName)
			policy3.Namespace = "policy"
			initClient(defaultBackupPolicy, policy2, policy3)

			By("test list-backup-policy cmd")
			cmd := NewListBackupPolicyCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			cmd.Run(cmd, nil)
			Expect(out.String()).Should(ContainSubstring(defaultBackupPolicy.Name))
			Expect(out.String()).Should(ContainSubstring("true"))
			Expect(len(strings.Split(strings.Trim(out.String(), "\n"), "\n"))).Should(Equal(3))

			By("test list all namespace")
			out.Reset()
			_ = cmd.Flags().Set("all-namespaces", "true")
			cmd.Run(cmd, nil)
			fmt.Println(out.String())
			Expect(out.String()).Should(ContainSubstring(policy2.Name))
			Expect(len(strings.Split(strings.Trim(out.String(), "\n"), "\n"))).Should(Equal(4))
		})

		It("edit-backup-policy", func() {
			By("fake client")
			defaultBackupPolicy := testing.FakeBackupPolicy(policyName, testing.ClusterName)
			repo := testing.FakeBackupRepo(repoName, false)
			tf.FakeDynamicClient = testing.FakeDynamicClient(defaultBackupPolicy, repo)

			By("test edit backup policy function")
			o := editBackupPolicyOptions{Factory: tf, IOStreams: streams, GVR: types.BackupPolicyGVR()}
			Expect(o.complete([]string{policyName})).Should(Succeed())
			o.values = []string{"backupRepoName=repo"}
			Expect(o.runEditBackupPolicy()).Should(Succeed())

			By("test backup repo not exists")
			o.values = []string{"backupRepoName=repo1"}
			Expect(o.runEditBackupPolicy()).Should(MatchError(ContainSubstring(`"repo1" not found`)))

			By("test with vim editor")
			o.values = []string{}
			o.isTest = true
			Expect(o.runEditBackupPolicy()).Should(Succeed())
		})

		It("validate create backup", func() {
			By("without cluster name")
			o := &CreateBackupOptions{
				CreateOptions: create.CreateOptions{
					Dynamic:   testing.FakeDynamicClient(),
					IOStreams: streams,
					Factory:   tf,
				},
			}
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
			o.BackupMethod = testing.BackupMethodName
			Expect(o.Validate()).Should(Succeed())
		})

		It("run backup command", func() {
			defaultBackupPolicy := testing.FakeBackupPolicy(policyName, testing.ClusterName)
			initClient(defaultBackupPolicy)
			By("test with specified backupPolicy")
			cmd := NewCreateBackupCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			// must succeed otherwise exit 1 and make test fails
			_ = cmd.Flags().Set("policy", defaultBackupPolicy.Name)
			_ = cmd.Flags().Set("method", testing.BackupMethodName)
			cmd.Run(cmd, []string{testing.ClusterName})

			By("test with logfile type")
			o := &CreateBackupOptions{
				CreateOptions: create.CreateOptions{
					IOStreams:       streams,
					Factory:         tf,
					GVR:             types.BackupGVR(),
					CueTemplateName: "backup_template.cue",
					Name:            testing.ClusterName,
				},
				BackupPolicy: defaultBackupPolicy.Name,
				BackupMethod: testing.BackupMethodName,
			}
			Expect(o.CompleteBackup()).Should(Succeed())
			err := o.Validate()
			Expect(err).Should(Succeed())
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
		Expect(PrintBackupList(o)).Should(Succeed())
		Expect(o.ErrOut.(*bytes.Buffer).String()).Should(ContainSubstring("No backups found"))

		By("test list-backup")
		backup1 := testing.FakeBackup("test1")
		backup1.Labels = map[string]string{
			constant.AppInstanceLabelKey: "apecloud-mysql",
		}
		backup1.Status.Phase = dpv1alpha1.BackupPhaseRunning
		backup2 := testing.FakeBackup("test1")
		backup2.Namespace = "backup"
		tf.FakeDynamicClient = testing.FakeDynamicClient(backup1, backup2)
		Expect(PrintBackupList(o)).Should(Succeed())
		Expect(o.Out.(*bytes.Buffer).String()).Should(ContainSubstring("test1"))
		Expect(o.Out.(*bytes.Buffer).String()).Should(ContainSubstring("apecloud-mysql"))

		By("test list all namespace")
		o.Out.(*bytes.Buffer).Reset()
		o.AllNamespaces = true
		Expect(PrintBackupList(o)).Should(Succeed())
		Expect(len(strings.Split(strings.Trim(o.Out.(*bytes.Buffer).String(), "\n"), "\n"))).Should(Equal(3))
	})

	It("restore", func() {
		timestamp := time.Now().Format("20060102150405")
		backupName := "backup-test-" + timestamp
		clusterName := "source-cluster-" + timestamp
		newClusterName := "new-cluster-" + timestamp
		secrets := testing.FakeSecrets(testing.Namespace, clusterName)
		clusterDef := testing.FakeClusterDef()
		clusterObj := testing.FakeCluster(clusterName, testing.Namespace)
		clusterDefLabel := map[string]string{
			constant.ClusterDefLabelKey: clusterDef.Name,
		}
		clusterObj.SetLabels(clusterDefLabel)
		backupPolicy := testing.FakeBackupPolicy("backPolicy", clusterObj.Name)

		pods := testing.FakePods(1, testing.Namespace, clusterName)
		tf.FakeDynamicClient = testing.FakeDynamicClient(&secrets.Items[0],
			&pods.Items[0], clusterDef, clusterObj, backupPolicy)
		tf.Client = &clientfake.RESTClient{}
		// create backup
		cmd := NewCreateBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		_ = cmd.Flags().Set("method", testing.BackupMethodName)
		_ = cmd.Flags().Set("name", backupName)
		cmd.Run(nil, []string{clusterName})

		By("restore new cluster from source cluster which is not deleted")
		// mock backup is ok
		mockBackupInfo(tf.FakeDynamicClient, backupName, clusterName, nil, "")
		cmdRestore := NewCreateRestoreCmd(tf, streams)
		Expect(cmdRestore != nil).To(BeTrue())
		_ = cmdRestore.Flags().Set("backup", backupName)
		cmdRestore.Run(nil, []string{newClusterName})
		newClusterObj := &appsv1alpha1.Cluster{}
		Expect(cluster.GetK8SClientObject(tf.FakeDynamicClient, newClusterObj, types.ClusterGVR(), testing.Namespace, newClusterName)).Should(Succeed())
		Expect(clusterObj.Spec.ComponentSpecs[0].Replicas).Should(Equal(int32(1)))
		By("restore new cluster from source cluster which is deleted")
		// mock cluster is not lived in kubernetes
		mockBackupInfo(tf.FakeDynamicClient, backupName, "deleted-cluster", nil, "")
		cmdRestore.Run(nil, []string{newClusterName + "1"})

		By("run restore cmd with cluster spec.affinity=nil")
		patchCluster := []byte(`{"spec":{"affinity":null}}`)
		_, _ = tf.FakeDynamicClient.Resource(types.ClusterGVR()).Namespace(testing.Namespace).Patch(context.TODO(), clusterName,
			k8sapitypes.MergePatchType, patchCluster, metav1.PatchOptions{})
		cmdRestore.Run(nil, []string{newClusterName + "-with-nil-affinity"})
	})

	// It("restore-to-time", func() {
	//	timestamp := time.Now().Format("20060102150405")
	//	backupName := "backup-test-" + timestamp
	//	backupName1 := backupName + "1"
	//	clusterName := "source-cluster-" + timestamp
	//	secrets := testing.FakeSecrets(testing.Namespace, clusterName)
	//	clusterDef := testing.FakeClusterDef()
	//	cluster := testing.FakeCluster(clusterName, testing.Namespace)
	//	clusterDefLabel := map[string]string{
	//		constant.ClusterDefLabelKey: clusterDef.Name,
	//	}
	//	cluster.SetLabels(clusterDefLabel)
	//	backupPolicy := testing.FakeBackupPolicy("backPolicy", cluster.Name)
	//	backupTypeMeta := testing.FakeBackup("backup-none").TypeMeta
	//	backupLabels := map[string]string{
	//		constant.AppInstanceLabelKey:             clusterName,
	//		constant.KBAppComponentLabelKey:          "test",
	//		dptypes.DataProtectionLabelClusterUIDKey: string(cluster.UID),
	//	}
	//	now := metav1.Now()
	//	baseBackup := testapps.NewBackupFactory(testing.Namespace, "backup-base").
	//		SetBackupMethod(dpv1alpha1.BackupTypeSnapshot).
	//		SetBackupTimeRange(now.Add(-time.Minute), now.Add(-time.Second)).
	//		SetLabels(backupLabels).GetObject()
	//	baseBackup.TypeMeta = backupTypeMeta
	//	baseBackup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
	//	logfileBackup := testapps.NewBackupFactory(testing.Namespace, backupName).
	//		SetBackupMethod(dpv1alpha1.BackupTypeLogFile).
	//		SetBackupTimeRange(now.Add(-time.Minute), now.Add(time.Minute)).
	//		SetLabels(backupLabels).GetObject()
	//	logfileBackup.TypeMeta = backupTypeMeta
	//
	//	logfileBackup1 := testapps.NewBackupFactory(testing.Namespace, backupName1).
	//		SetBackupMethod(dpv1alpha1.BackupTypeLogFile).
	//		SetBackupTimeRange(now.Add(-time.Minute), now.Add(2*time.Minute)).GetObject()
	//	uid := string(cluster.UID)
	//	logfileBackup1.Labels = map[string]string{
	//		constant.AppInstanceLabelKey:              clusterName,
	//		constant.KBAppComponentLabelKey:           "test",
	//		constant.DataProtectionLabelClusterUIDKey: uid[:30] + "00",
	//	}
	//	logfileBackup1.TypeMeta = backupTypeMeta
	//
	//	pods := testing.FakePods(1, testing.Namespace, clusterName)
	//	tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
	//		scheme.Scheme, &secrets.Items[0], &pods.Items[0], cluster, backupPolicy, baseBackup, logfileBackup, logfileBackup1)
	//	tf.Client = &clientfake.RESTClient{}
	//
	//	By("restore new cluster from source cluster which is not deleted")
	//	cmdRestore := NewCreateRestoreCmd(tf, streams)
	//	Expect(cmdRestore != nil).To(BeTrue())
	//	_ = cmdRestore.Flags().Set("restore-to-time", util.TimeFormatWithDuration(&now, time.Second))
	//	_ = cmdRestore.Flags().Set("source-cluster", clusterName)
	//	cmdRestore.Run(nil, []string{})
	//
	//	// test with RFC3339 format
	//	_ = cmdRestore.Flags().Set("restore-to-time", now.Format(time.RFC3339))
	//	_ = cmdRestore.Flags().Set("source-cluster", clusterName)
	//	cmdRestore.Run(nil, []string{"new-cluster"})
	//
	//	By("restore should be failed when backups belong to different source clusters")
	//	o := &CreateRestoreOptions{CreateOptions: create.CreateOptions{
	//		IOStreams: streams,
	//		Factory:   tf,
	//	}}
	//	restoreTime := time.Now().Add(90 * time.Second)
	//	o.RestoreTimeStr = util.TimeFormatWithDuration(&metav1.Time{Time: restoreTime}, time.Second)
	//	o.SourceCluster = clusterName
	//	Expect(o.Complete()).Should(Succeed())
	//	Expect(o.validateRestoreTime().Error()).Should(ContainSubstring("restore-to-time is out of time range"))
	// })

	It("describe-backup", func() {
		cmd := NewDescribeBackupCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		By("test describe-backup cmd with no backup")
		tf.FakeDynamicClient = testing.FakeDynamicClient()
		o := DescribeBackupOptions{
			Factory:   tf,
			IOStreams: streams,
			Gvr:       types.BackupGVR(),
		}
		args := []string{}
		Expect(o.Complete(args)).Should(HaveOccurred())

		By("test describe-backup")
		backupName := "test1"
		backup1 := testing.FakeBackup(backupName)
		args = append(args, backupName)
		backup1.Status.Phase = dpv1alpha1.BackupPhaseCompleted
		logNow := metav1.Now()
		backup1.Status.StartTimestamp = &logNow
		backup1.Status.CompletionTimestamp = &logNow
		backup1.Status.Expiration = &logNow
		backup1.Status.Duration = &metav1.Duration{Duration: logNow.Sub(logNow.Time)}
		tf.FakeDynamicClient = testing.FakeDynamicClient(backup1)
		Expect(o.Complete(args)).Should(Succeed())
		o.client = testing.FakeClientSet()
		Expect(o.Run()).Should(Succeed())
	})

	It("describe-backup-policy", func() {
		cmd := NewDescribeBackupPolicyCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		By("test describe-backup-policy cmd with no backup policy")
		tf.FakeDynamicClient = testing.FakeDynamicClient()
		o := describeBackupPolicyOptions{
			Factory:   tf,
			IOStreams: streams,
		}
		args := []string{}
		Expect(o.Complete(args)).Should(HaveOccurred())

		By("test describe-backup-policy")
		policyName := "test1"
		policy1 := testing.FakeBackupPolicy(policyName, testing.ClusterName)
		args = append(args, policyName)
		tf.FakeDynamicClient = testing.FakeDynamicClient(policy1)
		Expect(o.Complete(args)).Should(Succeed())
		o.client = testing.FakeClientSet()
		Expect(o.Run()).Should(Succeed())
	})

})

func mockBackupInfo(dynamic dynamic.Interface, backupName, clusterName string, timeRange map[string]any, backupMethod string) {
	clusterString := fmt.Sprintf(`{"metadata":{"name":"deleted-cluster","namespace":"%s"},"spec":{"clusterDefinitionRef":"apecloud-mysql","clusterVersionRef":"ac-mysql-8.0.30","componentSpecs":[{"name":"mysql","componentDefRef":"mysql","replicas":1}]}}`, testing.Namespace)
	backupStatus := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"phase":     "Completed",
				"timeRange": timeRange,
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
			"spec": map[string]any{
				"backupMethod": backupMethod,
			},
		},
	}
	_, err := dynamic.Resource(types.BackupGVR()).Namespace(testing.Namespace).UpdateStatus(context.TODO(),
		backupStatus, metav1.UpdateOptions{})
	Expect(err).Should(Succeed())
}
